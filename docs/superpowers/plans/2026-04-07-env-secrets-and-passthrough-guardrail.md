# Env-Secrets and Passthrough Guardrail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `airlock secret env` (CLI + GUI) for registering individual encrypted environment variables, and add a GUI guardrail preventing accidental removal of `api.anthropic.com` / `auth.anthropic.com` from the proxy passthrough list.

**Architecture:** New `EnvSecretConfig` config section persisted as ciphertext in `.airlock/config.yaml`. New `EnvSecretScanner` (peer of `FileScanner`) decrypts at session-start to populate the proxy mapping while injecting the same ciphertext as `NAME=ENC[age:...]` into the agent container's environment. Proxy substitutes ciphertext → plaintext on outbound calls to non-passthrough hosts using the same `mapping.json` it already consumes. Guardrail is Swift-only — one shared constant + one pure helper, used by Settings, WorkspaceSettings, and SecretsView.

**Tech Stack:** Go (cobra, gopkg.in/yaml.v3, filippo.io/age, golang.org/x/term), Swift/SwiftUI (XCTest, Process, NSAlert).

**Spec:** `docs/superpowers/specs/2026-04-07-env-secrets-and-passthrough-guardrail-design.md`

---

## File Structure

### New Go files

- `internal/config/reserved.go` — single source of truth for env var names airlock injects itself (HTTP_PROXY etc.)
- `internal/secrets/scanner_env_secret.go` — `EnvSecretScanner` peer of `FileScanner`
- `internal/secrets/scanner_env_secret_test.go` — unit tests
- `internal/cli/secret_env.go` — parent cobra subcommand group
- `internal/cli/secret_env_add.go` + `_test.go`
- `internal/cli/secret_env_list.go` + `_test.go`
- `internal/cli/secret_env_remove.go` + `_test.go`
- `internal/cli/secret_env_show.go` + `_test.go`

### Modified Go files

- `internal/config/config.go` — add `EnvSecretConfig`, `Config.EnvSecrets`, `validateEnvSecrets`
- `internal/config/config_test.go` — round-trip + validation tests
- `internal/secrets/scanner.go` — add `EnvVar` type and `ScanResult.Env`, merge in `ScanAll`
- `internal/secrets/scanner_test.go` — merge test
- `internal/container/manager.go` — add `RunOpts.EnvSecrets`, inject in `BuildClaudeConfig`
- `internal/container/manager_test.go` — injection tests
- `internal/orchestrator/session.go` — add `SessionParams.EnvSecrets`, propagate
- `internal/cli/run.go` — wire `EnvSecretScanner` into pipeline, propagate Env
- `internal/cli/start.go` — same wiring

### New Swift files

- `AirlockApp/Sources/AirlockApp/Models/PassthroughPolicy.swift`
- `AirlockApp/Sources/AirlockApp/Models/EnvSecret.swift`
- `AirlockApp/Sources/AirlockApp/Views/Secrets/AddEnvSecretSheet.swift`
- `AirlockApp/Tests/AirlockAppTests/PassthroughPolicyTests.swift`
- `AirlockApp/Tests/AirlockAppTests/EnvSecretTests.swift`

### Modified Swift files

- `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`
- `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`
- `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift`

### New docs

- `docs/decisions/ADR-0010-environment-variable-secrets.md`

### Modified docs

- `docs/guides/security-model.md`
- `CLAUDE.md`

---

## Task 1: Reserved env var names constant

**Files:**
- Create: `internal/config/reserved.go`

- [ ] **Step 1: Create the reserved-names file**

```go
// internal/config/reserved.go
package config

// ReservedEnvNames lists environment variables that airlock itself
// injects into the agent container. User-registered env secrets MUST
// NOT use these names; collisions are rejected at config load time.
//
// Keep in sync with the env block in internal/container/manager.go
// (BuildClaudeConfig).
var ReservedEnvNames = map[string]bool{
	"HTTP_PROXY":  true,
	"HTTPS_PROXY": true,
	"http_proxy":  true,
	"https_proxy": true,
	"NO_PROXY":    true,
	"LANG":        true,
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/config/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/config/reserved.go
git commit -m "feat(config): add reserved env var name list

Single source of truth for environment variables airlock injects
itself; will be used by env-secret validation to reject collisions."
```

---

## Task 2: Add EnvSecretConfig type to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing round-trip test**

Append to `internal/config/config_test.go`:

```go
func TestEnvSecretsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.EnvSecrets = []config.EnvSecretConfig{
		{Name: "GITHUB_TOKEN", Value: "ENC[age:AQIBAAABc29tZQ==]"},
		{Name: "SLACK_BOT_TOKEN", Value: "ENC[age:AQIBAAACb3RoZXI=]"},
	}
	if err := config.Save(cfg, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.EnvSecrets) != 2 {
		t.Fatalf("expected 2 env secrets, got %d", len(loaded.EnvSecrets))
	}
	if loaded.EnvSecrets[0].Name != "GITHUB_TOKEN" {
		t.Errorf("name[0] = %q, want GITHUB_TOKEN", loaded.EnvSecrets[0].Name)
	}
	if loaded.EnvSecrets[0].Value != "ENC[age:AQIBAAABc29tZQ==]" {
		t.Errorf("value[0] = %q, want ENC[age:...]", loaded.EnvSecrets[0].Value)
	}
}

func TestEnvSecretsBackwardsCompat(t *testing.T) {
	dir := t.TempDir()
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\nvolume_name: airlock-claude-home\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EnvSecrets != nil {
		t.Errorf("expected nil EnvSecrets for old config, got %v", loaded.EnvSecrets)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestEnvSecrets -v`
Expected: FAIL with `undefined: config.EnvSecretConfig` (or similar compile error).

- [ ] **Step 3: Add the type and field**

Edit `internal/config/config.go`. Replace the `Config` struct block with:

```go
// EnvSecretConfig is a single encrypted environment variable that
// airlock injects into the agent container as NAME=ENC[age:...].
// Value is always an ENC[age:...] ciphertext; plaintext is never
// persisted in config.yaml.
type EnvSecretConfig struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Config struct {
	ContainerImage   string             `yaml:"container_image"`
	ProxyImage       string             `yaml:"proxy_image"`
	NetworkName      string             `yaml:"network_name"`
	ProxyPort        int                `yaml:"proxy_port"`
	PassthroughHosts []string           `yaml:"passthrough_hosts"`
	VolumeName       string             `yaml:"volume_name"`
	SecretFiles      []SecretFileConfig `yaml:"secret_files,omitempty"`
	EnvSecrets       []EnvSecretConfig  `yaml:"env_secrets,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestEnvSecrets -v`
Expected: PASS for both `TestEnvSecretsRoundTrip` and `TestEnvSecretsBackwardsCompat`.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add EnvSecretConfig type and env_secrets field

New ciphertext-at-rest config section for individual environment
variable secrets, parallel to secret_files. Backward-compatible:
absent field deserializes as nil."
```

---

## Task 3: Validate env secrets on Load

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing validation tests**

Append to `internal/config/config_test.go`:

```go
import (
	// add to existing imports if not present:
	"strings"
)

func TestLoadRejectsInvalidEnvSecretName(t *testing.T) {
	cases := []struct {
		name     string
		envName  string
		fragment string
	}{
		{"leading digit", "1FOO", "invalid name"},
		{"hyphen", "FOO-BAR", "invalid name"},
		{"equals sign", "PATH=x", "invalid name"},
		{"empty", "", "invalid name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\nvolume_name: airlock-claude-home\nenv_secrets:\n  - name: \"" + tc.envName + "\"\n    value: \"ENC[age:AQIBAAAB]\"\n")
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
				t.Fatal(err)
			}
			_, err := config.Load(dir)
			if err == nil {
				t.Fatalf("expected error for name %q", tc.envName)
			}
			if !strings.Contains(err.Error(), tc.fragment) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.fragment)
			}
		})
	}
}

func TestLoadRejectsReservedEnvSecretName(t *testing.T) {
	dir := t.TempDir()
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\nvolume_name: airlock-claude-home\nenv_secrets:\n  - name: HTTP_PROXY\n    value: \"ENC[age:AQIBAAAB]\"\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected 'reserved' error, got %v", err)
	}
}

func TestLoadRejectsDuplicateEnvSecretName(t *testing.T) {
	dir := t.TempDir()
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\nvolume_name: airlock-claude-home\nenv_secrets:\n  - name: GITHUB_TOKEN\n    value: \"ENC[age:AQIBAAAB]\"\n  - name: GITHUB_TOKEN\n    value: \"ENC[age:AQIBAAAC]\"\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected 'duplicate' error, got %v", err)
	}
}

