# Settings Secret Protection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Protect secrets in `.claude/` settings files by encrypting them with the existing `ENC[age:...]` pipeline, using a modular Scanner interface that supports multiple config formats.

**Architecture:** A `Scanner` interface abstracts secret discovery and encryption per config format. `ScanAll` orchestrates registered scanners, merging their shadow mounts and proxy mappings. Heuristic-based detection identifies secrets by key name and value prefix patterns. The existing `EnvShadowPath` single field is replaced by a `ShadowMounts` slice supporting arbitrary shadow bind mounts.

**Tech Stack:** Go 1.22+, `encoding/json`, `filippo.io/age`, `go test -race`

**Spec:** `docs/superpowers/specs/2026-03-27-settings-secret-protection.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/secrets/heuristic.go` | Create | `IsSecret(key, value) bool` heuristic detection |
| `internal/secrets/heuristic_test.go` | Create | Table-driven tests for heuristic |
| `internal/secrets/scanner.go` | Create | `Scanner` interface, `ScanOpts`, `ShadowMount`, `ScanResult`, `ScanAll` |
| `internal/secrets/scanner_test.go` | Create | `ScanAll` integration tests |
| `internal/secrets/scanner_claude.go` | Create | `ClaudeScanner` for `.claude/settings*.json` |
| `internal/secrets/scanner_claude_test.go` | Create | ClaudeScanner tests |
| `internal/secrets/scanner_env.go` | Create | `EnvScanner` wrapping existing `.env` logic |
| `internal/secrets/scanner_env_test.go` | Create | EnvScanner tests |
| `internal/container/manager.go` | Modify | Replace `EnvShadowPath` with `ShadowMounts` in `RunOpts` |
| `internal/container/manager_test.go` | Modify | Update shadow bind tests for `ShadowMounts` |
| `internal/orchestrator/session.go` | Modify | Replace `EnvShadowPath` with `ShadowMounts` in `SessionParams` |
| `internal/orchestrator/session_test.go` | Modify | Update shadow propagation tests for `ShadowMounts` |
| `internal/config/config.go` | Modify | Change default `PassthroughHosts` to `[]string{}` |
| `internal/config/config_test.go` | Modify | Update default config test |
| `internal/cli/run.go` | Modify | Replace env-specific code with scanner pipeline |
| `internal/cli/start.go` | Modify | Replace env-specific code with scanner pipeline |

---

### Task 1: Heuristic secret detection

**Files:**
- Create: `internal/secrets/heuristic.go`
- Create: `internal/secrets/heuristic_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/secrets/heuristic_test.go`:

