# GUI-Primary Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform airlock from a CLI-first tool to a GUI-primary desktop app with multi-workspace support, container-based terminals, secrets management, and proxy activity logging.

**Architecture:** The Go CLI gains `start`, `stop --id`, and `status` commands for detached container management. The macOS GUI uses SwiftTerm to spawn `docker exec` subprocesses into running containers, with split-pane terminal support. All Docker interaction from Swift is via subprocess (never Docker API directly). The proxy addon gains structured JSON logging for activity monitoring.

**Tech Stack:** Go 1.22+ (CLI engine), Swift 5.9 / SwiftUI / macOS 14+ (GUI), SwiftTerm (terminal), Python 3 / mitmproxy (proxy addon), Docker Engine SDK (Go)

**Spec:** [docs/superpowers/specs/2026-03-25-gui-primary-redesign.md](../specs/2026-03-25-gui-primary-redesign.md)

---

## Branch Strategy

- **Phase 1-2 (Go + Python):** Completed on `main`. All tests pass.
- **Phase 3-7 (Swift GUI):** Work on `feat/gui-primary-redesign` branch. Requires macOS + Xcode for build verification (`make gui-build && make gui-test`). Merge to `main` after all tasks pass.

```bash
git checkout -b feat/gui-primary-redesign
# ... implement Tasks 6-15 ...
# After all tasks pass:
git checkout main && git merge feat/gui-primary-redesign
```

---

## Phase 1: CLI Engine -- New Commands (Go)

### Task 1: Parameterize container/network names with workspace ID

**Files:**
- Modify: `internal/container/manager.go`
- Modify: `internal/orchestrator/session.go`
- Test: `internal/container/manager_test.go`
- Test: `internal/orchestrator/session_test.go`

- [ ] **Step 1: Write failing tests for ID-based naming (TDD: RED)**

In `internal/container/manager_test.go`:

```go
func TestBuildProxyConfigWithID(t *testing.T) {
	opts := container.RunOpts{
		ID: "abc123", ProxyImage: "airlock-proxy:latest",
		NetworkName: "airlock-net-abc123", MappingPath: "/tmp/m.json",
		ProxyPort: 8080, PassthroughHosts: []string{"api.anthropic.com"},
	}
	cfg := container.BuildProxyConfig(opts)
	if cfg.Name != "airlock-proxy-abc123" {
		t.Errorf("expected airlock-proxy-abc123, got %s", cfg.Name)
	}
}

func TestBuildClaudeConfigWithID(t *testing.T) {
	opts := container.RunOpts{
		ID: "abc123", Workspace: "/tmp/ws", Image: "airlock-claude:latest",
		NetworkName: "airlock-net-abc123", ClaudeDir: "/home/user/.claude",
		ProxyPort: 8080,
	}
	cfg := container.BuildClaudeConfig(opts)
	if cfg.Name != "airlock-claude-abc123" {
		t.Errorf("expected airlock-claude-abc123, got %s", cfg.Name)
	}
	// Verify proxy URL uses ID-based hostname
	hasProxy := false
	for _, env := range cfg.Env {
		if env == "HTTP_PROXY=http://airlock-proxy-abc123:8080" {
			hasProxy = true
		}
	}
	if !hasProxy {
		t.Error("HTTP_PROXY should reference airlock-proxy-abc123")
	}
}

func TestBuildProxyConfigWithoutID(t *testing.T) {
	opts := container.RunOpts{
		ProxyImage: "airlock-proxy:latest", NetworkName: "airlock-net",
		MappingPath: "/tmp/m.json", ProxyPort: 8080,
		PassthroughHosts: []string{"api.anthropic.com"},
	}
	cfg := container.BuildProxyConfig(opts)
	if cfg.Name != "airlock-proxy" {
		t.Errorf("empty ID should produce airlock-proxy, got %s", cfg.Name)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
make test
```

Expected: FAIL -- `RunOpts` has no `ID` field.

- [ ] **Step 3: Implement ID-based naming (TDD: GREEN)**

In `internal/container/manager.go`:
1. Add `ID string` field to `RunOpts`
2. In `BuildProxyConfig`: use `"airlock-proxy-" + opts.ID` when ID is non-empty
3. In `BuildClaudeConfig`: use `"airlock-claude-" + opts.ID` when ID is non-empty
4. Update proxy URL hostname to match container name

- [ ] **Step 4: Run tests to verify they pass**

```bash
make test
```

Expected: PASS

- [ ] **Step 5: Update `SessionParams`, orchestrator, and ALL callers together**

In `internal/orchestrator/session.go`, add `ID` to `SessionParams`:

```go
type SessionParams struct {
	ID          string // workspace identifier
	Workspace   string
	// ... existing fields
}
```

Pass `ID` to `RunOpts`:

```go
opts := container.RunOpts{
    ID:               params.ID,
    // ... existing fields
}
```