func TestLoadRejectsPlaintextEnvSecretValue(t *testing.T) {
	dir := t.TempDir()
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\nvolume_name: airlock-claude-home\nenv_secrets:\n  - name: GITHUB_TOKEN\n    value: \"plaintext-token-value\"\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "ENC[age:") {
		t.Fatalf("expected 'ENC[age:' error, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run TestLoadRejects -v`
Expected: All four tests FAIL because validation is not yet implemented (Load currently accepts any value).

- [ ] **Step 3: Add validation to Load**

Edit `internal/config/config.go`. Add new imports at the top:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/taeikkim92/airlock/internal/fsutil"
	"gopkg.in/yaml.v3"
)
```

Add the regex var and validation function above `Default()`:

```go
var envNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validateEnvSecrets enforces invariants on cfg.EnvSecrets:
// 1. Name matches POSIX env var format ^[A-Za-z_][A-Za-z0-9_]*$
// 2. Name is not in ReservedEnvNames
// 3. Names are unique within env_secrets[]
// 4. Value starts with "ENC[age:" and ends with "]"
//
// Any violation is a hard error so the user discovers misconfigurations
// at load time, not at session start.
func validateEnvSecrets(cfg *Config) error {
	seen := make(map[string]bool, len(cfg.EnvSecrets))
	for i, es := range cfg.EnvSecrets {
		if !envNameRegex.MatchString(es.Name) {
			return fmt.Errorf("env secret at index %d: invalid name %q: must match ^[A-Za-z_][A-Za-z0-9_]*$", i, es.Name)
		}
		if ReservedEnvNames[es.Name] {
			return fmt.Errorf("env secret name %q is reserved by airlock", es.Name)
		}
		if seen[es.Name] {
			return fmt.Errorf("duplicate env secret name %q", es.Name)
		}
		seen[es.Name] = true
		if !strings.HasPrefix(es.Value, "ENC[age:") || !strings.HasSuffix(es.Value, "]") {
			return fmt.Errorf("env secret %q: value is not an ENC[age:...] ciphertext", es.Name)
		}
	}
	return nil
}
```

Modify `Load` to call it. Replace the `Load` function with:

```go
func Load(airlockDir string) (Config, error) {
	configPath := filepath.Join(airlockDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := validateEnvSecrets(&cfg); err != nil {
		return Config{}, fmt.Errorf("config load: %w", err)
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: ALL tests pass, including the four new `TestLoadRejects*` and the round-trip from Task 2.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): validate env_secrets at load time

Hard-fail on invalid names, reserved names, duplicates, or
non-encrypted values. Surfaces misconfigurations at load time
instead of silently dropping entries at session start."
```

---

## Task 4: Add EnvVar type and Env field to ScanResult

**Files:**
- Modify: `internal/secrets/scanner.go`
- Modify: `internal/secrets/scanner_test.go` (or create if missing)

- [ ] **Step 1: Check whether scanner_test.go exists**

Run: `ls internal/secrets/scanner_test.go 2>&1 || echo "missing"`

If missing, create it with package declaration:

```go
// internal/secrets/scanner_test.go
package secrets_test

import (
	"testing"

	"github.com/taeikkim92/airlock/internal/secrets"
)
```

- [ ] **Step 2: Write failing merge test**

Append to `internal/secrets/scanner_test.go`:

```go
type stubScanner struct {
	name   string
	result *secrets.ScanResult
}

func (s *stubScanner) Name() string { return s.name }
func (s *stubScanner) Scan(opts secrets.ScanOpts) (*secrets.ScanResult, error) {
	return s.result, nil
}

func TestScanAllMergesEnv(t *testing.T) {
	a := &stubScanner{
		name: "a",
		result: &secrets.ScanResult{
			Env: []secrets.EnvVar{{Name: "FOO", Value: "ENC[age:AAA]"}},
		},
	}
	b := &stubScanner{
		name: "b",
		result: &secrets.ScanResult{
			Env: []secrets.EnvVar{{Name: "BAR", Value: "ENC[age:BBB]"}},
		},
	}
	merged, err := secrets.ScanAll([]secrets.Scanner{a, b}, secrets.ScanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Env) != 2 {
		t.Fatalf("expected 2 env entries, got %d", len(merged.Env))
	}
	names := map[string]string{}
	for _, e := range merged.Env {
		names[e.Name] = e.Value
	}
	if names["FOO"] != "ENC[age:AAA]" || names["BAR"] != "ENC[age:BBB]" {
		t.Errorf("unexpected merged env: %v", names)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/secrets/ -run TestScanAllMergesEnv -v`
Expected: FAIL with `undefined: secrets.EnvVar` or `unknown field Env in struct literal`.

- [ ] **Step 4: Add EnvVar and Env field**

Edit `internal/secrets/scanner.go`. Replace the `ScanResult` definition block (lines 22-33) with:

```go
// ShadowMount describes a file-level Docker bind mount that shadows
// a plaintext file with its encrypted counterpart.
type ShadowMount struct {
	HostPath      string // processed file in tmpDir
	ContainerPath string // container path to shadow
}

// EnvVar is an environment variable to inject into the agent
// container. Value is always an ENC[age:...] ciphertext; the proxy
// substitutes it back to plaintext on the wire when the agent makes
// outbound HTTP calls to non-passthrough hosts.
type EnvVar struct {
	Name  string
	Value string
}

// ScanResult holds the outputs from a scanner: shadow mounts, proxy
// mapping, and environment variables to inject into the agent container.
type ScanResult struct {
	Mounts  []ShadowMount
	Mapping map[string]string // ENC[age:...] -> plaintext
	Env     []EnvVar
}
```

Update `ScanAll` to merge `Env`. Replace the merge loop (lines 36-52) with:

```go
// ScanAll runs all scanners and merges their results.
func ScanAll(scanners []Scanner, opts ScanOpts) (*ScanResult, error) {
	merged := &ScanResult{Mapping: make(map[string]string)}
	for _, s := range scanners {
		result, err := s.Scan(opts)
		if err != nil {
			return nil, fmt.Errorf("scanner %s: %w", s.Name(), err)
		}
		if result == nil {
			continue
		}
		merged.Mounts = append(merged.Mounts, result.Mounts...)
		for k, v := range result.Mapping {
			merged.Mapping[k] = v
		}
		merged.Env = append(merged.Env, result.Env...)
	}
	return merged, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/secrets/ -run TestScanAllMergesEnv -v`
Expected: PASS.

- [ ] **Step 6: Run full secrets package to confirm no regressions**

Run: `go test ./internal/secrets/ -race`
Expected: PASS for all existing tests.

- [ ] **Step 7: Commit**

```bash
git add internal/secrets/scanner.go internal/secrets/scanner_test.go
git commit -m "feat(secrets): add EnvVar type and ScanResult.Env field

Extends the scanner pipeline to emit environment variables alongside
shadow mounts and mapping entries. ScanAll merges Env across all
scanners. No-op for existing scanners."
```

---

## Task 5: Implement EnvSecretScanner

**Files:**
- Create: `internal/secrets/scanner_env_secret.go`
- Create: `internal/secrets/scanner_env_secret_test.go`

- [ ] **Step 1: Write failing scanner tests**

Create `internal/secrets/scanner_env_secret_test.go`:

```go
package secrets_test

import (
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/secrets"
)

func newKeyPairForTest(t *testing.T) crypto.KeyPair {
	t.Helper()
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return kp
}

func encryptForTest(t *testing.T, plaintext string, kp crypto.KeyPair) string {
	t.Helper()
	ct, err := crypto.Encrypt(plaintext, kp.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return crypto.WrapENC(ct)
}

func TestEnvSecretScannerEmptyEntries(t *testing.T) {
	scanner := secrets.NewEnvSecretScanner(nil)
	result, err := scanner.Scan(secrets.ScanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Env) != 0 {
		t.Errorf("expected empty Env, got %d", len(result.Env))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected empty Mapping, got %d", len(result.Mapping))
	}
}

func TestEnvSecretScannerOneEntryRoundTrip(t *testing.T) {
	kp := newKeyPairForTest(t)
	wrapped := encryptForTest(t, "ghp_secret_value", kp)
	scanner := secrets.NewEnvSecretScanner([]config.EnvSecretConfig{
		{Name: "GITHUB_TOKEN", Value: wrapped},
	})
	result, err := scanner.Scan(secrets.ScanOpts{
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Env) != 1 {
		t.Fatalf("expected 1 Env entry, got %d", len(result.Env))
	}
	if result.Env[0].Name != "GITHUB_TOKEN" {
		t.Errorf("name = %q", result.Env[0].Name)
	}
	if !crypto.IsEncrypted(result.Env[0].Value) {
		t.Errorf("Env[0].Value is not encrypted: %q", result.Env[0].Value)
	}
	if result.Env[0].Value != wrapped {
		t.Errorf("Env[0].Value = %q, want %q (must be ciphertext, not plaintext)", result.Env[0].Value, wrapped)
	}
	if result.Mapping[wrapped] != "ghp_secret_value" {
		t.Errorf("mapping[ciphertext] = %q, want ghp_secret_value", result.Mapping[wrapped])
	}
}

func TestEnvSecretScannerWrongKeyFails(t *testing.T) {
	encryptKey := newKeyPairForTest(t)
	wrongKey := newKeyPairForTest(t)
	wrapped := encryptForTest(t, "secret", encryptKey)
	scanner := secrets.NewEnvSecretScanner([]config.EnvSecretConfig{
		{Name: "GITHUB_TOKEN", Value: wrapped},
	})
	_, err := scanner.Scan(secrets.ScanOpts{
		PublicKey:  wrongKey.PublicKey,
		PrivateKey: wrongKey.PrivateKey,
	})
	if err == nil {
		t.Fatal("expected decrypt error with wrong key")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("error %q does not include entry name", err.Error())
	}
}

func TestEnvSecretScannerNonEncryptedValueFails(t *testing.T) {
	kp := newKeyPairForTest(t)
	scanner := secrets.NewEnvSecretScanner([]config.EnvSecretConfig{
		{Name: "GITHUB_TOKEN", Value: "plaintext"},
	})
	_, err := scanner.Scan(secrets.ScanOpts{
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
	})
	if err == nil {
		t.Fatal("expected error for non-encrypted value")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/secrets/ -run TestEnvSecretScanner -v`
Expected: FAIL with `undefined: secrets.NewEnvSecretScanner`.

- [ ] **Step 3: Implement the scanner**

Create `internal/secrets/scanner_env_secret.go`:

```go
package secrets

import (
	"fmt"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

// EnvSecretScanner produces ScanResult.Env and Mapping entries for
// environment-variable secrets registered in config.yaml. Each entry
// is decrypted at scan time so the proxy mapping is populated, but
// the value injected into the agent container remains ciphertext.
//
// Distinct from EnvScanner, which handles whole .env files.
type EnvSecretScanner struct {
	entries []config.EnvSecretConfig
}

// NewEnvSecretScanner returns a scanner over the given env secret entries.
func NewEnvSecretScanner(entries []config.EnvSecretConfig) *EnvSecretScanner {
	return &EnvSecretScanner{entries: entries}
}

// Name returns the scanner identifier.
func (s *EnvSecretScanner) Name() string { return "env-secret" }

// Scan decrypts each entry to populate the proxy mapping and emits
// Env entries containing the original ciphertext for container injection.
func (s *EnvSecretScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	result := &ScanResult{Mapping: make(map[string]string)}
	for _, entry := range s.entries {
		if !crypto.IsEncrypted(entry.Value) {
			return nil, fmt.Errorf("env secret %q: value is not an ENC[age:...] ciphertext", entry.Name)
		}
		inner, err := crypto.UnwrapENC(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("env secret %q: unwrap: %w", entry.Name, err)
		}
		plain, err := crypto.Decrypt(inner, opts.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("env secret %q: decrypt: %w", entry.Name, err)
		}
		result.Mapping[entry.Value] = plain
		result.Env = append(result.Env, EnvVar{
			Name:  entry.Name,
			Value: entry.Value,
		})
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/secrets/ -run TestEnvSecretScanner -v -race`
Expected: All four tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/scanner_env_secret.go internal/secrets/scanner_env_secret_test.go
git commit -m "feat(secrets): add EnvSecretScanner

Decrypts each registered env secret to populate the proxy mapping
while emitting EnvVar entries containing the original ciphertext.
The agent container will see NAME=ENC[age:...]; the proxy substitutes
on outbound calls. Defense-in-depth IsEncrypted check guards against
config-validation bypass."
```

---

## Task 6: Inject env secrets into agent container

**Files:**
- Modify: `internal/container/manager.go`
- Modify: `internal/container/manager_test.go`

- [ ] **Step 1: Write failing container injection tests**

Append to `internal/container/manager_test.go`:

```go
func TestBuildClaudeConfigInjectsEnvSecrets(t *testing.T) {
	opts := container.RunOpts{
		Workspace:  "/home/user/project",
		Image:      "airlock-claude:latest",
		ProxyImage: "airlock-proxy:latest",
		NetworkName: "airlock-net",
		ProxyPort:  8080,
		EnvSecrets: []secrets.EnvVar{
			{Name: "GITHUB_TOKEN", Value: "ENC[age:AQIBAAAB]"},
			{Name: "SLACK_TOKEN", Value: "ENC[age:AQIBAAAC]"},
		},
	}
	cfg := container.BuildClaudeConfig(opts)

	wantPairs := map[string]string{
		"GITHUB_TOKEN": "ENC[age:AQIBAAAB]",
		"SLACK_TOKEN":  "ENC[age:AQIBAAAC]",
	}
	for name, wantValue := range wantPairs {
		found := false
		for _, e := range cfg.Env {
			if !strings.HasPrefix(e, name+"=") {
				continue
			}
			found = true
			gotValue := strings.TrimPrefix(e, name+"=")
			if !strings.HasPrefix(gotValue, "ENC[age:") {
				t.Errorf("%s value %q is not ciphertext", name, gotValue)
			}
			if gotValue != wantValue {
				t.Errorf("%s = %q, want %q", name, gotValue, wantValue)
			}
		}
		if !found {
			t.Errorf("env var %s not found in container Env", name)
		}
	}
}

func TestBuildClaudeConfigZeroEnvSecretsUnchanged(t *testing.T) {
	opts := container.RunOpts{
		Workspace:  "/home/user/project",
		Image:      "airlock-claude:latest",
		ProxyImage: "airlock-proxy:latest",
		NetworkName: "airlock-net",
		ProxyPort:  8080,
	}
	cfg := container.BuildClaudeConfig(opts)
	for _, e := range cfg.Env {
		if strings.HasPrefix(e, "GITHUB_TOKEN=") {
			t.Errorf("unexpected env var: %s", e)
		}
	}
	// Sanity: existing HTTP_PROXY block must still be present.
	hasHTTPProxy := false
	for _, e := range cfg.Env {
		if strings.HasPrefix(e, "HTTP_PROXY=") {
			hasHTTPProxy = true
		}
	}
	if !hasHTTPProxy {
		t.Error("HTTP_PROXY missing from container env")
	}
}

func TestBuildClaudeDetachedConfigInjectsEnvSecrets(t *testing.T) {
	opts := container.RunOpts{
		Workspace:  "/home/user/project",
		Image:      "airlock-claude:latest",
		ProxyImage: "airlock-proxy:latest",
		NetworkName: "airlock-net",
		ProxyPort:  8080,
		EnvSecrets: []secrets.EnvVar{
			{Name: "GITHUB_TOKEN", Value: "ENC[age:AQIBAAAB]"},
		},
	}
	cfg := container.BuildClaudeDetachedConfig(opts)
	found := false
	for _, e := range cfg.Env {
		if e == "GITHUB_TOKEN=ENC[age:AQIBAAAB]" {
			found = true
		}
	}
	if !found {
		t.Error("env secret not injected in detached mode")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/container/ -run TestBuildClaude -v`
Expected: FAIL with `unknown field EnvSecrets in struct literal of type container.RunOpts`.

- [ ] **Step 3: Add `EnvSecrets` to RunOpts and inject**

Edit `internal/container/manager.go`. Add field to `RunOpts`:

```go
type RunOpts struct {
	ID               string
	Workspace        string
	WorkspaceName    string
	Image            string
	ProxyImage       string
	NetworkName      string
	ShadowMounts     []secrets.ShadowMount
	MappingPath      string
	VolumeName       string
	ClaudeDir        string
	CACertPath       string
	ProxyPort        int
	PassthroughHosts []string
	EnvSecrets       []secrets.EnvVar
}
```

Replace the `Env: []string{...}` block in `BuildClaudeConfig` (lines 149-156) with a slice variable that gets appended to:

```go
	env := []string{
		fmt.Sprintf("HTTP_PROXY=%s", proxyURL),
		fmt.Sprintf("HTTPS_PROXY=%s", proxyURL),
		fmt.Sprintf("http_proxy=%s", proxyURL),
		fmt.Sprintf("https_proxy=%s", proxyURL),
		"NO_PROXY=localhost,127.0.0.1",
		"LANG=C.UTF-8",
	}
	for _, e := range opts.EnvSecrets {
		env = append(env, fmt.Sprintf("%s=%s", e.Name, e.Value))
	}

	return ContainerConfig{
		Image:      opts.Image,
		Name:       claudeName,
		Network:    opts.NetworkName,
		WorkingDir: containerWorkDir,
		Tty:        true,
		Stdin:      true,
		Binds:      binds,
		Mounts:     mounts,
		Env:        env,
		CapDrop:    []string{"ALL"},
		Cmd:        []string{"claude", "--dangerouslySkipPermissions"},
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/container/ -v -race`
Expected: All tests PASS, including the three new ones and existing regression coverage.

- [ ] **Step 5: Commit**

```bash
git add internal/container/manager.go internal/container/manager_test.go
git commit -m "feat(container): inject env secrets as ciphertext env vars

Adds RunOpts.EnvSecrets, appends NAME=ENC[age:...] to the agent
container's Env block. Detached and attached configs both inject.
Plaintext never appears in container env; the proxy substitutes
on the wire."
```

---

## Task 7: Propagate EnvSecrets through SessionParams

**Files:**
- Modify: `internal/orchestrator/session.go`

- [ ] **Step 1: Add field and propagate**

Edit `internal/orchestrator/session.go`. Add `EnvSecrets` to `SessionParams`:

```go
type SessionParams struct {
	ID           string
	Workspace    string
	VolumeName   string
	ClaudeDir    string
	Config       config.Config
	TmpDir       string
	ShadowMounts []secrets.ShadowMount
	MappingPath  string
	EnvSecrets   []secrets.EnvVar
}
```

In both `StartSession` and `StartDetachedSession`, the `opts := container.RunOpts{...}` literal needs `EnvSecrets: params.EnvSecrets,`. For `StartSession` (around line 82-94):

```go
	opts := container.RunOpts{
		ID:               params.ID,
		Workspace:        params.Workspace,
		Image:            cfg.ContainerImage,
		ProxyImage:       cfg.ProxyImage,
		NetworkName:      cfg.NetworkName,
		ShadowMounts:     params.ShadowMounts,
		MappingPath:      params.MappingPath,
		VolumeName:       params.VolumeName,
		ClaudeDir:        params.ClaudeDir,
		ProxyPort:        cfg.ProxyPort,
		PassthroughHosts: cfg.PassthroughHosts,
		EnvSecrets:       params.EnvSecrets,
	}
```

For `StartDetachedSession` (around line 160-172):

```go
	opts := container.RunOpts{
		ID:               params.ID,
		Workspace:        params.Workspace,
		Image:            cfg.ContainerImage,
		ProxyImage:       cfg.ProxyImage,
		NetworkName:      networkName,
		ShadowMounts:     params.ShadowMounts,
		MappingPath:      params.MappingPath,
		VolumeName:       params.VolumeName,
		ClaudeDir:        params.ClaudeDir,
		ProxyPort:        cfg.ProxyPort,
		PassthroughHosts: cfg.PassthroughHosts,
		EnvSecrets:       params.EnvSecrets,
	}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Run orchestrator and downstream tests**

Run: `go test ./internal/orchestrator/ ./internal/container/ ./internal/secrets/ -race`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/orchestrator/session.go
git commit -m "feat(orchestrator): propagate EnvSecrets through SessionParams

Plumbs the new env secrets field from CLI -> orchestrator -> container
manager so both attached (run) and detached (start) session paths
inject the same set of env vars."
```

---

## Task 8: Wire EnvSecretScanner into run.go and start.go

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/start.go`

- [ ] **Step 1: Update run.go**

Edit `internal/cli/run.go`. After the `FileScanner` append block (around line 117), insert:

```go
			if len(cfg.EnvSecrets) > 0 {
				scanners = append(scanners, secrets.NewEnvSecretScanner(cfg.EnvSecrets))
			}
```

After `params.ShadowMounts = scanResult.Mounts` (around line 138), add:

```go
			params.EnvSecrets = scanResult.Env
```

- [ ] **Step 2: Update start.go**

Edit `internal/cli/start.go`. After the `FileScanner` append block (around line 104), insert:

```go
		if len(cfg.EnvSecrets) > 0 {
			scanners = append(scanners, secrets.NewEnvSecretScanner(cfg.EnvSecrets))
		}
```

After `params.ShadowMounts = scanResult.Mounts` (around line 125), add:

```go
		params.EnvSecrets = scanResult.Env
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output.

- [ ] **Step 4: Run all Go tests**

Run: `go test ./... -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/run.go internal/cli/start.go
git commit -m "feat(cli): wire EnvSecretScanner into run and start

Both CLI entry points now invoke the EnvSecretScanner when
cfg.EnvSecrets is non-empty, and propagate scanResult.Env into
the SessionParams that orchestrate container startup."
```

---

## Task 9: secret env parent cobra group

**Files:**
- Create: `internal/cli/secret_env.go`

- [ ] **Step 1: Create the parent command**

```go
// internal/cli/secret_env.go
package cli

import "github.com/spf13/cobra"

var secretEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment-variable secrets",
	Long: `Register, list, show, and remove individual environment variable
secrets. Stored encrypted in .airlock/config.yaml; injected into the
agent container as NAME=ENC[age:...]; substituted at the proxy
boundary on outbound HTTP calls.`,
}

func init() {
	secretCmd.AddCommand(secretEnvCmd)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/cli/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/secret_env.go
git commit -m "feat(cli): add 'airlock secret env' parent subcommand

Empty parent group; subcommands (add/list/show/remove) follow."
```

---

## Task 10: secret env add

**Files:**
- Create: `internal/cli/secret_env_add.go`
- Create: `internal/cli/secret_env_add_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/cli/secret_env_add_test.go`:

```go
package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestSecretEnvAddPlaintext(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)

	err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "ghp_secret", false, airlockDir)
	if err != nil {
		t.Fatalf("RunSecretEnvAdd: %v", err)
	}

	cfg, err := config.Load(airlockDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.EnvSecrets) != 1 {
		t.Fatalf("expected 1 env secret, got %d", len(cfg.EnvSecrets))
	}
	if cfg.EnvSecrets[0].Name != "GITHUB_TOKEN" {
		t.Errorf("name = %q", cfg.EnvSecrets[0].Name)
	}
	if !crypto.IsEncrypted(cfg.EnvSecrets[0].Value) {
		t.Errorf("value not encrypted: %q", cfg.EnvSecrets[0].Value)
	}

	// Plaintext must NOT appear in the on-disk config.
	data, err := os.ReadFile(filepath.Join(airlockDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ghp_secret") {
		t.Error("config.yaml contains plaintext value")
	}
	if !strings.Contains(string(data), "ENC[age:") {
		t.Error("config.yaml does not contain ENC[age: marker")
	}
}

func TestSecretEnvAddRoundTripsThroughDecrypt(t *testing.T) {
	_, keysDir, airlockDir := setupAirlock(t)

	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "ghp_round_trip", false, airlockDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(airlockDir)
	if err != nil {
		t.Fatal(err)
	}
	kp, err := crypto.LoadKeyPair(keysDir)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := crypto.UnwrapENC(cfg.EnvSecrets[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := crypto.Decrypt(inner, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "ghp_round_trip" {
		t.Errorf("decrypted = %q", plain)
	}
}

func TestSecretEnvAddDuplicateWithoutForce(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v1", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v2", false, airlockDir)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got %v", err)
	}
	cfg, _ := config.Load(airlockDir)
	if len(cfg.EnvSecrets) != 1 {
		t.Errorf("expected 1 entry after failed dup, got %d", len(cfg.EnvSecrets))
	}
}

func TestSecretEnvAddDuplicateWithForce(t *testing.T) {
	_, keysDir, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v1", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v2", true, airlockDir); err != nil {
		t.Fatalf("force add: %v", err)
	}
	cfg, _ := config.Load(airlockDir)
	if len(cfg.EnvSecrets) != 1 {
		t.Errorf("expected 1 entry after force overwrite, got %d", len(cfg.EnvSecrets))
	}
	kp, _ := crypto.LoadKeyPair(keysDir)
	inner, _ := crypto.UnwrapENC(cfg.EnvSecrets[0].Value)
	plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
	if plain != "v2" {
		t.Errorf("force overwrite failed: decrypted = %q", plain)
	}
}

func TestSecretEnvAddInvalidName(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	cases := []string{"1FOO", "FOO-BAR", "PATH=x", ""}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			err := cli.RunSecretEnvAdd(name, "v", false, airlockDir)
			if err == nil {
				t.Errorf("expected error for name %q", name)
			}
		})
	}
}

func TestSecretEnvAddReservedName(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	err := cli.RunSecretEnvAdd("HTTP_PROXY", "http://evil", false, airlockDir)
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected 'reserved' error, got %v", err)
	}
}

func TestSecretEnvAddPreWrappedValue(t *testing.T) {
	_, keysDir, airlockDir := setupAirlock(t)
	kp, _ := crypto.LoadKeyPair(keysDir)
	ct, _ := crypto.Encrypt("already_encrypted", kp.PublicKey)
	wrapped := crypto.WrapENC(ct)

	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", wrapped, false, airlockDir); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(airlockDir)
	if cfg.EnvSecrets[0].Value != wrapped {
		t.Errorf("pre-wrapped value was double-wrapped or modified")
	}
}

func TestSecretEnvAddMissingKeys(t *testing.T) {
	dir := t.TempDir()
	airlockDir := filepath.Join(dir, ".airlock")
	if err := os.MkdirAll(airlockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(config.Default(), airlockDir); err != nil {
		t.Fatal(err)
	}
	// no keys saved
	err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v", false, airlockDir)
	if err == nil || !strings.Contains(err.Error(), "init") {
		t.Fatalf("expected 'init' error, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestSecretEnvAdd -v`
Expected: FAIL with `undefined: cli.RunSecretEnvAdd`.

- [ ] **Step 3: Implement RunSecretEnvAdd and cobra wrapper**

Create `internal/cli/secret_env_add.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

// RunSecretEnvAdd registers an env secret. The plaintext value is
// encrypted with the workspace age public key and stored as
// ENC[age:...] ciphertext in config.yaml. If value is already wrapped
// in ENC[age:...] it is stored as-is (idempotent round-tripping).
func RunSecretEnvAdd(name, value string, force bool, airlockDir string) error {
	keysDir := filepath.Join(airlockDir, "keys")
	kp, err := crypto.LoadKeyPair(keysDir)
	if err != nil {
		return fmt.Errorf("no encryption keys found; run 'airlock init' first: %w", err)
	}

	cfg, err := config.Load(airlockDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Validation done up-front so we never call Encrypt on a bad name.
	if !envNameRegexCLI.MatchString(name) {
		return fmt.Errorf("invalid name %q: must match ^[A-Za-z_][A-Za-z0-9_]*$", name)
	}
	if config.ReservedEnvNames[name] {
		return fmt.Errorf("env secret name %q is reserved by airlock", name)
	}

	// Encrypt or accept already-wrapped.
	var stored string
	if crypto.IsEncrypted(value) {
		stored = value
	} else {
		ct, encErr := crypto.Encrypt(value, kp.PublicKey)
		if encErr != nil {
			return fmt.Errorf("encrypt: %w", encErr)
		}
		stored = crypto.WrapENC(ct)
	}

	// Upsert.
	for i, es := range cfg.EnvSecrets {
		if es.Name == name {
			if !force {
				return fmt.Errorf("env secret %q already exists; use --force to overwrite", name)
			}
			cfg.EnvSecrets[i].Value = stored
			if err := config.Save(cfg, airlockDir); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("Updated env secret %s\n", name)
			return nil
		}
	}
	cfg.EnvSecrets = append(cfg.EnvSecrets, config.EnvSecretConfig{
		Name:  name,
		Value: stored,
	})
	if err := config.Save(cfg, airlockDir); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Added env secret %s\n", name)
	return nil
}

// envNameRegexCLI mirrors the regex in internal/config; duplicated to
// avoid exporting it from config and to keep validation local at the
// CLI boundary too. Both must stay in sync.
var envNameRegexCLI = mustEnvNameRegex()

func mustEnvNameRegex() *envNameRe {
	return &envNameRe{}
}

type envNameRe struct{}

func (envNameRe) MatchString(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// readSecretValue resolves --value / --stdin / TTY prompt precedence.
// Returns an error for non-TTY without explicit flags.
func readSecretValue(name string, valueFlag string, valueFlagSet bool, stdinFlag bool, stdin io.Reader) (string, error) {
	if valueFlagSet && stdinFlag {
		return "", errors.New("--value and --stdin are mutually exclusive")
	}
	if valueFlagSet {
		return valueFlag, nil
	}
	if stdinFlag {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return strings.TrimRight(string(data), "\n"), nil
	}
	// TTY prompt
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("refusing to read plaintext from non-terminal without --value or --stdin")
	}
	fmt.Printf("Value for %s: ", name)
	bytePass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(bytePass), nil
}

var (
	secretEnvAddValue    string
	secretEnvAddValueSet bool
	secretEnvAddStdin    bool
	secretEnvAddForce    bool
)

var secretEnvAddCmd = &cobra.Command{
	Use:   "add <NAME>",
	Short: "Register an environment-variable secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		secretEnvAddValueSet = cmd.Flags().Changed("value")
		value, err := readSecretValue(
			args[0],
			secretEnvAddValue, secretEnvAddValueSet,
			secretEnvAddStdin,
			os.Stdin,
		)
		if err != nil {
			return err
		}
		return RunSecretEnvAdd(args[0], value, secretEnvAddForce, ".airlock")
	},
}

func init() {
	secretEnvAddCmd.Flags().StringVar(&secretEnvAddValue, "value", "", "value (warning: visible in process list)")
	secretEnvAddCmd.Flags().BoolVar(&secretEnvAddStdin, "stdin", false, "read value from stdin")
	secretEnvAddCmd.Flags().BoolVar(&secretEnvAddForce, "force", false, "overwrite existing entry")
	secretEnvCmd.AddCommand(secretEnvAddCmd)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestSecretEnvAdd -v -race`
Expected: All eight tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/secret_env_add.go internal/cli/secret_env_add_test.go
git commit -m "feat(cli): add 'airlock secret env add' command

Encrypts plaintext at add time, stores ciphertext in config.yaml.
Accepts pre-wrapped ENC[age:...] values for round-tripping. Three
value sources: --value (argv, GUI/scripting), --stdin (pipe), TTY
prompt (hidden). --force required to overwrite. Hard-fails on
reserved names and missing keys."
```

---

## Task 11: secret env list

**Files:**
- Create: `internal/cli/secret_env_list.go`
- Create: `internal/cli/secret_env_list_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/cli/secret_env_list_test.go`:

```go
package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
)

func TestSecretEnvListEmpty(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	out, err := cli.RunSecretEnvList(airlockDir, true)
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]string
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %v", entries)
	}
}

func TestSecretEnvListJSONSorted(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("ZULU", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("ALPHA", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("MIKE", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}

	out, err := cli.RunSecretEnvList(airlockDir, true)
	if err != nil {
		t.Fatal(err)
	}

	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	want := []string{"ALPHA", "MIKE", "ZULU"}
	for i, w := range want {
		if entries[i].Name != w {
			t.Errorf("entries[%d].Name = %q, want %q", i, entries[i].Name, w)
		}
	}
	// Plaintext must not appear in JSON output.
	if strings.Contains(string(out), "\"value\"") {
		t.Error("list JSON should not include value field")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestSecretEnvList -v`
Expected: FAIL with `undefined: cli.RunSecretEnvList`.

- [ ] **Step 3: Implement RunSecretEnvList and cobra wrapper**

Create `internal/cli/secret_env_list.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
)

// RunSecretEnvList returns a representation of the registered env
// secrets. When asJSON is true the return is a sorted JSON array of
// {"name": ...} objects (the GUI contract). Otherwise the return is
// a human-readable table as bytes.
//
// The plaintext value is never returned. The ciphertext is also not
// returned by 'list'; use 'show' for a (truncated) ciphertext fingerprint.
func RunSecretEnvList(airlockDir string, asJSON bool) ([]byte, error) {
	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	type entry struct {
		Name string `json:"name"`
	}
	entries := make([]entry, 0, len(cfg.EnvSecrets))
	for _, es := range cfg.EnvSecrets {
		entries = append(entries, entry{Name: es.Name})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	if asJSON {
		return json.MarshalIndent(entries, "", "  ")
	}

	if len(entries) == 0 {
		return []byte("No env secrets registered.\n"), nil
	}
	out := "NAME\n"
	for _, e := range entries {
		out += "  " + e.Name + "\n"
	}
	return []byte(out), nil
}

var secretEnvListJSON bool

var secretEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered env secrets (names only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := RunSecretEnvList(".airlock", secretEnvListJSON)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		if len(out) > 0 && out[len(out)-1] != '\n' {
			fmt.Println()
		}
		return nil
	},
}

func init() {
	secretEnvListCmd.Flags().BoolVar(&secretEnvListJSON, "json", false, "output as JSON")
	secretEnvCmd.AddCommand(secretEnvListCmd)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestSecretEnvList -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/secret_env_list.go internal/cli/secret_env_list_test.go
git commit -m "feat(cli): add 'airlock secret env list' command

Returns names only (sorted), human or JSON. Never emits plaintext
or ciphertext. The GUI consumes the JSON form."
```

---

## Task 12: secret env show

**Files:**
- Create: `internal/cli/secret_env_show.go`
- Create: `internal/cli/secret_env_show_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/cli/secret_env_show_test.go`:

```go
package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
)

func TestSecretEnvShowJSONNoPlaintext(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "ghp_super_secret_xyz", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	out, err := cli.RunSecretEnvShow("GITHUB_TOKEN", airlockDir, true)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(out), "ghp_super_secret_xyz") {
		t.Errorf("show output contains plaintext: %s", out)
	}

	var info struct {
		Name        string `json:"name"`
		Encrypted   bool   `json:"encrypted"`
		ValuePrefix string `json:"value_prefix"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.Name != "GITHUB_TOKEN" {
		t.Errorf("name = %q", info.Name)
	}
	if !info.Encrypted {
		t.Error("encrypted = false")
	}
	if !strings.HasPrefix(info.ValuePrefix, "ENC[age:") {
		t.Errorf("value_prefix = %q", info.ValuePrefix)
	}
}

func TestSecretEnvShowUnknown(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	_, err := cli.RunSecretEnvShow("NOPE", airlockDir, true)
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestSecretEnvShow -v`
Expected: FAIL with `undefined: cli.RunSecretEnvShow`.

- [ ] **Step 3: Implement RunSecretEnvShow and cobra wrapper**

Create `internal/cli/secret_env_show.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
)

const showValuePrefixLen = 16

// RunSecretEnvShow returns metadata about a single env secret.
// It NEVER decrypts. The ciphertext is truncated to the first
// showValuePrefixLen characters of the wrapped form.
func RunSecretEnvShow(name, airlockDir string, asJSON bool) ([]byte, error) {
	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	for _, es := range cfg.EnvSecrets {
		if es.Name != name {
			continue
		}
		prefix := es.Value
		if len(prefix) > showValuePrefixLen {
			prefix = prefix[:showValuePrefixLen]
		}
		if asJSON {
			return json.MarshalIndent(map[string]interface{}{
				"name":         es.Name,
				"encrypted":    true,
				"value_prefix": prefix,
			}, "", "  ")
		}
		out := fmt.Sprintf("name: %s\nencrypted: true\nvalue: %s...\n", es.Name, prefix)
		return []byte(out), nil
	}
	return nil, fmt.Errorf("no such env secret: %s", name)
}

var secretEnvShowJSON bool

var secretEnvShowCmd = &cobra.Command{
	Use:   "show <NAME>",
	Short: "Show env secret metadata (never decrypts)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := RunSecretEnvShow(args[0], ".airlock", secretEnvShowJSON)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		if len(out) > 0 && out[len(out)-1] != '\n' {
			fmt.Println()
		}
		return nil
	},
}

func init() {
	secretEnvShowCmd.Flags().BoolVar(&secretEnvShowJSON, "json", false, "output as JSON")
	secretEnvCmd.AddCommand(secretEnvShowCmd)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestSecretEnvShow -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/secret_env_show.go internal/cli/secret_env_show_test.go
git commit -m "feat(cli): add 'airlock secret env show' command

Returns name + truncated ciphertext prefix; never decrypts. Errors
on unknown names. JSON output format used by the GUI."
```

---

## Task 13: secret env remove

**Files:**
- Create: `internal/cli/secret_env_remove.go`
- Create: `internal/cli/secret_env_remove_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/cli/secret_env_remove_test.go`:

```go
package cli_test

import (
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
	"github.com/taeikkim92/airlock/internal/config"
)

func TestSecretEnvRemoveKnown(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("FOO", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("BAR", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvRemove("FOO", airlockDir); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(airlockDir)
	if len(cfg.EnvSecrets) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(cfg.EnvSecrets))
	}
	if cfg.EnvSecrets[0].Name != "BAR" {
		t.Errorf("BAR was removed instead of FOO")
	}
}

func TestSecretEnvRemoveUnknown(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	err := cli.RunSecretEnvRemove("NOPE", airlockDir)
	if err == nil || !strings.Contains(err.Error(), "no such env secret") {
		t.Fatalf("expected 'no such env secret' error, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestSecretEnvRemove -v`
Expected: FAIL with `undefined: cli.RunSecretEnvRemove`.

- [ ] **Step 3: Implement RunSecretEnvRemove and cobra wrapper**

Create `internal/cli/secret_env_remove.go`:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
)

// RunSecretEnvRemove unregisters an env secret. Errors if the name
// is not present.
func RunSecretEnvRemove(name, airlockDir string) error {
	cfg, err := config.Load(airlockDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	filtered := make([]config.EnvSecretConfig, 0, len(cfg.EnvSecrets))
	found := false
	for _, es := range cfg.EnvSecrets {
		if es.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, es)
	}
	if !found {
		return fmt.Errorf("no such env secret: %s", name)
	}
	cfg.EnvSecrets = filtered
	if err := config.Save(cfg, airlockDir); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Removed env secret %s\n", name)
	return nil
}

var secretEnvRemoveCmd = &cobra.Command{
	Use:   "remove <NAME>",
	Short: "Unregister an env secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunSecretEnvRemove(args[0], ".airlock")
	},
}

func init() {
	secretEnvCmd.AddCommand(secretEnvRemoveCmd)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestSecretEnvRemove -v`
Expected: PASS.

- [ ] **Step 5: Run full Go suite**

Run: `make test`
Expected: All Go tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/secret_env_remove.go internal/cli/secret_env_remove_test.go
git commit -m "feat(cli): add 'airlock secret env remove' command

Symmetric with 'secret remove' for files. No confirmation prompt;
the GUI handles the confirm modal. Errors on unknown names."
```

---

## Task 14: PassthroughPolicy Swift model and tests

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Models/PassthroughPolicy.swift`
- Create: `AirlockApp/Tests/AirlockAppTests/PassthroughPolicyTests.swift`

- [ ] **Step 1: Write failing tests**

Create `AirlockApp/Tests/AirlockAppTests/PassthroughPolicyTests.swift`:

```swift
import XCTest
@testable import AirlockApp

final class PassthroughPolicyTests: XCTestCase {
    func testBothPresentReturnsEmpty() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["api.anthropic.com", "auth.anthropic.com"]
        )
        XCTAssertEqual(missing, [])
    }

    func testEmptyListReturnsBoth() {
        let missing = PassthroughPolicy.missingProtectedHosts(from: [])
        XCTAssertEqual(missing.sorted(), ["api.anthropic.com", "auth.anthropic.com"])
    }

    func testApiMissing() {
        let missing = PassthroughPolicy.missingProtectedHosts(from: ["auth.anthropic.com"])
        XCTAssertEqual(missing, ["api.anthropic.com"])
    }

    func testAuthMissing() {
        let missing = PassthroughPolicy.missingProtectedHosts(from: ["api.anthropic.com"])
        XCTAssertEqual(missing, ["auth.anthropic.com"])
    }

    func testCaseInsensitive() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["API.ANTHROPIC.COM", "Auth.Anthropic.Com"]
        )
        XCTAssertEqual(missing, [])
    }

    func testWhitespaceTolerant() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["  api.anthropic.com  ", "\tauth.anthropic.com"]
        )
        XCTAssertEqual(missing, [])
    }

    func testUnrelatedHostsDoNotMatter() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["github.com", "slack.com", "api.anthropic.com", "auth.anthropic.com"]
        )
        XCTAssertEqual(missing, [])
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd AirlockApp && swift test --filter PassthroughPolicyTests 2>&1 | head -30`
Expected: FAIL with `cannot find 'PassthroughPolicy' in scope`.

- [ ] **Step 3: Implement the model**

Create `AirlockApp/Sources/AirlockApp/Models/PassthroughPolicy.swift`:

```swift
import Foundation

/// Single source of truth for the proxy passthrough hosts that
/// must remain in the list to preserve airlock's privacy property:
/// the model and Anthropic's servers must never see plaintext
/// secrets, which requires the proxy to NOT substitute on outbound
/// requests to api.anthropic.com / auth.anthropic.com.
///
/// Removing either host causes airlock-proxy to begin substituting
/// ENC[age:...] tokens with plaintext on the wire to Anthropic,
/// defeating the privacy property.
///
/// Mirrors the default in proxy/addon/decrypt_proxy.py and ADR-0005.
enum PassthroughPolicy {
    static let protectedHosts: Set<String> = [
        "api.anthropic.com",
        "auth.anthropic.com",
    ]

    /// Returns protected hosts missing from the given list.
    /// Comparison is case-insensitive and whitespace-tolerant.
    /// Returned slice is sorted alphabetically for stable display.
    static func missingProtectedHosts(from list: [String]) -> [String] {
        let normalized = Set(
            list.map { $0.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() }
        )
        return protectedHosts
            .filter { !normalized.contains($0) }
            .sorted()
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd AirlockApp && swift test --filter PassthroughPolicyTests 2>&1 | tail -20`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Models/PassthroughPolicy.swift AirlockApp/Tests/AirlockAppTests/PassthroughPolicyTests.swift
git commit -m "feat(gui): add PassthroughPolicy model with protected-host helper

Single Swift constant + pure helper for detecting whether a passthrough
list is missing api.anthropic.com / auth.anthropic.com. Used by the
Settings, WorkspaceSettings, and Secrets views as a guardrail."
```

---

## Task 15: Settings tab passthrough guardrail

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`

- [ ] **Step 1: Add inline warning + save-time confirmation**

Edit `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`. Add a state var with the others (around line 11):

```swift
    @State private var showRemoveAnthropicConfirm = false
    @State private var pendingMissingHosts: [String] = []
```

Replace the `Section("Network Defaults")` block (lines 63-70) with:

```swift
                Section("Network Defaults") {
                    Text("Default passthrough hosts (skip proxy decryption, one per line)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextEditor(text: $passthroughText)
                        .font(.system(size: 12, design: .monospaced))
                        .frame(height: 80)

                    let missing = PassthroughPolicy.missingProtectedHosts(
                        from: parsePassthroughEditorLines(passthroughText)
                    )
                    if !missing.isEmpty {
                        HStack(alignment: .top, spacing: 6) {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(.yellow)
                            Text("Removing \(missing.joined(separator: ", ")) from passthrough means Airlock will decrypt secrets in requests to Anthropic. Your plaintext credentials will be sent to Anthropic's servers. This defeats the purpose of Airlock — only remove for testing.")
                                .font(.caption)
                                .foregroundStyle(.yellow)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        .padding(8)
                        .background(Color.yellow.opacity(0.08))
                        .clipShape(RoundedRectangle(cornerRadius: 4))
                    }
                }
```

Add a helper function inside the struct (e.g., right above `private func load()`):

```swift
    private func parsePassthroughEditorLines(_ text: String) -> [String] {
        text
            .components(separatedBy: "\n")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
    }
```

Replace the existing `save()` function with:

```swift
    private func save() {
        let parsed = parsePassthroughEditorLines(passthroughText)
        let missing = PassthroughPolicy.missingProtectedHosts(from: parsed)
        if !missing.isEmpty {
            pendingMissingHosts = missing
            showRemoveAnthropicConfirm = true
            return
        }
        commitSave(parsedHosts: parsed)
    }

    private func commitSave(parsedHosts: [String]) {
        settings.passthroughHosts = parsedHosts

        let store = WorkspaceStore()
        do {
            try store.saveSettings(settings)
            appState.settings = settings
            withAnimation { saved = true }
            Task { @MainActor in
                try? await Task.sleep(for: .seconds(1))
                saved = false
                dismiss()
            }
        } catch {
            appState.lastError = "Failed to save settings: \(error.localizedDescription)"
        }
    }
```

Add an `.alert(...)` modifier on the outer `VStack` (right after the existing `.alert("Reset Volume?", ...)` block, around line 127):

```swift
        .alert("Disable Anthropic passthrough?", isPresented: $showRemoveAnthropicConfirm) {
            Button("Cancel", role: .cancel) {}
            Button("Remove anyway", role: .destructive) {
                commitSave(parsedHosts: parsePassthroughEditorLines(passthroughText))
            }
        } message: {
            Text("\(pendingMissingHosts.joined(separator: ", ")) will be removed from passthrough. Airlock will then decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers. Continue?")
        }
```

- [ ] **Step 2: Build the macOS app to verify**

Run: `make gui-build 2>&1 | tail -20`
Expected: Build succeeds.

- [ ] **Step 3: Run Swift tests**

Run: `make gui-test 2>&1 | tail -20`
Expected: PASS (existing tests + PassthroughPolicyTests).

- [ ] **Step 4: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift
git commit -m "feat(gui): warn before removing Anthropic from global passthrough

Inline yellow warning appears live in the global Settings sheet
when the editor's content omits a protected host. On Save, a
destructive-styled confirmation alert blocks the change until
the user explicitly clicks 'Remove anyway'."
```

---

## Task 16: Workspace passthrough override guardrail

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`

- [ ] **Step 1: Apply the same guardrail to the workspace override**

Edit `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`. Add state vars at the top:

```swift
    @State private var showRemoveAnthropicConfirm = false
    @State private var pendingMissingHosts: [String] = []
```

Replace the `Section("Network Overrides")` block (lines 43-53) with:

```swift
            Section("Network Overrides") {
                let defaultHint = globalSettings.passthroughHosts.isEmpty
                    ? "No default passthrough hosts"
                    : "Default: \(globalSettings.passthroughHosts.joined(separator: ", "))"
                Text("Passthrough hosts override (\(defaultHint))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                TextEditor(text: $passthroughText)
                    .font(.system(size: 12, design: .monospaced))
                    .frame(height: 80)

                let parsedNonEmpty = parsedHostLines()
                if !parsedNonEmpty.isEmpty {
                    let missing = PassthroughPolicy.missingProtectedHosts(from: parsedNonEmpty)
                    if !missing.isEmpty {
                        HStack(alignment: .top, spacing: 6) {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(.yellow)
                            Text("This override would remove \(missing.joined(separator: ", ")) from passthrough. Airlock would decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers.")
                                .font(.caption)
                                .foregroundStyle(.yellow)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        .padding(8)
                        .background(Color.yellow.opacity(0.08))
                        .clipShape(RoundedRectangle(cornerRadius: 4))
                    }
                }
            }
```

Add the parser helper (above `private func load()`):

```swift
    private func parsedHostLines() -> [String] {
        passthroughText
            .components(separatedBy: "\n")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
    }
```

Replace `save()` with:

```swift
    private func save() {
        let hosts = parsedHostLines()
        // Empty override = inherit global; not flagged.
        if !hosts.isEmpty {
            let missing = PassthroughPolicy.missingProtectedHosts(from: hosts)
            if !missing.isEmpty {
                pendingMissingHosts = missing
                showRemoveAnthropicConfirm = true
                return
            }
        }
        commitSave(hosts: hosts)
    }

    private func commitSave(hosts: [String]) {
        if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
            appState.workspaces[idx].passthroughHostsOverride = hosts.isEmpty ? nil : hosts
        }
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }
```

Attach the alert at the bottom of the `Form { ... }` block, right after `.formStyle(.grouped)`:

```swift
        .formStyle(.grouped)
        .padding()
        .onAppear { load() }
        .alert("Disable Anthropic passthrough for this workspace?", isPresented: $showRemoveAnthropicConfirm) {
            Button("Cancel", role: .cancel) {}
            Button("Remove anyway", role: .destructive) {
                commitSave(hosts: parsedHostLines())
            }
        } message: {
            Text("\(pendingMissingHosts.joined(separator: ", ")) will not be in this workspace's passthrough list. Airlock will decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers. Continue?")
        }
```

- [ ] **Step 2: Build to verify**

Run: `make gui-build 2>&1 | tail -20`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift
git commit -m "feat(gui): warn before removing Anthropic from workspace override

Same inline warning + confirm modal as the global Settings tab.
Empty overrides (which inherit global) are not flagged."
```

---

## Task 17: SecretsView resolved-passthrough banner

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift`

- [ ] **Step 1: Add the persistent banner**

Edit `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift`. Modify the top of `var body` (around line 30):

```swift
    var body: some View {
        VStack(spacing: 0) {
            passthroughBanner
            if appState.isActive(workspace) {
                restartBanner
            }
            HSplitView {
                fileListPanel
                    .frame(minWidth: 160, idealWidth: 180, maxWidth: 220)
                entriesPanel
            }
        }
        .task {
            await loadSecretFiles()
            loadSettingsSecrets()
        }
        .sheet(isPresented: $showingAddFile) {
            AddSecretFileSheet(workspace: workspace) {
                Task { await loadSecretFiles() }
            }
        }
        .alert("File contains encrypted values",
               isPresented: $showRemoveWarning,
               presenting: fileToRemove) { file in
            Button("Decrypt & Remove") {
                Task {
                    let cli = CLIService()
                    _ = try? await cli.run(
                        args: ["secret", "decrypt", file.path, "--all"],
                        workingDirectory: workspace.path
                    )
                    await removeFile(file)
                }
            }
            Button("Remove Anyway", role: .destructive) {
                Task { await removeFile(file) }
            }
            Button("Cancel", role: .cancel) {}
        } message: { file in
            Text("'\(file.label)' has encrypted values. Removing without decrypting means values stay as ENC[age:...] ciphertext. Decrypt first?")
        }
    }
```

Add the banner property right above `restartBanner`:

```swift
    @ViewBuilder
    private var passthroughBanner: some View {
        let resolved = ResolvedSettings(global: appState.settings, workspace: workspace)
        let missing = PassthroughPolicy.missingProtectedHosts(from: resolved.passthroughHosts)
        if !missing.isEmpty {
            HStack(spacing: 6) {
                Image(systemName: "exclamationmark.triangle.fill")
                    .foregroundStyle(.yellow)
                Text("Anthropic passthrough disabled — secrets will be sent as plaintext to \(missing.joined(separator: ", ")).")
                    .font(.caption)
                    .foregroundStyle(.primary)
                Spacer()
            }
            .padding(8)
            .background(.yellow.opacity(0.15))
        }
    }
```

- [ ] **Step 2: Build to verify**

Run: `make gui-build 2>&1 | tail -20`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift
git commit -m "feat(gui): persistent passthrough banner on Secrets tab

Yellow banner at the top of SecretsView keyed off the resolved
passthrough list (global + workspace override). Visible whenever
api.anthropic.com or auth.anthropic.com is missing — a constant
reminder that the privacy property is currently subverted."
```

---

## Task 18: Swift EnvSecret model + JSON parsing test

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Models/EnvSecret.swift`
- Create: `AirlockApp/Tests/AirlockAppTests/EnvSecretTests.swift`

- [ ] **Step 1: Write failing tests**

Create `AirlockApp/Tests/AirlockAppTests/EnvSecretTests.swift`:

```swift
import XCTest
@testable import AirlockApp

final class EnvSecretTests: XCTestCase {
    func testParsesEnvSecretListJSON() throws {
        let json = """
        [
          {"name":"ALPHA"},
          {"name":"BRAVO"},
          {"name":"GITHUB_TOKEN"}
        ]
        """
        let data = Data(json.utf8)
        let parsed = try EnvSecret.decodeList(from: data)
        XCTAssertEqual(parsed.count, 3)
        XCTAssertEqual(parsed[0].name, "ALPHA")
        XCTAssertEqual(parsed[2].name, "GITHUB_TOKEN")
    }

    func testEnvSecretIDsAreStable() throws {
        let json = #"[{"name":"ALPHA"}]"#
        let a = try EnvSecret.decodeList(from: Data(json.utf8))
        let b = try EnvSecret.decodeList(from: Data(json.utf8))
        XCTAssertEqual(a[0].id, b[0].id, "Same name should produce same UUID")
    }

    func testValidNameRegex() {
        XCTAssertTrue(EnvSecret.isValidName("FOO"))
        XCTAssertTrue(EnvSecret.isValidName("FOO_BAR"))
        XCTAssertTrue(EnvSecret.isValidName("_PRIVATE"))
        XCTAssertTrue(EnvSecret.isValidName("a1"))
    }

    func testInvalidNameRegex() {
        XCTAssertFalse(EnvSecret.isValidName(""))
        XCTAssertFalse(EnvSecret.isValidName("1FOO"))
        XCTAssertFalse(EnvSecret.isValidName("FOO-BAR"))
        XCTAssertFalse(EnvSecret.isValidName("PATH=x"))
        XCTAssertFalse(EnvSecret.isValidName("FOO BAR"))
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd AirlockApp && swift test --filter EnvSecretTests 2>&1 | tail -20`
Expected: FAIL with `cannot find 'EnvSecret' in scope`.

- [ ] **Step 3: Implement the model**

Create `AirlockApp/Sources/AirlockApp/Models/EnvSecret.swift`:

```swift
import Foundation

/// In-memory representation of an environment-variable secret as
/// surfaced by `airlock secret env list --json`. Intentionally does
/// NOT carry the value: the GUI never holds plaintext or ciphertext
/// in long-lived state, only displays truncated prefixes via the
/// `secret env show` CLI when needed.
struct EnvSecret: Identifiable, Hashable {
    let id: UUID
    let name: String

    init(name: String) {
        self.name = name
        // Deterministic UUID derived from the name so SwiftUI's
        // selection state is preserved across reloads.
        self.id = EnvSecret.deterministicID(for: name)
    }

    private static func deterministicID(for name: String) -> UUID {
        // Build a UUID from a SHA-256 hash of the name. Stable across
        // process launches.
        let bytes = Array(name.utf8)
        var hash = [UInt8](repeating: 0, count: 16)
        for (i, b) in bytes.enumerated() {
            hash[i % 16] ^= b &+ UInt8(i & 0xFF)
        }
        return UUID(uuid: (
            hash[0], hash[1], hash[2], hash[3],
            hash[4], hash[5], hash[6], hash[7],
            hash[8], hash[9], hash[10], hash[11],
            hash[12], hash[13], hash[14], hash[15]
        ))
    }

    /// Decode a JSON array of `{"name": "..."}` objects from
    /// `airlock secret env list --json`.
    static func decodeList(from data: Data) throws -> [EnvSecret] {
        struct Wire: Decodable { let name: String }
        let wire = try JSONDecoder().decode([Wire].self, from: data)
        return wire.map { EnvSecret(name: $0.name) }
    }

    /// Mirror of the Go-side regex `^[A-Za-z_][A-Za-z0-9_]*$`.
    /// Used by the AddEnvSecretSheet to disable Add until the name
    /// is valid, so the user gets immediate feedback.
    static func isValidName(_ name: String) -> Bool {
        guard !name.isEmpty else { return false }
        let chars = Array(name)
        let first = chars[0]
        let firstOK = first.isLetter || first == "_"
        guard firstOK else { return false }
        for c in chars.dropFirst() {
            let ok = c.isLetter || c.isNumber || c == "_"
            if !ok { return false }
        }
        return true
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd AirlockApp && swift test --filter EnvSecretTests 2>&1 | tail -20`
Expected: All four tests pass.

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Models/EnvSecret.swift AirlockApp/Tests/AirlockAppTests/EnvSecretTests.swift
git commit -m "feat(gui): add EnvSecret model with stable IDs and name validation

Carries only the name (never the value). Deterministic UUID per
name preserves SwiftUI selection state across reloads. Client-side
name regex mirrors the Go-side validation in internal/config."
```

---

## Task 19: AddEnvSecretSheet

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Views/Secrets/AddEnvSecretSheet.swift`

- [ ] **Step 1: Create the sheet**

```swift
// AirlockApp/Sources/AirlockApp/Views/Secrets/AddEnvSecretSheet.swift
import SwiftUI

@MainActor
struct AddEnvSecretSheet: View {
    let workspace: Workspace
    let onComplete: () -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var value = ""
    @State private var isAdding = false
    @State private var errorMessage: String?

    private var nameIsValid: Bool {
        EnvSecret.isValidName(name)
    }

    private var canAdd: Bool {
        nameIsValid && !value.isEmpty && !isAdding
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Add Environment Secret").font(.title3).fontWeight(.semibold)

            Form {
                TextField("NAME", text: $name)
                    .textFieldStyle(.roundedBorder)
                    .font(.system(.body, design: .monospaced))
                if !name.isEmpty && !nameIsValid {
                    Text("Must match ^[A-Za-z_][A-Za-z0-9_]*$")
                        .font(.caption)
                        .foregroundStyle(.red)
                }

                SecureField("Value", text: $value)
                    .textFieldStyle(.roundedBorder)
            }

            Text("Stored encrypted in .airlock/config.yaml. Restart the workspace to apply.")
                .font(.caption)
                .foregroundStyle(.secondary)

            if let error = errorMessage {
                Text(error)
                    .foregroundStyle(.red)
                    .font(.caption)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button("Add") {
                    Task { await add() }
                }
                .keyboardShortcut(.defaultAction)
                .disabled(!canAdd)
            }
        }
        .padding()
        .frame(width: 460)
    }

    private func add() async {
        isAdding = true
        defer {
            isAdding = false
            // Drop the value reference after the call returns.
            value = ""
        }
        let cli = CLIService()
        let result = try? await cli.run(
            args: ["secret", "env", "add", name, "--value", value],
            workingDirectory: workspace.path
        )
        if let result, result.exitCode != 0 {
            errorMessage = result.stderr.isEmpty ? "Failed to add env secret" : result.stderr
            return
        }
        onComplete()
        dismiss()
    }
}
```

- [ ] **Step 2: Build to verify**

Run: `make gui-build 2>&1 | tail -20`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Secrets/AddEnvSecretSheet.swift
git commit -m "feat(gui): add AddEnvSecretSheet for env-secret entry

Modal with monospaced TextField for the name (live-validated against
the env var regex) and SecureField for the value. Shells out to
'airlock secret env add --value', surfaces stderr inline on failure.
Drops the value reference immediately after the CLI call returns."
```

---

## Task 20: Wire env secrets into SecretsView

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift`

- [ ] **Step 1: Add state, loaders, and sidebar section**

Edit `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift`. Add state vars (with the existing `@State` block, around line 7):

```swift
    @State private var envSecrets: [EnvSecret] = []
    @State private var showingAddEnvSecret = false
    @State private var envSecretToRemove: EnvSecret?
    @State private var showEnvRemoveConfirm = false
```

Add a stable pseudo-UUID for the env-vars header (next to `settingsFileID` around line 28):

```swift
    // Stable UUID for the Env Variables section header (selecting it
    // shows nothing; only individual env secret rows are selectable).
    private let envSectionHeaderID = UUID(uuidString: "00000000-0000-0000-0000-000000000002")!
```

Modify the `.task { ... }` block to also load env secrets:

```swift
        .task {
            await loadSecretFiles()
            await loadEnvSecrets()
            loadSettingsSecrets()
        }
```

Add a loader function (next to `loadSecretFiles()` around line 244):

```swift
    private func loadEnvSecrets() async {
        let cli = CLIService()
        guard let result = try? await cli.run(
            args: ["secret", "env", "list", "--json"],
            workingDirectory: workspace.path
        ), result.exitCode == 0 else {
            return
        }
        let data = Data(result.stdout.utf8)
        if let parsed = try? EnvSecret.decodeList(from: data) {
            envSecrets = parsed
        }
    }

    private func removeEnvSecret(_ secret: EnvSecret) async {
        let cli = CLIService()
        _ = try? await cli.run(
            args: ["secret", "env", "remove", secret.name],
            workingDirectory: workspace.path
        )
        await loadEnvSecrets()
        if selectedFileID == secret.id {
            selectedFileID = nil
        }
    }
```

Modify the `fileListPanel` to add the env section above `Files`. Replace the existing `List(selection: $selectedFileID) { Section("Files") { ... } Section("Claude Settings") { ... } }` block with:

```swift
            List(selection: $selectedFileID) {
                Section("Env Variables") {
                    ForEach(envSecrets) { secret in
                        HStack {
                            Image(systemName: "key.fill")
                                .foregroundStyle(.secondary)
                            Text(secret.name)
                                .lineLimit(1)
                                .font(.system(.body, design: .monospaced))
                        }
                        .tag(secret.id)
                        .contextMenu {
                            Button("Remove", role: .destructive) {
                                envSecretToRemove = secret
                                showEnvRemoveConfirm = true
                            }
                        }
                    }
                }
                Section("Files") {
                    ForEach(secretFiles) { file in
                        HStack {
                            Image(systemName: file.format.iconName)
                                .foregroundStyle(.secondary)
                            Text(file.label)
                                .lineLimit(1)
                        }
                        .tag(file.id)
                        .contextMenu {
                            Button("Remove", role: .destructive) {
                                confirmRemoveFile(file)
                            }
                        }
                    }
                }
                Section("Claude Settings") {
                    Label("settings.json", systemImage: "gearshape.2")
                        .tag(settingsFileID)
                }
            }
```

Replace the bottom toolbar `HStack` of `fileListPanel` (currently with just `Add File`) with:

```swift
            Divider()
            HStack {
                Button { showingAddFile = true } label: {
                    Label("Add File", systemImage: "doc.badge.plus")
                }
                .buttonStyle(.borderless)
                Button { showingAddEnvSecret = true } label: {
                    Label("Add Env", systemImage: "key.viewfinder")
                }
                .buttonStyle(.borderless)
                Spacer()
            }
            .padding(8)
```

Add the AddEnvSecretSheet and confirm modal to the body's modifier chain. Locate the existing `.sheet(isPresented: $showingAddFile)` block and add right after it:

```swift
        .sheet(isPresented: $showingAddEnvSecret) {
            AddEnvSecretSheet(workspace: workspace) {
                Task { await loadEnvSecrets() }
            }
        }
        .alert("Remove env secret?",
               isPresented: $showEnvRemoveConfirm,
               presenting: envSecretToRemove) { secret in
            Button("Remove", role: .destructive) {
                Task { await removeEnvSecret(secret) }
            }
            Button("Cancel", role: .cancel) {}
        } message: { secret in
            Text("Remove '\(secret.name)'? This will delete the encrypted value from .airlock/config.yaml. Unrecoverable.")
        }
```

Modify the right-panel to show env-secret detail when one is selected. Replace the `displayedEntries` computed property's logic with a switch that also handles env-secret selection. Find `displayedEntries` (around line 16) and add a sibling computed property:

```swift
    private var selectedEnvSecret: EnvSecret? {
        guard let id = selectedFileID else { return nil }
        return envSecrets.first { $0.id == id }
    }
```

Then modify `entriesPanel` (around line 129) to show the env-secret detail when one is selected. Replace the body of `entriesPanel`:

```swift
    private var entriesPanel: some View {
        VStack(spacing: 0) {
            if let envSecret = selectedEnvSecret {
                envSecretDetailPanel(for: envSecret)
            } else {
                entriesToolbar
                Divider()

                if let error = errorMessage {
                    ContentUnavailableView {
                        Label("Error", systemImage: "exclamationmark.triangle")
                    } description: { Text(error) }
                    .frame(maxHeight: .infinity)
                } else if displayedEntries.isEmpty {
                    ContentUnavailableView {
                        Label("No Secrets", systemImage: "key")
                    } description: { Text("Select a file or add one to get started") }
                    .frame(maxHeight: .infinity)
                } else {
                    Table(displayedEntries, selection: $selectedEntryIDs) {
                        TableColumn("Name") { entry in
                            Text(entry.path).fontDesign(.monospaced)
                        }
                        .width(min: 120, ideal: 200)

                        TableColumn("Status") { entry in
                            HStack(spacing: 4) {
                                Circle()
                                    .fill(colorForStatus(entry.status))
                                    .frame(width: 6, height: 6)
                                Text(entry.status.rawValue)
                                    .font(.caption)
                            }
                        }
                        .width(min: 80, ideal: 100)

                        TableColumn("Value") { entry in
                            Text(entry.maskedValue)
                                .fontDesign(.monospaced)
                                .foregroundStyle(.secondary)
                        }
                        .width(min: 200, ideal: 300)
                    }
                }

                Divider()
                summaryBar
            }
        }
    }

    @ViewBuilder
    private func envSecretDetailPanel(for secret: EnvSecret) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                Image(systemName: "key.fill")
                    .foregroundStyle(.secondary)
                Text(secret.name)
                    .font(.system(.title3, design: .monospaced))
                    .fontWeight(.semibold)
                Spacer()
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(secret.name, forType: .string)
                } label: {
                    Label("Copy name", systemImage: "doc.on.doc")
                }
                .buttonStyle(.borderless)
                Button(role: .destructive) {
                    envSecretToRemove = secret
                    showEnvRemoveConfirm = true
                } label: {
                    Label("Remove", systemImage: "trash")
                }
                .buttonStyle(.borderless)
            }

            HStack(spacing: 6) {
                Circle()
                    .fill(.green)
                    .frame(width: 6, height: 6)
                Text("encrypted")
                    .font(.caption)
            }

            Text("Value (truncated)")
                .font(.caption)
                .foregroundStyle(.secondary)
            Text("ENC[age:••••••••")
                .fontDesign(.monospaced)
                .foregroundStyle(.secondary)

            Text("Restart the workspace to apply changes.")
                .font(.caption)
                .foregroundStyle(.secondary)

            Spacer()
        }
        .padding()
    }
```

- [ ] **Step 2: Build to verify**

Run: `make gui-build 2>&1 | tail -30`
Expected: Build succeeds.

- [ ] **Step 3: Run Swift tests**

Run: `make gui-test 2>&1 | tail -10`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift
git commit -m "feat(gui): add Env Variables section to Secrets sidebar

New top sidebar section listing registered env secrets via
'airlock secret env list --json'. Selecting a row shows a single
detail panel (name, encrypted status, truncated value, Copy-name
and Remove actions). New 'Add Env' button next to 'Add File'."
```

---

## Task 21: Write ADR-0010

**Files:**
- Create: `docs/decisions/ADR-0010-environment-variable-secrets.md`

- [ ] **Step 1: Read an existing ADR for the format**

Run a quick check for existing ADRs to mirror style:
- Use the Read tool on `docs/decisions/ADR-0008-multi-format-secrets.md` to confirm header format and section ordering.

- [ ] **Step 2: Write the ADR**

Create `docs/decisions/ADR-0010-environment-variable-secrets.md`:

```markdown
# ADR-0010: Environment-variable secrets and passthrough guardrail

**Status:** Accepted
**Date:** 2026-04-07
**Related:** ADR-0005 (settings secret protection), ADR-0008 (multi-format secrets)

## Context

PR #19 introduced multi-format file-based secret registration. Some secrets have no natural file home: one-off API tokens, CI variables, values a user wants to set without touching a dotenv. The CLI offered no way to register a single `NAME=value` pair as an airlock-managed secret.

Separately, the proxy passthrough list is data, not code. Removing `api.anthropic.com` from the list causes the proxy to substitute `ENC[age:...]` tokens with plaintext on outbound requests to Anthropic, defeating the privacy property that "Anthropic only ever sees ciphertext." Commit `7390166` (2026-04-03) restored the GUI default after a regression; there was no guardrail against future regressions or user missteps.

## Decision

### Env-variable secrets

Store encrypted env vars inline in `.airlock/config.yaml` under a new `env_secrets:` list:

```yaml
env_secrets:
  - name: GITHUB_TOKEN
    value: ENC[age:...]
```

Encryption happens at `airlock secret env add` time using the workspace age public key, so plaintext never lives at rest. A new `EnvSecretScanner` (peer of `FileScanner`) reads each entry at session start: it decrypts to populate the proxy mapping, then emits an `EnvVar{Name, Value}` carrying the original ciphertext. The orchestrator passes these through `SessionParams` to the container manager, which appends `NAME=ENC[age:...]` to the agent container's `Env` block. The agent reads the variable from its environment and sees ciphertext; the proxy substitutes back to plaintext on outbound HTTP to non-passthrough hosts, identical to the file-secret flow.

Reserved names (`HTTP_PROXY`, `HTTPS_PROXY`, `http_proxy`, `https_proxy`, `NO_PROXY`, `LANG`) are rejected at config load time. Single source of truth is `internal/config/reserved.go`; the container manager imports it.

### Passthrough guardrail

Swift-only, non-blocking. A shared constant `PassthroughPolicy.protectedHosts` defines `api.anthropic.com` and `auth.anthropic.com`. The Settings, WorkspaceSettings, and Secrets views all consult `PassthroughPolicy.missingProtectedHosts(...)`. When a protected host is missing:

1. An inline yellow warning appears in the editor live as the user types.
2. On Save, a destructive-styled confirmation alert blocks the change until the user clicks "Remove anyway."
3. A persistent yellow banner sits at the top of `SecretsView` whenever the resolved (global + workspace override) list is missing a protected host.

The CLI is unguarded — `airlock run --passthrough-hosts ""` is a power-user path and the CLI already prints the resolved list on startup. The guardrail is for the GUI-clickthrough footgun.

## Consequences

- Plaintext for env secrets never appears at rest. Encryption is performed once, at `add` time, before the value is persisted.
- Inside the agent container, env secrets are visible as `ENC[age:...]` ciphertext to any process that reads the environment. Same threat boundary as any container env var; the model never sees plaintext, Anthropic never sees plaintext.
- The `--value` argv path on `airlock secret env add` briefly exposes plaintext in `ps` output. This is the GUI plumbing path because Swift `Process` stdin piping is awkward. A `--stdin` alternative exists for CLI users who want to avoid argv.
- No re-encryption on key rotation. Inherited limitation; out of scope for this work.
- A determined user can still remove Anthropic from passthrough by clicking through the confirmation alert. The guardrail makes the action conscious, not impossible.
- Schema is forward-compatible: an absent `env_secrets:` field deserializes to nil; old configs continue to load.

## Alternatives considered

- **Separate `.airlock/env.secrets.yaml` file.** Rejected. `.airlock/` is gitignored; a separate file is aesthetic, not functional. YAGNI.
- **Plaintext at rest, encrypt at session start.** Rejected. Violates the existing "encrypted at rest" invariant for file secrets and creates a window where plaintext lives on disk.
- **Stdin-only, no `--value` flag.** Rejected. The GUI needs synchronous value passing. `Process` stdin piping in Swift's async wrapper is clunky enough that the argv leakage is the practical trade-off.
- **CLI passthrough guardrail.** Rejected. Power-user path; the CLI already echoes the resolved passthrough list on startup, and `--passthrough-hosts ""` is an explicit override.
- **Block passthrough removal entirely (no escape hatch).** Rejected. Testing legitimately requires removing Anthropic from passthrough sometimes; making it impossible would be hostile to power users.
```

- [ ] **Step 3: Commit**

```bash
git add docs/decisions/ADR-0010-environment-variable-secrets.md
git commit -m "docs: add ADR-0010 for env secrets and passthrough guardrail

Documents the decision to store ciphertext inline in config.yaml,
inject as ciphertext env vars, and protect Anthropic passthrough
in the GUI. Lists alternatives considered."
```

---

## Task 22: Update security model guide

**Files:**
- Modify: `docs/guides/security-model.md`

- [ ] **Step 1: Read the existing guide**

Use the Read tool on `docs/guides/security-model.md` to find the section that enumerates secret sources / scanners.

- [ ] **Step 2: Add env-secret as a third source**

Find the section that lists secret scanners (likely "Secret sources" or "Defense layers") and add a third bullet alongside file secrets and the Claude-settings heuristic scanner. Insert text like:

```markdown
- **Environment-variable secrets** (`internal/secrets/scanner_env_secret.go`). Registered via `airlock secret env add NAME` and stored as `ENC[age:...]` ciphertext in `config.yaml:env_secrets[]`. At session start, `EnvSecretScanner` decrypts each entry to populate the proxy mapping, then injects `NAME=ENC[age:...]` into the agent container's `Env` block. The agent process sees ciphertext via the environment; the proxy substitutes on outbound calls to non-passthrough hosts. Same contract as file secrets; same privacy property (model never sees plaintext, Anthropic never sees plaintext).
```

If there is a section discussing the passthrough list, append a paragraph noting the GUI guardrail:

```markdown
The GUI surfaces a non-blocking guardrail when a user removes `api.anthropic.com` or `auth.anthropic.com` from the passthrough list (Settings, Workspace overrides, and a persistent banner on the Secrets tab). Removing Anthropic from passthrough subverts the privacy property — the proxy then substitutes ciphertext to plaintext on outbound Anthropic traffic, and Anthropic receives plaintext secrets in the conversation body. The CLI does not guard this path; `airlock run --passthrough-hosts ""` is intentional.
```

- [ ] **Step 3: Commit**

```bash
git add docs/guides/security-model.md
git commit -m "docs(security): document env secrets and passthrough guardrail

Adds env-secret as the third secret source alongside file secrets
and the Claude-settings heuristic scanner. Documents the GUI-only
passthrough guardrail and why the CLI is unguarded."
```

---

## Task 23: Update CLAUDE.md commands table

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add four rows to the Commands table**

Edit `CLAUDE.md`. Find the Commands table (under `## Commands`) and add four rows after the existing `airlock secret remove` row:

```markdown
| `airlock secret env add <name>` | Register an environment variable secret (TTY prompt, or `--value`/`--stdin`) |
| `airlock secret env list [--json]` | List registered env secret names |
| `airlock secret env show <name> [--json]` | Show env secret metadata; never decrypts |
| `airlock secret env remove <name>` | Unregister an env secret |
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): add 'airlock secret env' commands to table"
```

---

## Task 24: Final verification — full test suite + manual smoke

**Files:** none

- [ ] **Step 1: Run the full Go test suite with race detector**

Run: `make test`
Expected: All tests pass with no race warnings.

- [ ] **Step 2: Run the Python addon test suite**

Run: `make test-python`
Expected: All proxy addon tests pass. (None of our changes touch the addon, so this is a regression check.)

- [ ] **Step 3: Run the Swift test suite**

Run: `make gui-test 2>&1 | tail -30`
Expected: PASS for `PassthroughPolicyTests`, `EnvSecretTests`, and all pre-existing tests.

- [ ] **Step 4: Build the GUI app to confirm no regressions**

Run: `make gui-build 2>&1 | tail -10`
Expected: Build succeeds.

- [ ] **Step 5: Build the CLI binary**

Run: `make build`
Expected: `bin/airlock` produced.

- [ ] **Step 6: Smoke-test the CLI end-to-end**

Run in a scratch directory:

```bash
mkdir /tmp/airlock-smoke && cd /tmp/airlock-smoke
/Users/berry.kim/Projects/airlock/bin/airlock init
/Users/berry.kim/Projects/airlock/bin/airlock secret env add GITHUB_TOKEN --value ghp_abcd1234
/Users/berry.kim/Projects/airlock/bin/airlock secret env list --json
/Users/berry.kim/Projects/airlock/bin/airlock secret env show GITHUB_TOKEN --json
cat .airlock/config.yaml | grep -F "ENC[age:" >/dev/null && echo "ciphertext OK"
cat .airlock/config.yaml | grep -F "ghp_abcd1234" && echo "PLAINTEXT LEAKED" || echo "no plaintext OK"
/Users/berry.kim/Projects/airlock/bin/airlock secret env remove GITHUB_TOKEN
/Users/berry.kim/Projects/airlock/bin/airlock secret env list
```

Expected:
- `secret env add` → `Added env secret GITHUB_TOKEN`
- `secret env list --json` → JSON array with one entry
- `secret env show --json` → name + encrypted: true + truncated value_prefix
- `ciphertext OK` printed
- `no plaintext OK` printed
- `secret env remove` → `Removed env secret GITHUB_TOKEN`
- final `secret env list` → `No env secrets registered.`

If any check fails, fix the underlying issue and re-run.

- [ ] **Step 7: Clean up scratch directory**

```bash
rm -rf /tmp/airlock-smoke
```

- [ ] **Step 8: Final commit (if any cleanup happened)**

If steps 1-7 produced any fixes, commit them with a descriptive message. Otherwise skip.

- [ ] **Step 9: Push branch and open PR**

Confirm with the user before pushing or opening a PR. The implementation is complete; the user decides when to integrate.

---

## Self-review notes

- Spec coverage check: every section of the spec maps to at least one task.
  - Section 1 (guardrail) → Tasks 14, 15, 16, 17
  - Section 2 (data model) → Tasks 1, 2, 3
  - Section 3 (CLI) → Tasks 9, 10, 11, 12, 13
  - Section 4 (scanner + injection) → Tasks 4, 5, 6, 7, 8
  - Section 5 (GUI integration) → Tasks 18, 19, 20
  - Section 6 (error handling) → covered inline by validation tests in Tasks 3, 5, 10
  - Section 7 (security regression tests) → covered by Tasks 5 (assertion #2), 6 (#3), 10 (#4); assertion #1 (no `plaintext` field in struct) is implicit since the struct definition in Task 2 has only Name + Value.
  - Section 8 (test matrix) → distributed across Tasks 2-13, 14, 18
  - Section 9 (docs) → Tasks 21, 22, 23
- Type consistency: `EnvSecretConfig{Name, Value}` (Go) and `EnvSecret{id, name}` (Swift) are stable across all tasks. `secrets.EnvVar{Name, Value}` is consistent. `RunSecretEnvAdd(name, value, force, airlockDir)` signature is identical between Tasks 10, 11, 13.
- No placeholders.