```go
package secrets

import "testing"

func TestIsSecret(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		expect bool
	}{
		// Key name matches
		{name: "key contains TOKEN", key: "SLACK_TOKEN", value: "xoxb-1234-abcdef", expect: true},
		{name: "key contains KEY", key: "API_KEY", value: "some-long-api-value", expect: true},
		{name: "key contains SECRET", key: "WEBHOOK_SECRET", value: "whsec_abcdefghijk", expect: true},
		{name: "key contains PASSWORD", key: "DB_PASSWORD", value: "p4ssw0rd-long-enough", expect: true},
		{name: "key contains CREDENTIAL", key: "MY_CREDENTIAL", value: "cred-abcdefghijk", expect: true},
		{name: "key contains AUTH", key: "AUTH_BEARER", value: "bearer-token-value", expect: true},
		{name: "key case insensitive", key: "slack_token", value: "xoxb-1234-abcdef", expect: true},

		// Value prefix matches
		{name: "sk- prefix", key: "STRIPE", value: "sk-ant-api03-abcdefgh", expect: true},
		{name: "sk_live_ prefix", key: "STRIPE", value: "sk_live_abcdefghijk", expect: true},
		{name: "sk_test_ prefix", key: "STRIPE", value: "sk_test_abcdefghijk", expect: true},
		{name: "pk_live_ prefix", key: "STRIPE", value: "pk_live_abcdefghijk", expect: true},
		{name: "xoxb- prefix", key: "SLACK", value: "xoxb-123-456-abcdef", expect: true},
		{name: "xoxp- prefix", key: "SLACK", value: "xoxp-123-456-abcdef", expect: true},
		{name: "ghp_ prefix", key: "GH", value: "ghp_abcdefghijklmnop", expect: true},
		{name: "gho_ prefix", key: "GH", value: "gho_abcdefghijklmnop", expect: true},
		{name: "ghs_ prefix", key: "GH", value: "ghs_abcdefghijklmnop", expect: true},
		{name: "ghu_ prefix", key: "GH", value: "ghu_abcdefghijklmnop", expect: true},
		{name: "glpat- prefix", key: "GL", value: "glpat-abcdefghijklmn", expect: true},
		{name: "AKIA prefix", key: "AWS", value: "AKIAIOSFODNN7EXAMPLE", expect: true},
		{name: "eyJ prefix", key: "JWT", value: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", expect: true},
		{name: "whsec_ prefix", key: "WH", value: "whsec_abcdefghijklm", expect: true},
		{name: "rk_live_ prefix", key: "STRIPE", value: "rk_live_abcdefghijk", expect: true},
		{name: "rk_test_ prefix", key: "STRIPE", value: "rk_test_abcdefghijk", expect: true},

		// Exclusions
		{name: "short value", key: "API_KEY", value: "short", expect: false},
		{name: "boolean true", key: "AUTH_ENABLED", value: "true", expect: false},
		{name: "boolean false", key: "AUTH_ENABLED", value: "false", expect: false},
		{name: "number 0", key: "TOKEN_COUNT", value: "0", expect: false},
		{name: "number 1", key: "TOKEN_COUNT", value: "1", expect: false},
		{name: "path value", key: "SECRET_PATH", value: "/usr/local/bin/node", expect: false},
		{name: "url value", key: "AUTH_URL", value: "https://auth.example.com", expect: false},
		{name: "http url", key: "AUTH_URL", value: "http://localhost:8080", expect: false},

		// Not a secret
		{name: "generic key and value", key: "LOG_LEVEL", value: "production", expect: false},
		{name: "region value", key: "AWS_REGION", value: "us-east-1", expect: false},
		{name: "empty value", key: "API_KEY", value: "", expect: false},
		{name: "feature flag", key: "ENABLE_FEATURE", value: "enabled", expect: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSecret(tt.key, tt.value)
			if got != tt.expect {
				t.Errorf("IsSecret(%q, %q) = %v, want %v", tt.key, tt.value, got, tt.expect)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests -- verify compilation fails**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/secrets/... -run "TestIsSecret"
```

Expected: compilation error (`IsSecret` not defined)

- [ ] **Step 3: Implement heuristic**

Create `internal/secrets/heuristic.go`:

```go
package secrets

import "strings"

// secretKeywords are substrings that indicate a key name holds a secret.
var secretKeywords = []string{"token", "key", "secret", "password", "credential", "auth"}

// secretPrefixes are value prefixes that indicate a secret value.
var secretPrefixes = []string{
	"sk-", "sk_live_", "sk_test_", "pk_live_", "pk_test_",
	"xoxb-", "xoxp-", "xoxa-", "xoxr-",
	"ghp_", "gho_", "ghs_", "ghu_",
	"glpat-",
	"AKIA",
	"eyJ",
	"whsec_",
	"rk_live_", "rk_test_",
}

// IsSecret returns true if the key-value pair likely represents a secret
// based on heuristic rules. Exclusions are checked first, then positive signals.
func IsSecret(key, value string) bool {
	if isExcluded(value) {
		return false
	}
	return keyMatches(key) || valueMatches(value)
}

func isExcluded(value string) bool {
	if len(value) < 8 {
		return true
	}
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" {
		return true
	}
	if strings.HasPrefix(value, "/") {
		return true
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	return false
}

func keyMatches(key string) bool {
	lower := strings.ToLower(key)
	for _, kw := range secretKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func valueMatches(value string) bool {
	for _, prefix := range secretPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests -- verify they pass**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/secrets/... -run "TestIsSecret"
```

Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/heuristic.go internal/secrets/heuristic_test.go
git commit -m "feat: add heuristic secret detection for settings files"
```

---

### Task 2: Scanner interface and ScanAll orchestrator

**Files:**
- Create: `internal/secrets/scanner.go`
- Create: `internal/secrets/scanner_test.go`

- [ ] **Step 6: Write Scanner interface and ScanAll**

Create `internal/secrets/scanner.go`:

```go
package secrets

import "fmt"

// Scanner finds and encrypts secrets in a specific config format.
type Scanner interface {
	Name() string
	Scan(opts ScanOpts) (*ScanResult, error)
}

// ScanOpts holds parameters shared by all scanners.
type ScanOpts struct {
	Workspace  string
	HomeDir    string
	PublicKey  string
	PrivateKey string
	TmpDir     string
}