Update `WaitForFile` and `CopyFromContainer` calls to use ID-based proxy name:

```go
proxyName := "airlock-proxy"
if params.ID != "" {
    proxyName = "airlock-proxy-" + params.ID
}
if err := runtime.WaitForFile(ctx, proxyName, mitmproxyCAPath, maxCAWaitRetries); err != nil {
```

Update `CleanupSession` to accept ID:

```go
func CleanupSession(ctx context.Context, runtime container.ContainerRuntime, cfg config.Config, id string) {
	claudeName := "airlock-claude"
	proxyName := "airlock-proxy"
	if id != "" {
		claudeName = "airlock-claude-" + id
		proxyName = "airlock-proxy-" + id
	}
	networkName := cfg.NetworkName
	if id != "" {
		networkName = cfg.NetworkName + "-" + id
	}
	fmt.Println("\n--- Session ended. Cleaning up...")
	runtime.Remove(ctx, claudeName)
	runtime.Remove(ctx, proxyName)
	runtime.RemoveNetwork(ctx, networkName)
}
```

- [ ] **Step 6: Update ALL callers and tests in one step (prevents compile errors)**

Update simultaneously -- all files must be changed together before the code compiles:

In `internal/cli/run.go`: `orchestrator.CleanupSession(ctx, docker, cfg, "")`
In `internal/cli/stop.go`: `orchestrator.CleanupSession(ctx, docker, cfg, "")`
In `internal/orchestrator/session_test.go`: Update all `CleanupSession` calls to pass empty ID `""`.
Also update `FailingMockRuntime.Remove/RemoveNetwork` if they reference the cleanup function.

- [ ] **Step 7: Run tests**

```bash
make test
```

Expected: All existing tests pass with no regressions.

- [ ] **Step 8: Commit**

```bash
git add internal/container/manager.go internal/container/manager_test.go \
       internal/orchestrator/session.go internal/orchestrator/session_test.go \
       internal/cli/run.go internal/cli/stop.go
git commit -m "refactor: parameterize container/network names with workspace ID"
```

---

### Task 2: Create `airlock start` command

**Files:**
- Create: `internal/cli/start.go`
- Create: `container/entrypoint-keepalive.sh`
- Modify: `internal/cli/root.go`
- Modify: `internal/container/manager.go`
- Test: `internal/orchestrator/session_test.go`

- [ ] **Step 1: Create keep-alive entrypoint script**

Create `container/entrypoint-keepalive.sh`:

```bash
#!/bin/bash
set -e

# Load encrypted env file if mounted.
if [ -f /run/airlock/env.enc ]; then
    set -a
    source /run/airlock/env.enc
    set +a
fi

# Trust the custom CA cert for proxy MITM
if [ -f /usr/local/share/ca-certificates/airlock-proxy.crt ]; then
    update-ca-certificates 2>/dev/null || true
    export NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/airlock-proxy.crt
fi

# Keep container alive for docker exec sessions
exec tail -f /dev/null
```

- [ ] **Step 2: Update Dockerfile to include keep-alive entrypoint**

In `container/Dockerfile`, add after the existing COPY of entrypoint.sh:

```dockerfile
COPY entrypoint-keepalive.sh /usr/local/bin/entrypoint-keepalive.sh
RUN chmod +x /usr/local/bin/entrypoint-keepalive.sh
```

- [ ] **Step 3: Write failing test for `StartDetachedSession` (TDD: RED)**

In `internal/orchestrator/session_test.go`, add the test from Step 5 below FIRST, then run `make test` to confirm it fails (function does not exist yet).

- [ ] **Step 4: Add `BuildClaudeDetachedConfig` to manager.go**

In `internal/container/manager.go`:

```go
// BuildClaudeDetachedConfig returns a ContainerConfig for a keep-alive agent container.
// Unlike BuildClaudeConfig, this container stays alive waiting for docker exec sessions.
func BuildClaudeDetachedConfig(opts RunOpts) ContainerConfig {
	cfg := BuildClaudeConfig(opts)
	cfg.Cmd = []string{"/usr/local/bin/entrypoint-keepalive.sh"}
	cfg.Tty = false
	cfg.Stdin = false
	return cfg
}
```

- [ ] **Step 4: Add `StartDetachedSession` to orchestrator**

In `internal/orchestrator/session.go`:

```go
// StartDetachedSession creates the network, starts proxy and agent containers in
// detached mode, and returns immediately. The agent container uses a keep-alive
// entrypoint instead of running Claude directly.
func StartDetachedSession(ctx context.Context, runtime container.ContainerRuntime, params SessionParams) error {
	cfg := params.Config
	networkName := cfg.NetworkName
	if params.ID != "" {
		networkName = networkName + "-" + params.ID
	}

	fmt.Println("Creating airlock network...")
	netOpts := container.NetworkConfig(networkName)
	_, err := runtime.EnsureNetwork(ctx, netOpts)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	opts := container.RunOpts{
		ID:               params.ID,
		Workspace:        params.Workspace,
		Image:            cfg.ContainerImage,
		ProxyImage:       cfg.ProxyImage,
		NetworkName:      networkName,
		EnvFilePath:      params.EnvFilePath,
		MappingPath:      params.MappingPath,
		ClaudeDir:        params.ClaudeDir,
		ProxyPort:        cfg.ProxyPort,
		PassthroughHosts: cfg.PassthroughHosts,
	}

	proxyName := "airlock-proxy"
	if params.ID != "" {
		proxyName = "airlock-proxy-" + params.ID
	}

	fmt.Println("Starting decryption proxy...")
	proxyCfg := container.BuildProxyConfig(opts)
	proxyID, err := runtime.RunDetached(ctx, proxyCfg)
	if err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}
	runtime.ConnectNetwork(ctx, "bridge", proxyID)

	fmt.Println("Waiting for proxy CA certificate...")
	if err := runtime.WaitForFile(ctx, proxyName, mitmproxyCAPath, maxCAWaitRetries); err != nil {
		return fmt.Errorf("proxy CA cert not ready: %w", err)
	}

	caCertDst := filepath.Join(params.TmpDir, "mitmproxy-ca-cert.pem")
	if err := runtime.CopyFromContainer(ctx, proxyName, mitmproxyCAPath, caCertDst); err != nil {
		return fmt.Errorf("extract proxy CA cert: %w", err)
	}
	opts.CACertPath = caCertDst

	fmt.Println("Starting agent container...")
	claudeCfg := container.BuildClaudeDetachedConfig(opts)
	if _, err := runtime.RunDetached(ctx, claudeCfg); err != nil {
		return fmt.Errorf("start agent: %w", err)
	}

	return nil
}
```

- [ ] **Step 5: (test already written in Step 3) Verify test passes (TDD: GREEN)**

The test written in Step 3:

```go
func TestStartDetachedSession(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	params := orchestrator.SessionParams{
		ID: "test123", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartDetachedSession failed: %v", err)
	}
	// Verify both containers started as detached
	if len(mock.DetachedConfigs) < 2 {
		t.Errorf("expected 2 detached containers, got %d", len(mock.DetachedConfigs))
	}
	// Verify no attached container (unlike StartSession)
	if mock.AttachedConfig != nil {
		t.Error("detached session should not attach any container")
	}
	// Verify ID-based naming
	for _, cfg := range mock.DetachedConfigs {
		if !strings.Contains(cfg.Name, "test123") {
			t.Errorf("container name %s should contain workspace ID", cfg.Name)
		}
	}
}
```

- [ ] **Step 6: Create `internal/cli/start.go`**

```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/orchestrator"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var (
	startID      string
	startEnvFile string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start containers in detached mode",
	Long:  `Starts the proxy and agent containers in the background. Use docker exec to open shell sessions into the agent container.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if startID == "" {
			return fmt.Errorf("--id is required")
		}
		ctx := context.Background()
		airlockDir := ".airlock"
		keysDir := filepath.Join(airlockDir, "keys")

		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config (run 'airlock init' first): %w", err)
		}

		workspace, _ := os.Getwd()
		workspace, _ = filepath.Abs(workspace)

		homeDir, _ := os.UserHomeDir()
		claudeDir := filepath.Join(homeDir, ".claude")

		tmpDir, _ := os.MkdirTemp("", "airlock-*")
		// Note: tmpDir is NOT cleaned up on exit for detached mode.
		// It persists until airlock stop is called.

		params := orchestrator.SessionParams{
			ID:        startID,
			Workspace: workspace,
			ClaudeDir: claudeDir,
			Config:    cfg,
			TmpDir:    tmpDir,
		}

		if startEnvFile != "" {
			kp, err := crypto.LoadKeyPair(keysDir)
			if err != nil {
				return fmt.Errorf("load keypair: %w", err)
			}
			entries, err := secrets.ParseEnvFile(startEnvFile)
			if err != nil {
				return fmt.Errorf("parse env file: %w", err)
			}
			result, err := secrets.EncryptEntries(entries, kp.PublicKey)
			if err != nil {
				return fmt.Errorf("encrypt entries: %w", err)
			}
			params.EnvFilePath = filepath.Join(tmpDir, "env.enc")
			if err := secrets.WriteEnvFile(params.EnvFilePath, result.Encrypted); err != nil {
				return fmt.Errorf("write encrypted env: %w", err)
			}
			var mappingErr error
			params.MappingPath, mappingErr = secrets.SaveMapping(result.Mapping, tmpDir)
			if mappingErr != nil {
				return fmt.Errorf("save mapping: %w", mappingErr)
			}
		}

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		if err := orchestrator.StartDetachedSession(ctx, docker, params); err != nil {
			return err
		}

		out, _ := json.Marshal(map[string]string{
			"status":    "running",
			"container": "airlock-claude-" + startID,
			"proxy":     "airlock-proxy-" + startID,
			"network":   cfg.NetworkName + "-" + startID,
		})
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	startCmd.Flags().StringVar(&startID, "id", "", "workspace identifier (required)")
	startCmd.Flags().StringVarP(&startEnvFile, "env", "e", "", "env file to encrypt and mount")
	rootCmd.AddCommand(startCmd)
}
```

- [ ] **Step 7: Run tests**

```bash
make test
```

- [ ] **Step 8: Commit**

```bash
git add internal/cli/start.go internal/orchestrator/session.go \
       internal/orchestrator/session_test.go internal/container/manager.go \
       container/entrypoint-keepalive.sh container/Dockerfile