// ShadowMount describes a file-level Docker bind mount that shadows
// a plaintext file with its encrypted counterpart.
type ShadowMount struct {
	HostPath      string // processed file in tmpDir
	ContainerPath string // container path to shadow
}

// ScanResult holds the outputs from a scanner: shadow mounts and proxy mapping.
type ScanResult struct {
	Mounts  []ShadowMount
	Mapping map[string]string // ENC[age:...] -> plaintext
}

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
	}
	return merged, nil
}
```

- [ ] **Step 7: Write ScanAll tests**

Create `internal/secrets/scanner_test.go`:

```go
package secrets

import (
	"fmt"
	"testing"
)

type mockScanner struct {
	name   string
	result *ScanResult
	err    error
}

func (m *mockScanner) Name() string                        { return m.name }
func (m *mockScanner) Scan(opts ScanOpts) (*ScanResult, error) { return m.result, m.err }

func TestScanAllMergesResults(t *testing.T) {
	s1 := &mockScanner{
		name: "s1",
		result: &ScanResult{
			Mounts:  []ShadowMount{{HostPath: "/tmp/a.json", ContainerPath: "/workspace/a.json"}},
			Mapping: map[string]string{"ENC[age:aaa]": "secret_a"},
		},
	}
	s2 := &mockScanner{
		name: "s2",
		result: &ScanResult{
			Mounts:  []ShadowMount{{HostPath: "/tmp/b.json", ContainerPath: "/workspace/b.json"}},
			Mapping: map[string]string{"ENC[age:bbb]": "secret_b"},
		},
	}

	result, err := ScanAll([]Scanner{s1, s2}, ScanOpts{})
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}
	if len(result.Mounts) != 2 {
		t.Errorf("expected 2 mounts, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 2 {
		t.Errorf("expected 2 mapping entries, got %d", len(result.Mapping))
	}
	if result.Mapping["ENC[age:aaa]"] != "secret_a" {
		t.Error("missing mapping from s1")
	}
	if result.Mapping["ENC[age:bbb]"] != "secret_b" {
		t.Error("missing mapping from s2")
	}
}

func TestScanAllPropagatesError(t *testing.T) {
	s1 := &mockScanner{name: "good", result: &ScanResult{Mapping: map[string]string{}}}
	s2 := &mockScanner{name: "bad", err: fmt.Errorf("parse failed")}

	_, err := ScanAll([]Scanner{s1, s2}, ScanOpts{})
	if err == nil {
		t.Fatal("expected error from failing scanner")
	}
	if got := err.Error(); got != "scanner bad: parse failed" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestScanAllEmptyScanners(t *testing.T) {
	result, err := ScanAll([]Scanner{}, ScanOpts{})
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected 0 mapping entries, got %d", len(result.Mapping))
	}
}

func TestScanAllNilResult(t *testing.T) {
	s := &mockScanner{name: "nil", result: nil}
	result, err := ScanAll([]Scanner{s}, ScanOpts{})
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(result.Mounts))
	}
}
```

- [ ] **Step 8: Run tests -- verify they pass**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/secrets/... -run "TestScanAll"
```

Expected: ALL PASS

- [ ] **Step 9: Commit**

```bash
git add internal/secrets/scanner.go internal/secrets/scanner_test.go
git commit -m "feat: add Scanner interface and ScanAll orchestrator"
```

---

### Task 3: ClaudeScanner

**Files:**
- Create: `internal/secrets/scanner_claude.go`
- Create: `internal/secrets/scanner_claude_test.go`

- [ ] **Step 10: Write ClaudeScanner tests**

Create `internal/secrets/scanner_claude_test.go`:

```go
package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
)

func setupTestKeys(t *testing.T) (string, string) {
	t.Helper()
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return kp.PublicKey, kp.PrivateKey
}

func TestClaudeScannerMCPEnvSecrets(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	claudeDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	settings := map[string]any{
		"mcpServers": map[string]any{
			"slack": map[string]any{
				"command": "npx",
				"env": map[string]any{
					"SLACK_TOKEN": "xoxb-1234567890-abcdef",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(result.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(result.Mounts))
	}
	if result.Mounts[0].ContainerPath != "/home/airlock/.claude/settings.json" {
		t.Errorf("unexpected container path: %s", result.Mounts[0].ContainerPath)
	}

	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping entry, got %d", len(result.Mapping))
	}
	for enc, plain := range result.Mapping {
		if !crypto.IsEncrypted(enc) {
			t.Errorf("mapping key should be ENC pattern: %s", enc)
		}
		if plain != "xoxb-1234567890-abcdef" {
			t.Errorf("mapping value should be plaintext, got: %s", plain)
		}
	}

	// Verify processed file has ENC value
	processed, _ := os.ReadFile(result.Mounts[0].HostPath)
	if strings.Contains(string(processed), "xoxb-1234567890") {
		t.Error("processed file should not contain plaintext secret")
	}
	if !strings.Contains(string(processed), "ENC[age:") {
		t.Error("processed file should contain ENC pattern")
	}
	// Non-secret fields preserved
	if !strings.Contains(string(processed), "npx") {
		t.Error("processed file should preserve non-secret values")
	}
}

func TestClaudeScannerTopLevelEnv(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	claudeDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	settings := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_API_KEY": "sk-ant-api03-realkey12345",
			"FEATURE_FLAG":     "1",
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Only ANTHROPIC_API_KEY should be encrypted (key matches + value prefix)
	// FEATURE_FLAG="1" is excluded (length < 8)
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping entry, got %d: %v", len(result.Mapping), result.Mapping)
	}
	for _, plain := range result.Mapping {
		if plain != "sk-ant-api03-realkey12345" {
			t.Errorf("unexpected plaintext: %s", plain)
		}
	}
}

func TestClaudeScannerNoSecretsNoMount(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	claudeDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	settings := map[string]any{
		"env": map[string]any{
			"FEATURE_FLAG": "1",
			"LOG_LEVEL":    "debug",
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts when no secrets, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected 0 mapping when no secrets, got %d", len(result.Mapping))
	}
}

func TestClaudeScannerMissingFile(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	// No .claude directory created

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan should not fail on missing files: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts for missing files, got %d", len(result.Mounts))
	}
}

func TestClaudeScannerProjectAndGlobal(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	workspace := t.TempDir()

	// Global settings
	globalClaude := filepath.Join(homeDir, ".claude")
	os.MkdirAll(globalClaude, 0755)
	globalSettings := map[string]any{
		"env": map[string]any{"ANTHROPIC_API_KEY": "sk-ant-api03-globalkey123"},
	}
	data, _ := json.MarshalIndent(globalSettings, "", "  ")
	os.WriteFile(filepath.Join(globalClaude, "settings.json"), data, 0644)

	// Project settings
	projClaude := filepath.Join(workspace, ".claude")
	os.MkdirAll(projClaude, 0755)
	projSettings := map[string]any{
		"mcpServers": map[string]any{
			"github": map[string]any{
				"env": map[string]any{"GITHUB_TOKEN": "ghp_abcdefghijklmnopqrst"},
			},
		},
	}
	data, _ = json.MarshalIndent(projSettings, "", "  ")
	os.WriteFile(filepath.Join(projClaude, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(result.Mounts) != 2 {
		t.Errorf("expected 2 mounts (global + project), got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 2 {
		t.Errorf("expected 2 mapping entries, got %d", len(result.Mapping))
	}

	// Check container paths
	paths := make(map[string]bool)
	for _, m := range result.Mounts {
		paths[m.ContainerPath] = true
	}
	if !paths["/home/airlock/.claude/settings.json"] {
		t.Error("missing global settings mount")
	}
	if !paths["/workspace/.claude/settings.json"] {
		t.Error("missing project settings mount")
	}
}
```

- [ ] **Step 11: Run tests -- verify compilation fails**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/secrets/... -run "TestClaude"
```

Expected: compilation error (`NewClaudeScanner` not defined)

- [ ] **Step 12: Implement ClaudeScanner**

Create `internal/secrets/scanner_claude.go`:

```go
package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taeikkim92/airlock/internal/crypto"
)

type claudeSettingsFile struct {
	hostPath      string
	containerPath string
}

// ClaudeScanner encrypts secrets in .claude/settings.json and settings.local.json.
type ClaudeScanner struct{}

// NewClaudeScanner returns a scanner for Claude Code settings files.
func NewClaudeScanner() *ClaudeScanner {
	return &ClaudeScanner{}
}

func (s *ClaudeScanner) Name() string { return "claude" }

func (s *ClaudeScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	files := []claudeSettingsFile{
		{filepath.Join(opts.HomeDir, ".claude", "settings.json"), "/home/airlock/.claude/settings.json"},
		{filepath.Join(opts.HomeDir, ".claude", "settings.local.json"), "/home/airlock/.claude/settings.local.json"},
		{filepath.Join(opts.Workspace, ".claude", "settings.json"), "/workspace/.claude/settings.json"},
		{filepath.Join(opts.Workspace, ".claude", "settings.local.json"), "/workspace/.claude/settings.local.json"},
	}

	result := &ScanResult{Mapping: make(map[string]string)}

	for _, f := range files {
		mounts, mapping, err := s.processFile(f, opts)
		if err != nil {
			return nil, fmt.Errorf("process %s: %w", f.hostPath, err)
		}
		result.Mounts = append(result.Mounts, mounts...)
		for k, v := range mapping {
			result.Mapping[k] = v
		}
	}

	return result, nil
}

func (s *ClaudeScanner) processFile(f claudeSettingsFile, opts ScanOpts) ([]ShadowMount, map[string]string, error) {
	data, err := os.ReadFile(f.hostPath)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("parse JSON: %w", err)
	}

	mapping := make(map[string]string)
	modified := false

	// Process top-level "env" block
	if envBlock, ok := root["env"].(map[string]any); ok {
		if encryptEnvBlock(envBlock, opts.PublicKey, mapping) {
			modified = true
		}
	}

	// Process "mcpServers" -> each server's "env" block
	if mcpServers, ok := root["mcpServers"].(map[string]any); ok {
		for _, serverVal := range mcpServers {
			server, ok := serverVal.(map[string]any)
			if !ok {
				continue
			}
			if envBlock, ok := server["env"].(map[string]any); ok {
				if encryptEnvBlock(envBlock, opts.PublicKey, mapping) {
					modified = true
				}
			}
		}
	}

	if !modified {
		return nil, nil, nil
	}

	// Write processed JSON to tmpDir
	processed, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal: %w", err)
	}

	baseName := filepath.Base(f.hostPath)
	prefix := "global-"
	if f.containerPath[:10] == "/workspace" {
		prefix = "proj-"
	}
	tmpPath := filepath.Join(opts.TmpDir, prefix+baseName)
	if err := os.WriteFile(tmpPath, processed, 0644); err != nil {
		return nil, nil, fmt.Errorf("write: %w", err)
	}

	mount := ShadowMount{HostPath: tmpPath, ContainerPath: f.containerPath}
	return []ShadowMount{mount}, mapping, nil
}

// encryptEnvBlock encrypts secret values in an env map in-place.
// Returns true if any value was encrypted.
func encryptEnvBlock(envBlock map[string]any, publicKey string, mapping map[string]string) bool {
	modified := false
	for key, val := range envBlock {
		value, ok := val.(string)
		if !ok {
			continue
		}
		if crypto.IsEncrypted(value) {
			continue
		}
		if !IsSecret(key, value) {
			continue
		}
		ciphertext, err := crypto.Encrypt(value, publicKey)
		if err != nil {
			continue
		}
		wrapped := crypto.WrapENC(ciphertext)
		envBlock[key] = wrapped
		mapping[wrapped] = value
		modified = true
	}
	return modified
}
```

- [ ] **Step 13: Run tests -- verify they pass**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/secrets/... -run "TestClaude"
```

Expected: ALL PASS

- [ ] **Step 14: Commit**

```bash
git add internal/secrets/scanner_claude.go internal/secrets/scanner_claude_test.go
git commit -m "feat: add ClaudeScanner for .claude/settings.json secret encryption"
```

---

### Task 4: EnvScanner

**Files:**
- Create: `internal/secrets/scanner_env.go`
- Create: `internal/secrets/scanner_env_test.go`

- [ ] **Step 15: Write EnvScanner tests**

Create `internal/secrets/scanner_env_test.go`:

```go
package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestEnvScannerEncryptsAndMounts(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	tmpDir := t.TempDir()
	workspace := t.TempDir()

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("STRIPE_KEY=sk_live_abcdefghijk\nLOG=debug\n"), 0644)

	scanner := NewEnvScanner(envPath, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, HomeDir: t.TempDir(),
		PublicKey: kp.PublicKey, PrivateKey: kp.PrivateKey, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// env.enc bind + shadow mount for .env inside workspace = 2 mounts
	if len(result.Mounts) != 2 {
		t.Errorf("expected 2 mounts, got %d: %v", len(result.Mounts), result.Mounts)
	}

	// Should have mapping entries for both KEY=VALUE pairs (all .env values are encrypted)
	if len(result.Mapping) == 0 {
		t.Error("expected mapping entries")
	}

	// Check shadow mount for workspace-internal .env
	hasShadow := false
	for _, m := range result.Mounts {
		if m.ContainerPath == "/workspace/.env" {
			hasShadow = true
		}
	}
	if !hasShadow {
		t.Error("expected shadow mount for /workspace/.env")
	}
}

func TestEnvScannerOutsideWorkspace(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	tmpDir := t.TempDir()
	workspace := t.TempDir()
	externalDir := t.TempDir()

	envPath := filepath.Join(externalDir, "secrets.env")
	os.WriteFile(envPath, []byte("API_KEY=sk-ant-test-12345678\n"), 0644)

	scanner := NewEnvScanner(envPath, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, HomeDir: t.TempDir(),
		PublicKey: kp.PublicKey, PrivateKey: kp.PrivateKey, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Only env.enc bind, no shadow (file is outside workspace)
	if len(result.Mounts) != 1 {
		t.Errorf("expected 1 mount (env.enc only), got %d", len(result.Mounts))
	}
	for _, m := range result.Mounts {
		if strings.Contains(m.ContainerPath, "/workspace/") {
			t.Errorf("should not have workspace shadow for external file: %s", m.ContainerPath)
		}
	}
}

func TestEnvScannerEmptyPath(t *testing.T) {
	scanner := NewEnvScanner("", "")
	result, err := scanner.Scan(ScanOpts{TmpDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts for empty path, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected 0 mapping for empty path, got %d", len(result.Mapping))
	}
}
```

- [ ] **Step 16: Implement EnvScanner**

Create `internal/secrets/scanner_env.go`:

```go
package secrets

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EnvScanner encrypts .env files and produces shadow mounts.
type EnvScanner struct {
	envFilePath string
	workspace   string
}

// NewEnvScanner returns a scanner for a specific .env file.
func NewEnvScanner(envFilePath, workspace string) *EnvScanner {
	return &EnvScanner{envFilePath: envFilePath, workspace: workspace}
}

func (s *EnvScanner) Name() string { return "env" }

func (s *EnvScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	if s.envFilePath == "" {
		return &ScanResult{Mapping: make(map[string]string)}, nil
	}

	entries, err := ParseEnvFile(s.envFilePath)
	if err != nil {
		return nil, fmt.Errorf("parse env file: %w", err)
	}

	encResult, err := EncryptEntries(entries, opts.PublicKey, opts.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt entries: %w", err)
	}

	encPath := filepath.Join(opts.TmpDir, "env.enc")
	if err := WriteEnvFile(encPath, encResult.Encrypted); err != nil {
		return nil, fmt.Errorf("write encrypted env: %w", err)
	}

	result := &ScanResult{Mapping: encResult.Mapping}

	// Mount encrypted env file at /run/airlock/env.enc
	result.Mounts = append(result.Mounts, ShadowMount{
		HostPath:      encPath,
		ContainerPath: "/run/airlock/env.enc",
	})

	// Shadow original .env if inside workspace
	absEnvFile, err := filepath.Abs(s.envFilePath)
	if err == nil {
		absWorkspace, _ := filepath.Abs(s.workspace)
		rel, relErr := filepath.Rel(absWorkspace, absEnvFile)
		if relErr == nil && !strings.HasPrefix(rel, "..") {
			result.Mounts = append(result.Mounts, ShadowMount{
				HostPath:      encPath,
				ContainerPath: "/workspace/" + filepath.ToSlash(rel),
			})
		}
	}

	return result, nil
}
```