git commit -m "feat: add airlock start command for detached container sessions"
```

---

### Task 3: Create `airlock status` command

**Files:**
- Create: `internal/cli/status.go`

- [ ] **Step 1: Write failing test for ID extraction (TDD: RED)**

Create `internal/cli/status_test.go` with `TestExtractIDFromContainerName` (same test as Step 4 below). Also write a stub `status.go` that registers the command but has no implementation, so the test file compiles. Run `make test` to confirm the test passes for the extraction logic but serves as a scaffold.

- [ ] **Step 2: Create `internal/cli/status.go`**

```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/container"
)

var statusID string

type workspaceStatus struct {
	ID        string `json:"id"`
	Container string `json:"container"`
	Proxy     string `json:"proxy"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime,omitempty"`
	Error     string `json:"error,omitempty"`
}

type statusOutput struct {
	Workspaces []workspaceStatus `json:"workspaces"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of running airlock containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		containers, err := docker.ListContainers(ctx, "airlock-claude")
		if err != nil {
			return fmt.Errorf("list containers: %w", err)
		}

		output := statusOutput{Workspaces: []workspaceStatus{}}
		for _, c := range containers {
			ws := workspaceStatus{
				Container: c.Name,
				Status:    c.Status,
				Uptime:    c.Uptime,
			}
			// Extract ID from name: airlock-claude-{id} → {id}
			if len(c.Name) > len("airlock-claude-") {
				ws.ID = c.Name[len("airlock-claude-"):]
			}
			ws.Proxy = "airlock-proxy-" + ws.ID
			if c.Status == "exited" {
				ws.Error = c.Error
			}
			if statusID == "" || ws.ID == statusID {
				output.Workspaces = append(output.Workspaces, ws)
			}
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusID, "id", "", "filter by workspace ID")
	rootCmd.AddCommand(statusCmd)
}
```

- [ ] **Step 3: Add `ListContainers` to ContainerRuntime interface and Docker implementation**

In `internal/container/runtime.go`, add:

```go
type ContainerInfo struct {
	Name   string
	Status string
	Uptime string
	Error  string
}
```

Add to the interface:

```go
ListContainers(ctx context.Context, prefix string) ([]ContainerInfo, error)
```

In `internal/container/docker.go`, implement:

```go
func (d *Docker) ListContainers(ctx context.Context, prefix string) ([]ContainerInfo, error) {
	containers, err := d.client.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	var result []ContainerInfo
	for _, c := range containers {
		for _, name := range c.Names {
			clean := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(clean, prefix) {
				info := ContainerInfo{
					Name:   clean,
					Status: c.State,
				}
				if c.State == "running" {
					info.Uptime = c.Status
				}
				if c.State == "exited" {
					info.Error = c.Status
				}
				result = append(result, info)
			}
		}
	}
	return result, nil
}
```

- [ ] **Step 4: Update MockRuntime in tests**

In `internal/orchestrator/session_test.go`, add to MockRuntime:

```go
func (m *MockRuntime) ListContainers(_ context.Context, prefix string) ([]container.ContainerInfo, error) {
	return nil, nil
}
```

Same for FailingMockRuntime.

- [ ] **Step 5: Verify tests pass with full implementation (TDD: GREEN)**

In a new test file `internal/cli/status_test.go`:

```go
package cli

import "testing"

func TestExtractIDFromContainerName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with ID", "airlock-claude-abc123", "abc123"},
		{"legacy name", "airlock-claude", ""},
		{"short ID", "airlock-claude-a1", "a1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := "airlock-claude-"
			id := ""
			if len(tt.input) > len(prefix) {
				id = tt.input[len(prefix):]
			}
			if id != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, id)
			}
		})
	}
}
```

- [ ] **Step 6: Run all tests**

```bash
make test
```

- [ ] **Step 7: Commit**

```bash
git add internal/cli/status.go internal/cli/status_test.go \
       internal/container/runtime.go internal/container/docker.go \
       internal/orchestrator/session_test.go
git commit -m "feat: add airlock status command for container state queries"
```

---

### Task 4: Update `airlock stop` with `--id` support

**Files:**
- Modify: `internal/cli/stop.go`

- [ ] **Step 1: Add `--id` flag to stop command**

Replace `internal/cli/stop.go`:

```go
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/orchestrator"
)

var stopID string

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop running airlock containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg, err := config.Load(".airlock")
		if err != nil {
			cfg = config.Default()
		}
		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		if stopID != "" {
			orchestrator.CleanupSession(ctx, docker, cfg, stopID)
		} else {
			// Stop all running airlock containers
			containers, err := docker.ListContainers(ctx, "airlock-claude")
			if err == nil && len(containers) > 0 {
				for _, c := range containers {
					id := ""
					if len(c.Name) > len("airlock-claude-") {
						id = c.Name[len("airlock-claude-"):]
					}
					orchestrator.CleanupSession(ctx, docker, cfg, id)
				}
			} else {
				// Fallback: try legacy fixed names
				orchestrator.CleanupSession(ctx, docker, cfg, "")
			}
		}
		fmt.Println("Done.")
		return nil
	},
}

func init() {
	stopCmd.Flags().StringVar(&stopID, "id", "", "workspace ID to stop (default: stop all)")
	rootCmd.AddCommand(stopCmd)
}
```

- [ ] **Step 2: Run tests**

```bash
make test
```

- [ ] **Step 3: Commit**

```bash
git add internal/cli/stop.go
git commit -m "feat: add --id flag to airlock stop, default stops all containers"
```

---

## Phase 2: Proxy Activity Logging (Python)

### Task 5: Add structured JSON logging to decrypt_proxy.py

**Files:**
- Modify: `proxy/addon/decrypt_proxy.py`
- Test: `proxy/addon/test_decrypt_proxy.py`

- [ ] **Step 1: Write test for structured logging**

In `proxy/addon/test_decrypt_proxy.py`, add:

```python
def test_request_emits_log_on_decrypt(capsys):
    mapping = {"ENC[age:tok1]": "secret_val"}
    addon = _make_addon(mapping)
    flow = _make_flow(
        host="api.stripe.com",
        headers={"Authorization": "Bearer ENC[age:tok1]"},
    )
    addon.request(flow)
    captured = capsys.readouterr()
    import json
    log = json.loads(captured.out.strip())
    assert log["host"] == "api.stripe.com"
    assert log["action"] == "decrypt"
    assert log["location"] == "header"
    assert log["key"] == "Authorization"
    assert "secret" not in captured.out  # plaintext NEVER in logs


def test_request_emits_log_on_passthrough(capsys):
    addon = _make_addon({}, passthrough=["api.anthropic.com"])
    flow = _make_flow(host="api.anthropic.com", headers={"Auth": "token"})
    addon.request(flow)
    captured = capsys.readouterr()
    import json
    log = json.loads(captured.out.strip())
    assert log["action"] == "passthrough"
    assert log["host"] == "api.anthropic.com"


def test_request_emits_log_on_no_match(capsys):
    addon = _make_addon({"ENC[age:other]": "val"})
    flow = _make_flow(host="cdn.example.com", headers={"Accept": "text/html"})
    addon.request(flow)
    captured = capsys.readouterr()
    import json
    log = json.loads(captured.out.strip())
    assert log["action"] == "none"
```

- [ ] **Step 2: Run tests -- verify they fail**

```bash
make test-python
```

Expected: FAIL (no logging implemented yet)

- [ ] **Step 3: Implement structured logging**

In `proxy/addon/decrypt_proxy.py`, add `_emit_log` method and update `request`:

```python
import datetime

class DecryptAddon:
    # ... existing __init__, _load_mapping, is_passthrough, replace_secrets ...

    def _emit_log(self, host: str, action: str, location: str | None = None, key: str | None = None) -> None:
        entry = {
            "time": datetime.datetime.now().strftime("%H:%M:%S"),
            "host": host,
            "action": action,
        }
        if location:
            entry["location"] = location
        if key:
            entry["key"] = key
        print(json.dumps(entry), flush=True)

    def request(self, flow: http.HTTPFlow) -> None:
        host = flow.request.pretty_host
        if self.is_passthrough(host):
            self._emit_log(host, "passthrough")
            return

        decrypted = False
        for name, value in flow.request.headers.items():
            replaced = self.replace_secrets(value)
            if replaced != value:
                flow.request.headers[name] = replaced
                self._emit_log(host, "decrypt", "header", name)
                decrypted = True

        if flow.request.query:
            for key, value in flow.request.query.items():
                replaced = self.replace_secrets(value)
                if replaced != value:
                    flow.request.query[key] = replaced
                    self._emit_log(host, "decrypt", "query", key)
                    decrypted = True

        if flow.request.content:
            try:
                body = flow.request.content.decode("utf-8")
                replaced = self.replace_secrets(body)
                if replaced != body:
                    flow.request.content = replaced.encode("utf-8")
                    self._emit_log(host, "decrypt", "body")
                    decrypted = True
            except UnicodeDecodeError:
                pass

        if not decrypted:
            self._emit_log(host, "none")
```

- [ ] **Step 4: Run tests -- verify they pass**

```bash
make test-python
```

Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add proxy/addon/decrypt_proxy.py proxy/addon/test_decrypt_proxy.py
git commit -m "feat: add structured JSON activity logging to proxy addon"
```

---

## Phase 3: GUI Data Model Changes (Swift)

### Task 6: Update Workspace and AppState models

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Models/Workspace.swift`
- Modify: `AirlockApp/Sources/AirlockApp/Models/AppState.swift`
- Test: `AirlockApp/Tests/AirlockAppTests/WorkspaceTests.swift`
- Test: `AirlockApp/Tests/AirlockAppTests/AppStateTests.swift`

**AppState Migration Matrix** -- all files referencing old properties must be updated:

| Old Property | New Property | Files to Update |
|---|---|---|
| `activeWorkspaceID: UUID?` | `activeWorkspaceIDs: Set<UUID>` | SidebarView.swift, TerminalView.swift, ContentView.swift, AppStateTests.swift |
| `sessionStatus: SessionStatus` | `statusFor(_:) -> SessionStatus` (per-workspace method) | SidebarView.swift, TerminalView.swift, ContentView.swift, AppStateTests.swift |
| `isRunning: Bool` | `isActive(_:) -> Bool` (per-workspace method) | SidebarView.swift |
| `activeWorkspace: Workspace?` | Remove (use `selectedWorkspace` + `isActive`) | ContentView.swift |

- [ ] **Step 1: Update Workspace model**

In `Workspace.swift`, add new fields. Mark runtime-only fields as non-Codable. Also create `TerminalSession`:

```swift
struct Workspace: Identifiable, Codable, Hashable {
    let id: UUID
    var name: String
    var path: String
    var envFilePath: String?
    var containerImageOverride: String?

    // Runtime state (not persisted)
    var isActive: Bool = false
    var containerId: String?
    var proxyId: String?
    var networkId: String?

    // Exclude runtime fields from Codable
    enum CodingKeys: String, CodingKey {
        case id, name, path, envFilePath, containerImageOverride
    }

    var shortID: String {
        String(id.uuidString.prefix(8)).lowercased()
    }

    var containerName: String {
        "airlock-claude-\(shortID)"
    }

    var proxyName: String {
        "airlock-proxy-\(shortID)"
    }

    // Runtime terminal tracking (not persisted)
    var terminalSessions: [TerminalSession] = []

    init(name: String, path: String, envFilePath: String? = nil, containerImageOverride: String? = nil) {
        self.id = UUID()
        self.name = name
        self.path = path
        self.envFilePath = envFilePath
        self.containerImageOverride = containerImageOverride
    }
}

/// Tracks an active docker exec terminal session (in-memory only, not persisted).
struct TerminalSession: Identifiable {
    let id: UUID = UUID()
    var isActive: Bool = true
    // Process handle managed by TerminalSplitView, not stored here
}
```

- [ ] **Step 2: Update AppState model**

In `AppState.swift`:

```swift
enum DetailTab: Hashable {
    case terminal
    case secrets
    case containers
    case diff
    case settings
}

@Observable
final class AppState {
    var workspaces: [Workspace] = []
    var selectedWorkspaceID: UUID?
    var activeWorkspaceIDs: Set<UUID> = []
    var selectedTab: DetailTab = .terminal
    var lastError: String?

    var selectedWorkspace: Workspace? {
        workspaces.first { $0.id == selectedWorkspaceID }
    }

    func isActive(_ workspace: Workspace) -> Bool {
        activeWorkspaceIDs.contains(workspace.id)
    }

    func statusFor(_ workspace: Workspace) -> SessionStatus {
        guard let ws = workspaces.first(where: { $0.id == workspace.id }) else { return .stopped }
        if activeWorkspaceIDs.contains(ws.id) { return ws.isActive ? .running : .error("activation failed") }
        return .stopped
    }
}
```

- [ ] **Step 3: Update tests**

In `AppStateTests.swift`, update tests for new `activeWorkspaceIDs` and `DetailTab` cases.

In `WorkspaceTests.swift`, add test for `shortID` and `containerName` properties, and verify that runtime fields are excluded from Codable:

```swift
func testShortIDFormat() {
    let ws = Workspace(name: "test", path: "/tmp")
    XCTAssertEqual(ws.shortID.count, 8)
    XCTAssertEqual(ws.containerName, "airlock-claude-\(ws.shortID)")
}

func testRuntimeFieldsNotEncoded() throws {
    var ws = Workspace(name: "test", path: "/tmp")
    ws.isActive = true
    ws.containerId = "abc"
    let data = try JSONEncoder().encode(ws)
    let decoded = try JSONDecoder().decode(Workspace.self, from: data)
    XCTAssertFalse(decoded.isActive)
    XCTAssertNil(decoded.containerId)
}
```

- [ ] **Step 4: Run tests**

```bash
make gui-test
```

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Models/ AirlockApp/Tests/
git commit -m "refactor: update data models for multi-workspace and container state"
```

---

## Phase 4: GUI Workspace Lifecycle (Swift)

### Task 7: Create ContainerSessionService

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Services/ContainerSessionService.swift`

- [ ] **Step 1: Implement ContainerSessionService**

```swift
import Foundation

final class ContainerSessionService {
    private let cli: CLIService

    init(cli: CLIService) {
        self.cli = cli
    }

    /// Start containers for a workspace (airlock start --id {shortID})
    func activate(workspace: Workspace) async throws -> CLIResult {
        var args = ["start", "--id", workspace.shortID]
        if let envFile = workspace.envFilePath {
            args += ["--env", envFile]
        }
        let result = cli.run(args: args, workingDirectory: workspace.path)
        if result.exitCode != 0 {
            throw NSError(
                domain: "ContainerSession",
                code: Int(result.exitCode),
                userInfo: [NSLocalizedDescriptionKey: result.stderr ?? "activation failed"]
            )
        }
        return result
    }

    /// Stop containers for a workspace (airlock stop --id {shortID})
    func deactivate(workspace: Workspace) {
        _ = cli.run(args: ["stop", "--id", workspace.shortID], workingDirectory: workspace.path)
    }

    /// Query status of all running workspaces
    func status() -> CLIResult {
        // Use home directory as working directory since status is global
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        return cli.run(args: ["status"], workingDirectory: home)
    }

    /// Check if Docker daemon is running
    func isDockerRunning() -> Bool {
        // Use docker info directly -- airlock version does NOT check Docker
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/local/bin/docker")
        process.arguments = ["info"]
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        do {
            try process.run()
            process.waitUntilExit()
            return process.terminationStatus == 0
        } catch {
            return false
        }
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Services/ContainerSessionService.swift
git commit -m "feat: add ContainerSessionService for workspace activation/deactivation"
```

---

### Task 8: Update SidebarView for multi-workspace activation

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Sidebar/SidebarView.swift`

- [ ] **Step 1: Update context menu to Activate/Deactivate/Remove**

Replace the existing context menu in SidebarView with:
- "Activate" (when stopped) -- calls ContainerSessionService.activate
- "Deactivate" (when running) -- calls ContainerSessionService.deactivate
- "Stop and Remove" (when running) -- deactivate then remove
- "Remove" (when stopped) -- remove directly

- [ ] **Step 2: Support multiple green dots**

Change status indicator logic from checking `appState.activeWorkspaceID == ws.id` to `appState.activeWorkspaceIDs.contains(ws.id)`.

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Sidebar/SidebarView.swift
git commit -m "feat: multi-workspace activation in sidebar with activate/deactivate controls"
```

---

### Task 9: Add WelcomeView for first launch

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Views/Welcome/WelcomeView.swift`
- Modify: `AirlockApp/Sources/AirlockApp/ContentView.swift`

- [ ] **Step 1: Create WelcomeView**

Shows when `appState.workspaces.isEmpty`. Displays Docker status check and "Create Your First Workspace" button.

- [ ] **Step 2: Update ContentView to show WelcomeView when no workspaces**

In ContentView, wrap the main area:

```swift
if appState.workspaces.isEmpty {
    WelcomeView()
} else {
    // existing tabbed content
}
```

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Welcome/WelcomeView.swift \
       AirlockApp/Sources/AirlockApp/ContentView.swift
git commit -m "feat: add welcome screen for first-launch experience"
```

---

## Phase 5: GUI Terminal Redesign (Swift)

### Task 10: Create TerminalSplitView with multi-pane support

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Views/Terminal/TerminalSplitView.swift`
- Modify: `AirlockApp/Sources/AirlockApp/Views/Terminal/TerminalView.swift`
- Modify: `AirlockApp/Sources/AirlockApp/ContentView.swift`

- [ ] **Step 1: Refactor TerminalView to accept container name as parameter**

Change TerminalView to spawn `docker exec -it {containerName} /bin/bash` instead of `airlock run`. Accept containerName as init parameter.

- [ ] **Step 2: Create TerminalSplitView**

Manages a list of TerminalView instances. Supports:
- Add terminal (max 4)
- Vertical split (HSplitView)
- Horizontal split (VSplitView)
- Toolbar with [+ Terminal], [Split V], [Split H] buttons
- Close individual panes

- [ ] **Step 3: Update ContentView to use TerminalSplitView**

Replace direct TerminalView usage with TerminalSplitView, passing the active workspace's container name.

- [ ] **Step 4: Run GUI tests**

```bash
make gui-test
```

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Terminal/
git commit -m "feat: multi-pane terminal with docker exec into container"
```

---

## Phase 6: GUI New Tabs (Swift)

### Task 11: Create SecretsView tab

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift`
- Modify: `AirlockApp/Sources/AirlockApp/ContentView.swift`

- [ ] **Step 1: Implement SecretsView**

Table view showing .env file entries with:
- Name, Status (encrypted/plaintext/not-secret), masked Value, Actions
- [+ Add Entry], [Encrypt All], [Export] buttons
- Key info section at bottom
- "Restart workspace to apply changes" banner when workspace is active

Status detection:
- `ENC[age:...]` → encrypted
- Name contains KEY/SECRET/PASSWORD/TOKEN → plaintext (warning)
- Otherwise → not secret

- [ ] **Step 2: Add Secrets tab to ContentView**

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Secrets/ AirlockApp/Sources/AirlockApp/ContentView.swift
git commit -m "feat: add secrets management tab with encryption status and editing"
```

---

### Task 12: Create ContainerStatusView tab with proxy log

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Views/Containers/ContainerStatusView.swift`
- Modify: `AirlockApp/Sources/AirlockApp/ContentView.swift`

- [ ] **Step 1: Implement ContainerStatusView**

Shows when workspace is active:
- Agent container card (name, image, status, uptime, terminal count)
- Proxy container card (name, image, status, port, passthrough hosts)
- Network card (name, driver, status)
- Security summary section
- Proxy Activity Log (scrollable table)

Activity log implementation:
- Spawns `docker logs --follow airlock-proxy-{id}` as subprocess
- Parses JSON lines, skips non-JSON
- Displays in table: Time | Host | Result | Location
- Summary counters at bottom
- [Auto-scroll] toggle, [Clear] button

Shows when workspace is inactive:
- "No containers running. Activate workspace to start."

- [ ] **Step 2: Add Containers tab to ContentView**

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Containers/ AirlockApp/Sources/AirlockApp/ContentView.swift
git commit -m "feat: add container status tab with proxy activity log"
```

---

## Phase 7: GUI Polish

### Task 13: Fix menu bar notifications and add new shortcuts

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/AirlockApp.swift`

- [ ] **Step 1: Connect menu bar commands to actual functionality**

Replace notification-based menu commands with direct state mutations or method calls. Add keyboard shortcuts for new tabs:
- Cmd+1: Terminal
- Cmd+2: Secrets
- Cmd+3: Containers
- Cmd+4: Diff
- Cmd+5: Settings
- Cmd+D: Split vertical
- Cmd+Shift+D: Split horizontal
- Cmd+T: New terminal

- [ ] **Step 2: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/AirlockApp.swift
git commit -m "fix: connect menu bar commands and add keyboard shortcuts for all tabs"
```

---

### Task 14: Update workspace creation sheet with pre-checks

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift`

- [ ] **Step 1: Add pre-check indicators**

After directory selection, run pre-checks:
- Directory exists
- .airlock/ initialized
- Docker daemon running
- Container images exist
- .env plaintext secrets warning

Display as checklist with check/cross marks. Disable "Create" button only if directory doesn't exist.

- [ ] **Step 2: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift
git commit -m "feat: add pre-checks to workspace creation sheet"
```

---

### Task 15: Add crash recovery on app launch

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/AirlockApp.swift`

- [ ] **Step 1: On app launch, reconcile with running containers**

In the app's `onAppear` or init:
1. Call `airlock status` via CLIService
2. Parse JSON response
3. For each running workspace found:
   - If matching workspace exists in saved list → mark as active
   - If orphaned → show cleanup dialog
4. Update `appState.activeWorkspaceIDs` accordingly

- [ ] **Step 2: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/AirlockApp.swift
git commit -m "feat: crash recovery - reconcile running containers on app launch"
```

---

## Dependency Graph

```
Task 1 (ID naming) ──→ Task 2 (airlock start) ──→ Task 3 (status) ──→ Task 4 (stop --id)
                                                                              │
Task 5 (proxy logging) ─────────────────────────────────────────────────────→ │
                                                                              ▼
Task 6 (data models) ──→ Task 7 (ContainerSessionService) ──→ Task 8 (sidebar)
                    │                                                         │
                    └──→ Task 9 (welcome) ──→ Task 14 (creation sheet)       │
                    │                                                         ▼
                    └──→ Task 10 (terminal split) ──→ Task 11 (secrets tab)
                                                  └─→ Task 12 (container tab + proxy log)
                                                  └─→ Task 13 (menu bar fix)
                                                  └─→ Task 15 (crash recovery)
```

Phase 1-2 (Go + Python) have no Swift dependencies and can start immediately.
Phase 3-7 (Swift) depend on Phase 1 CLI commands being available.
Within Swift phases, tasks are ordered by dependency but many can run in parallel.