- [ ] **Step 17: Run tests -- verify they pass**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/secrets/... -run "TestEnvScanner"
```

Expected: ALL PASS

- [ ] **Step 18: Commit**

```bash
git add internal/secrets/scanner_env.go internal/secrets/scanner_env_test.go
git commit -m "feat: add EnvScanner wrapping .env encryption with shadow mount"
```

---

### Task 5: Replace EnvShadowPath with ShadowMounts

**Files:**
- Modify: `internal/container/manager.go`
- Modify: `internal/container/manager_test.go`
- Modify: `internal/orchestrator/session.go`
- Modify: `internal/orchestrator/session_test.go`

- [ ] **Step 19: Update RunOpts in manager.go**

In `internal/container/manager.go`:

Replace `EnvShadowPath` field with `ShadowMounts` and add import. Replace the single shadow bind logic with a loop.

Replace:
```go
import (
	"fmt"
	"strings"
)
```
With:
```go
import (
	"fmt"
	"strings"

	"github.com/taeikkim92/airlock/internal/secrets"
)
```

In the `RunOpts` struct, replace `EnvShadowPath string` with:
```go
ShadowMounts []secrets.ShadowMount
```

Remove `EnvFilePath` field (EnvScanner now handles this via ShadowMounts).

In `BuildClaudeConfig`, remove the `EnvFilePath` bind block and the `EnvShadowPath` block. Replace with a single loop after the CACertPath block:

```go
for _, m := range opts.ShadowMounts {
	binds = append(binds, fmt.Sprintf("%s:%s:ro", m.HostPath, m.ContainerPath))
}
```

Also remove `EnvFilePath` from `BuildProxyConfig` -- the mapping path is still needed but env file bind is now a ShadowMount.

- [ ] **Step 20: Update manager_test.go**

Replace the 3 `EnvShadowPath`-specific tests with `ShadowMounts` tests. Update `TestBuildClaudeConfigAllBindMounts` and `TestBuildClaudeConfigWithoutOptionalFields` to remove `EnvFilePath` and use `ShadowMounts` instead.

Tests that set `EnvFilePath` should now use `ShadowMounts` with an entry for `/run/airlock/env.enc`.

- [ ] **Step 21: Update SessionParams in session.go**

In `internal/orchestrator/session.go`:

Add import:
```go
"github.com/taeikkim92/airlock/internal/secrets"
```

Replace `EnvFilePath` and `EnvShadowPath` fields in `SessionParams` with:
```go
ShadowMounts []secrets.ShadowMount
MappingPath  string
```

In both `StartSession` and `StartDetachedSession`, replace `EnvFilePath` and `EnvShadowPath` in RunOpts with:
```go
ShadowMounts: params.ShadowMounts,
```

- [ ] **Step 22: Update session_test.go**

Update tests that set `EnvFilePath`/`EnvShadowPath` to use `ShadowMounts` instead. Update assertions to check `ShadowMounts` loop in container configs.

- [ ] **Step 23: Run all tests**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/...
```

Expected: ALL PASS

- [ ] **Step 24: Commit**

```bash
git add internal/container/manager.go internal/container/manager_test.go \
  internal/orchestrator/session.go internal/orchestrator/session_test.go
git commit -m "refactor: replace EnvShadowPath with ShadowMounts slice

Generalizes shadow bind mounts from a single env file to an arbitrary
list of shadow mounts produced by the scanner pipeline."
```

---

### Task 6: Change default passthrough to empty

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 25: Update default config**

In `internal/config/config.go`, change `Default()`:

Replace:
```go
PassthroughHosts: []string{
	"api.anthropic.com",
	"auth.anthropic.com",
},
```
With:
```go
PassthroughHosts: []string{},
```

- [ ] **Step 26: Update config test**

In `internal/config/config_test.go`, update `TestDefaultConfig` if it asserts on `PassthroughHosts` containing Anthropic hosts. Change to assert empty list.

- [ ] **Step 27: Run config tests**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/config/...
```

Expected: ALL PASS

- [ ] **Step 28: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: remove default passthrough hosts for full proxy coverage

All outbound traffic now goes through the decryption proxy by default.
Enables encrypted ANTHROPIC_API_KEY and other top-level env secrets.
Existing config.yaml files are not affected."
```

---

### Task 7: Integrate scanner pipeline into CLI

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/start.go`

- [ ] **Step 29: Rewrite run.go with scanner pipeline**

Replace the entire env-file-specific section in `run.go` with the scanner pipeline. The new flow:

1. Always attempt to load keypair (skip scanners if keys don't exist)
2. Build scanner list (ClaudeScanner always, EnvScanner if `--env` provided)
3. Call `ScanAll`
4. Save mapping if non-empty
5. Set `params.ShadowMounts` and `params.MappingPath`

Replace the body of the `RunE` function in `runCmd` with:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	airlockDir := ".airlock"
	keysDir := filepath.Join(airlockDir, "keys")

	cfg, err := config.Load(airlockDir)
	if err != nil {
		return fmt.Errorf("load config (run 'airlock init' first): %w", err)
	}

	workspace := runWorkspace
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	workspace, _ = filepath.Abs(workspace)

	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	tmpDir, _ := os.MkdirTemp("", "airlock-*")
	defer os.RemoveAll(tmpDir)

	params := orchestrator.SessionParams{
		Workspace: workspace,
		ClaudeDir: claudeDir,
		Config:    cfg,
		TmpDir:    tmpDir,
	}

	// Run scanner pipeline if keypair exists
	kp, kpErr := crypto.LoadKeyPair(keysDir)
	if kpErr == nil {
		scanners := []secrets.Scanner{
			secrets.NewClaudeScanner(),
		}
		if runEnvFile != "" {
			scanners = append(scanners, secrets.NewEnvScanner(runEnvFile, workspace))
		}
		scanResult, err := secrets.ScanAll(scanners, secrets.ScanOpts{
			Workspace:  workspace,
			HomeDir:    homeDir,
			PublicKey:  kp.PublicKey,
			PrivateKey: kp.PrivateKey,
			TmpDir:     tmpDir,
		})
		if err != nil {
			return fmt.Errorf("scan secrets: %w", err)
		}
		params.ShadowMounts = scanResult.Mounts
		if len(scanResult.Mapping) > 0 {
			mappingPath, mappingErr := secrets.SaveMapping(scanResult.Mapping, tmpDir)
			if mappingErr != nil {
				return fmt.Errorf("save mapping: %w", mappingErr)
			}
			params.MappingPath = mappingPath
		}
	}

	docker, err := container.NewDocker()
	if err != nil {
		return fmt.Errorf("docker init: %w", err)
	}
	defer docker.Close()

	err = orchestrator.StartSession(ctx, docker, params)
	orchestrator.CleanupSession(ctx, docker, cfg, "")
	return err
},
```

Update imports to include `secrets`.

- [ ] **Step 30: Rewrite start.go with scanner pipeline**

Replace the env-file-specific section in `RunStart` with the same scanner pipeline pattern. The function signature stays the same.

```go
func RunStart(ctx context.Context, runtime container.ContainerRuntime, id, workspace, envFile, airlockDir string) (*StartResult, error) {
	keysDir := filepath.Join(airlockDir, "keys")

	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config (run 'airlock init' first): %w", err)
	}

	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	workspace, _ = filepath.Abs(workspace)

	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	tmpDir, err := os.MkdirTemp("", "airlock-"+id+"-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	params := orchestrator.SessionParams{
		ID:        id,
		Workspace: workspace,
		ClaudeDir: claudeDir,
		Config:    cfg,
		TmpDir:    tmpDir,
	}

	kp, kpErr := crypto.LoadKeyPair(keysDir)
	if kpErr == nil {
		scanners := []secrets.Scanner{
			secrets.NewClaudeScanner(),
		}
		if envFile != "" {
			scanners = append(scanners, secrets.NewEnvScanner(envFile, workspace))
		}
		scanResult, err := secrets.ScanAll(scanners, secrets.ScanOpts{
			Workspace:  workspace,
			HomeDir:    homeDir,
			PublicKey:  kp.PublicKey,
			PrivateKey: kp.PrivateKey,
			TmpDir:     tmpDir,
		})
		if err != nil {
			return nil, fmt.Errorf("scan secrets: %w", err)
		}
		params.ShadowMounts = scanResult.Mounts
		if len(scanResult.Mapping) > 0 {
			mappingPath, mappingErr := secrets.SaveMapping(scanResult.Mapping, tmpDir)
			if mappingErr != nil {
				return nil, fmt.Errorf("save mapping: %w", mappingErr)
			}
			params.MappingPath = mappingPath
		}
	}

	if err := orchestrator.StartDetachedSession(ctx, runtime, params); err != nil {
		return nil, err
	}

	networkName := cfg.NetworkName + "-" + id

	return &StartResult{
		Status:    "running",
		Container: "airlock-claude-" + id,
		Proxy:     "airlock-proxy-" + id,
		Network:   networkName,
	}, nil
}
```

Update imports to include `secrets`.

- [ ] **Step 31: Run full test suite**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -v -race ./internal/...
```

Expected: ALL PASS

- [ ] **Step 32: Commit**

```bash
git add internal/cli/run.go internal/cli/start.go
git commit -m "feat: integrate scanner pipeline into CLI commands

Both run and start commands now use ScanAll with ClaudeScanner and
EnvScanner. Keypair is loaded eagerly. Settings file secrets are
encrypted alongside .env secrets in a unified pipeline."
```

---

### Task 8: Final verification

- [ ] **Step 33: Run full Go test suite**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && go test -race -cover ./internal/...
```

Expected: ALL PASS, no regressions.

- [ ] **Step 34: Run Python proxy tests**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && make test-python
```

Expected: ALL 29 PASS (no Python changes in this feature).

- [ ] **Step 35: Verify build**

```bash
cd /home/taeikkim92/Projects/quarantine-claude-code && make build
```

Expected: Binary builds successfully.
