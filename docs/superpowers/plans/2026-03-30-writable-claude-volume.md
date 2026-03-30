# Writable .claude Volume Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the read-only `~/.claude` bind mount with a persistent Docker named volume so OAuth tokens, history, and session state survive container restarts.

**Architecture:** A Docker named volume (`airlock-claude-home`) replaces the current `:ro` bind mount. Settings files are extracted from the volume before each start, scanned for secrets, and shadow-mounted back as encrypted overlays. A CLI `config import` command seeds the volume from the host's `~/.claude`.

**Tech Stack:** Go 1.22+, Docker Engine SDK v27, Docker `mount.Mount` API, cobra CLI, SwiftUI (GUI)

**Spec:** `docs/superpowers/specs/2026-03-30-writable-claude-volume-design.md`

---

### Task 1: Pin UID in Dockerfile

**Files:**
- Modify: `container/Dockerfile:15`

- [ ] **Step 1: Update Dockerfile to pin UID**

```dockerfile
# Before (line 15):
RUN useradd -m -s /bin/bash airlock

# After:
RUN useradd -m -s /bin/bash -u 1001 airlock
```

- [ ] **Step 2: Rebuild image to verify**

Run: `docker build -t airlock-claude:latest container/`
Expected: Build succeeds. `docker run --rm airlock-claude:latest id airlock` outputs `uid=1001(airlock) gid=1001(airlock) groups=1001(airlock)`

- [ ] **Step 3: Commit**

```bash
git add container/Dockerfile
git commit -m "fix: pin airlock user UID to 1001 for volume ownership stability"
```

---

### Task 2: Add VolumeName to Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestConfigVolumeNameDefault(t *testing.T) {
	cfg := config.Default()
	if cfg.VolumeName != "airlock-claude-home" {
		t.Errorf("expected default VolumeName airlock-claude-home, got %s", cfg.VolumeName)
	}
}

func TestConfigVolumeNameRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.VolumeName = "custom-volume"
	if err := config.Save(cfg, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.VolumeName != "custom-volume" {
		t.Errorf("expected custom-volume, got %s", loaded.VolumeName)
	}
}

func TestConfigVolumeNameBackwardsCompat(t *testing.T) {
	dir := t.TempDir()
	// Write a config without volume_name (simulates pre-upgrade)
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Zero value string is acceptable; caller applies default
	if loaded.VolumeName != "" {
		t.Errorf("expected empty VolumeName for old config, got %s", loaded.VolumeName)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run TestConfigVolumeName -v`
Expected: `TestConfigVolumeNameDefault` FAILS (field does not exist)

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add `VolumeName` to the struct and default:

```go
type Config struct {
	ContainerImage   string   `yaml:"container_image"`
	ProxyImage       string   `yaml:"proxy_image"`
	NetworkName      string   `yaml:"network_name"`
	ProxyPort        int      `yaml:"proxy_port"`
	PassthroughHosts []string `yaml:"passthrough_hosts"`
	VolumeName       string   `yaml:"volume_name"`
}

func Default() Config {
	return Config{
		ContainerImage:   "airlock-claude:latest",
		ProxyImage:       "airlock-proxy:latest",
		NetworkName:      "airlock-net",
		ProxyPort:        8080,
		PassthroughHosts: []string{},
		VolumeName:       "airlock-claude-home",
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add VolumeName field to Config with default airlock-claude-home"
```

---

### Task 3: Add volume operations to ContainerRuntime interface

**Files:**
- Modify: `internal/container/runtime.go`

- [ ] **Step 1: Add three methods to the interface**

```go
package container

import "context"

// ContainerInfo holds status information about a single container.
type ContainerInfo struct {
	Name   string
	Status string
	Uptime string
	Error  string
}

// ContainerRuntime abstracts container operations for testability.
type ContainerRuntime interface {
	EnsureNetwork(ctx context.Context, opts NetworkOpts) (string, error)
	RunDetached(ctx context.Context, cfg ContainerConfig) (string, error)
	RunAttached(ctx context.Context, cfg ContainerConfig) error
	Stop(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	RemoveNetwork(ctx context.Context, name string) error
	ConnectNetwork(ctx context.Context, networkID, containerID string) error
	CopyFromContainer(ctx context.Context, containerName, srcPath, dstPath string) error
	WaitForFile(ctx context.Context, containerName, path string, maxRetries int) error
	ListContainers(ctx context.Context, prefix string) ([]ContainerInfo, error)
	EnsureVolume(ctx context.Context, name string) error
	RemoveVolume(ctx context.Context, name string) error
	ReadFromVolume(ctx context.Context, volumeName, filePath, dstPath string) error
}
```

Note: `ReadFromVolume` writes to `dstPath` on the host (consistent with `CopyFromContainer` pattern) rather than returning `[]byte`. Returns `os.ErrNotExist` if the file is missing.

- [ ] **Step 2: Verify the build fails**

Run: `go build ./...`
Expected: FAIL -- `Docker` does not implement `ContainerRuntime` (missing 3 methods)

- [ ] **Step 3: Commit (compile-broken is OK, will fix in Task 4)**

```bash
git add internal/container/runtime.go
git commit -m "feat: add EnsureVolume, RemoveVolume, ReadFromVolume to ContainerRuntime"
```

---

### Task 4: Implement volume operations in Docker

**Files:**
- Modify: `internal/container/docker.go`

- [ ] **Step 1: Add imports**

Add to imports in `docker.go`:

```go
"github.com/docker/docker/api/types/volume"
```

- [ ] **Step 2: Implement EnsureVolume**

```go
// EnsureVolume creates a Docker volume if it does not already exist.
func (d *Docker) EnsureVolume(ctx context.Context, name string) error {
	_, err := d.client.VolumeInspect(ctx, name)
	if err == nil {
		return nil
	}
	_, err = d.client.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}
	return nil
}
```

- [ ] **Step 3: Implement RemoveVolume**

```go
// RemoveVolume removes a Docker volume by name.
func (d *Docker) RemoveVolume(ctx context.Context, name string) error {
	return d.client.VolumeRemove(ctx, name, true)
}
```

- [ ] **Step 4: Implement ReadFromVolume**

```go
// ReadFromVolume extracts a single file from a Docker volume to the local
// filesystem. It runs a temporary container with the volume mounted read-only,
// then uses CopyFromContainer to extract the file via tar stream. Returns
// os.ErrNotExist if the file does not exist in the volume.
func (d *Docker) ReadFromVolume(ctx context.Context, volumeName, filePath, dstPath string) error {
	tmpName := "airlock-vol-reader"

	containerConfig := &dockercontainer.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sleep", "30"},
		User:  "1001:1001",
	}
	hostConfig := &dockercontainer.HostConfig{
		Binds:      []string{fmt.Sprintf("%s:/vol:ro", volumeName)},
		AutoRemove: true,
	}

	d.client.ContainerRemove(ctx, tmpName, dockercontainer.RemoveOptions{Force: true})

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, tmpName)
	if err != nil {
		return fmt.Errorf("create reader container: %w", err)
	}
	defer func() {
		d.client.ContainerStop(ctx, resp.ID, dockercontainer.StopOptions{})
	}()

	if err := d.client.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		return fmt.Errorf("start reader container: %w", err)
	}

	srcPath := filepath.Join("/vol", filePath)
	err = d.CopyFromContainer(ctx, tmpName, srcPath, dstPath)
	if err != nil {
		if strings.Contains(err.Error(), "No such") || strings.Contains(err.Error(), "not found") {
			return os.ErrNotExist
		}
		return err
	}
	return nil
}
```

- [ ] **Step 5: Verify build passes**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/container/docker.go
git commit -m "feat: implement EnsureVolume, RemoveVolume, ReadFromVolume for Docker"
```

---

### Task 5: Add Mounts field to ContainerConfig and update Docker runtime

**Files:**
- Modify: `internal/container/manager.go`
- Modify: `internal/container/docker.go`

- [ ] **Step 1: Add Mounts field to ContainerConfig**

In `internal/container/manager.go`, add import and field:

```go
import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/taeikkim92/airlock/internal/secrets"
)

type ContainerConfig struct {
	Image      string
	Name       string
	Binds      []string
	Mounts     []mount.Mount
	Env        []string
	Network    string
	CapDrop    []string
	WorkingDir string
	Tty        bool
	Stdin      bool
	Cmd        []string
}
```

- [ ] **Step 2: Update Docker.RunDetached to pass Mounts**

In `docker.go`, update `RunDetached`:

```go
func (d *Docker) RunDetached(ctx context.Context, cfg ContainerConfig) (string, error) {
	hostConfig := &dockercontainer.HostConfig{
		Binds:   cfg.Binds,
		Mounts:  cfg.Mounts,
		CapDrop: cfg.CapDrop,
	}
	// ... rest unchanged
```

- [ ] **Step 3: Update Docker.RunAttached to pass Mounts**

In `docker.go`, update `RunAttached`:

```go
func (d *Docker) RunAttached(ctx context.Context, cfg ContainerConfig) error {
	hostConfig := &dockercontainer.HostConfig{
		Binds:   cfg.Binds,
		Mounts:  cfg.Mounts,
		CapDrop: cfg.CapDrop,
	}
	// ... rest unchanged
```

- [ ] **Step 4: Verify build passes**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Run existing tests**

Run: `go test ./internal/container/ -v`
Expected: ALL PASS (Mounts is nil by default, no behavior change)

- [ ] **Step 6: Commit**

```bash
git add internal/container/manager.go internal/container/docker.go
git commit -m "feat: add Mounts field to ContainerConfig for volume mount support"
```

---

### Task 6: Replace ClaudeDir with VolumeName in RunOpts and BuildClaudeConfig

**Files:**
- Modify: `internal/container/manager.go`
- Modify: `internal/container/manager_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/container/manager_test.go`:

```go
func TestBuildClaudeConfigWithVolume(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "airlock-claude-home",
		ProxyPort:   8080,
	}
	cfg := container.BuildClaudeConfig(opts)

	// Should have workspace bind mount but NO .claude bind mount
	for _, bind := range cfg.Binds {
		if strings.Contains(bind, ".claude") {
			t.Errorf("should not have .claude bind mount when VolumeName is set, got: %s", bind)
		}
	}
	if len(cfg.Binds) != 1 {
		t.Errorf("expected 1 bind mount (workspace only), got %d: %v", len(cfg.Binds), cfg.Binds)
	}

	// Should have a volume mount
	if len(cfg.Mounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(cfg.Mounts))
	}
	m := cfg.Mounts[0]
	if m.Type != mount.TypeVolume {
		t.Errorf("expected volume mount type, got %s", m.Type)
	}
	if m.Source != "airlock-claude-home" {
		t.Errorf("expected source airlock-claude-home, got %s", m.Source)
	}
	if m.Target != "/home/airlock/.claude" {
		t.Errorf("expected target /home/airlock/.claude, got %s", m.Target)
	}
}

func TestBuildClaudeConfigWithClaudeDirFallback(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		ClaudeDir:   "/home/user/.claude",
		ProxyPort:   8080,
	}
	cfg := container.BuildClaudeConfig(opts)

	// Fallback: should use bind mount, no volume mount
	if len(cfg.Mounts) != 0 {
		t.Errorf("expected 0 volume mounts for ClaudeDir fallback, got %d", len(cfg.Mounts))
	}
	foundClaude := false
	for _, bind := range cfg.Binds {
		if strings.Contains(bind, ".claude:ro") {
			foundClaude = true
		}
	}
	if !foundClaude {
		t.Errorf(".claude bind mount not found in fallback mode: %v", cfg.Binds)
	}
}
```

Add `mount` import: `"github.com/docker/docker/api/types/mount"`

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/container/ -run TestBuildClaudeConfigWith -v`
Expected: FAIL (VolumeName field does not exist)

- [ ] **Step 3: Implement RunOpts and BuildClaudeConfig changes**

In `internal/container/manager.go`:

```go
type RunOpts struct {
	ID               string
	Workspace        string
	Image            string
	ProxyImage       string
	NetworkName      string
	ShadowMounts     []secrets.ShadowMount
	MappingPath      string
	VolumeName       string // Docker named volume for .claude state
	ClaudeDir        string // Deprecated: bind mount fallback
	CACertPath       string
	ProxyPort        int
	PassthroughHosts []string
}

func BuildClaudeConfig(opts RunOpts) ContainerConfig {
	claudeName := "airlock-claude"
	proxyName := "airlock-proxy"
	if opts.ID != "" {
		claudeName = "airlock-claude-" + opts.ID
		proxyName = "airlock-proxy-" + opts.ID
	}
	proxyURL := fmt.Sprintf("http://%s:%d", proxyName, opts.ProxyPort)

	binds := []string{
		fmt.Sprintf("%s:/workspace", opts.Workspace),
	}

	var mounts []mount.Mount
	if opts.VolumeName != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: opts.VolumeName,
			Target: "/home/airlock/.claude",
		})
	} else if opts.ClaudeDir != "" {
		binds = append(binds, fmt.Sprintf("%s:/home/airlock/.claude:ro", opts.ClaudeDir))
	}

	if opts.CACertPath != "" {
		binds = append(binds, fmt.Sprintf("%s:/usr/local/share/ca-certificates/airlock-proxy.crt:ro", opts.CACertPath))
	}
	for _, m := range opts.ShadowMounts {
		binds = append(binds, fmt.Sprintf("%s:%s:ro", m.HostPath, m.ContainerPath))
	}

	return ContainerConfig{
		Image:      opts.Image,
		Name:       claudeName,
		Network:    opts.NetworkName,
		WorkingDir: "/workspace",
		Tty:        true,
		Stdin:      true,
		Binds:      binds,
		Mounts:     mounts,
		Env: []string{
			fmt.Sprintf("HTTP_PROXY=%s", proxyURL),
			fmt.Sprintf("HTTPS_PROXY=%s", proxyURL),
			fmt.Sprintf("http_proxy=%s", proxyURL),
			fmt.Sprintf("https_proxy=%s", proxyURL),
			"NO_PROXY=localhost,127.0.0.1",
			"LANG=C.UTF-8",
		},
		CapDrop: []string{"ALL"},
		Cmd:     []string{"claude", "--dangerouslySkipPermissions"},
	}
}
```

- [ ] **Step 4: Update existing tests to use VolumeName**

In `manager_test.go`, update all existing test cases that use `ClaudeDir` to use `VolumeName` instead. For each test, change:

```go
// Before:
ClaudeDir: "/home/user/.claude",

// After:
VolumeName: "test-volume",
```

Update bind count assertions: tests that expected a `.claude:ro` bind now expect one fewer bind and one `Mounts` entry. Specifically:

- `TestBuildClaudeContainerConfig`: binds 3 -> 2 (workspace + CA cert + shadow), mounts 1
- `TestBuildClaudeConfigWithoutOptionalFields`: binds 2 -> 1 (workspace only), mounts 1
- `TestBuildClaudeConfigAllBindMounts`: binds 4 -> 3, mounts 1. Remove `.claude:ro` from expectations map.
- `TestBuildClaudeConfigEnvShadowBind`: binds 4 -> 3, mounts 1
- `TestBuildClaudeConfigNoShadowWhenEmpty`: binds 2 -> 1, mounts 1
- Other tests: update `ClaudeDir` -> `VolumeName` and adjust bind count by -1

- [ ] **Step 5: Run all container tests**

Run: `go test ./internal/container/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/container/manager.go internal/container/manager_test.go
git commit -m "feat: replace ClaudeDir bind mount with VolumeName volume mount"
```

---

### Task 7: Add VolumeSettingsDir to scanner and update ClaudeScanner

**Files:**
- Modify: `internal/secrets/scanner.go`
- Modify: `internal/secrets/scanner_claude.go`
- Modify: `internal/secrets/scanner_claude_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/secrets/scanner_claude_test.go`:

```go
func TestClaudeScannerReadsFromVolumeSettingsDir(t *testing.T) {
	tmpDir := t.TempDir()
	volSettingsDir := filepath.Join(tmpDir, "vol-settings")
	os.MkdirAll(volSettingsDir, 0755)

	// Write a settings.json with a secret to the volume settings dir
	settings := `{"env":{"MY_SECRET_KEY":"sk-secret-value-12345678"}}`
	os.WriteFile(filepath.Join(volSettingsDir, "settings.json"), []byte(settings), 0644)

	kp, _ := crypto.GenerateKeyPair()
	scanner := secrets.NewClaudeScanner()
	result, err := scanner.Scan(secrets.ScanOpts{
		Workspace:         t.TempDir(),
		HomeDir:           t.TempDir(),
		PublicKey:          kp.PublicKey,
		PrivateKey:         kp.PrivateKey,
		TmpDir:            tmpDir,
		VolumeSettingsDir: volSettingsDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Mounts) == 0 {
		t.Fatal("expected shadow mounts for volume settings")
	}
	// Verify the shadow mount targets the container .claude path
	if result.Mounts[0].ContainerPath != "/home/airlock/.claude/settings.json" {
		t.Errorf("unexpected container path: %s", result.Mounts[0].ContainerPath)
	}
	if len(result.Mapping) == 0 {
		t.Error("expected mapping entries for encrypted secrets")
	}
}

func TestClaudeScannerSkipsVolumeWhenDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	volSettingsDir := filepath.Join(tmpDir, "vol-settings")
	os.MkdirAll(volSettingsDir, 0755)
	// No settings files written

	kp, _ := crypto.GenerateKeyPair()
	scanner := secrets.NewClaudeScanner()
	result, err := scanner.Scan(secrets.ScanOpts{
		Workspace:         t.TempDir(),
		HomeDir:           t.TempDir(),
		PublicKey:          kp.PublicKey,
		PrivateKey:         kp.PrivateKey,
		TmpDir:            tmpDir,
		VolumeSettingsDir: volSettingsDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected no mounts for empty volume settings, got %d", len(result.Mounts))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/secrets/ -run TestClaudeScannerReadsFromVolume -v`
Expected: FAIL (VolumeSettingsDir field does not exist)

- [ ] **Step 3: Add VolumeSettingsDir to ScanOpts**

In `internal/secrets/scanner.go`:

```go
type ScanOpts struct {
	Workspace         string
	HomeDir           string
	PublicKey         string
	PrivateKey        string
	TmpDir            string
	VolumeSettingsDir string // extracted settings from Docker volume
}
```

- [ ] **Step 4: Update ClaudeScanner.Scan to use VolumeSettingsDir**

In `internal/secrets/scanner_claude.go`:

```go
func (s *ClaudeScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	var files []claudeSettingsFile

	// Global settings: from volume if available, else from host HomeDir
	if opts.VolumeSettingsDir != "" {
		files = append(files,
			claudeSettingsFile{filepath.Join(opts.VolumeSettingsDir, "settings.json"), "/home/airlock/.claude/settings.json"},
			claudeSettingsFile{filepath.Join(opts.VolumeSettingsDir, "settings.local.json"), "/home/airlock/.claude/settings.local.json"},
		)
	} else {
		files = append(files,
			claudeSettingsFile{filepath.Join(opts.HomeDir, ".claude", "settings.json"), "/home/airlock/.claude/settings.json"},
			claudeSettingsFile{filepath.Join(opts.HomeDir, ".claude", "settings.local.json"), "/home/airlock/.claude/settings.local.json"},
		)
	}

	// Workspace settings always from host
	files = append(files,
		claudeSettingsFile{filepath.Join(opts.Workspace, ".claude", "settings.json"), "/workspace/.claude/settings.json"},
		claudeSettingsFile{filepath.Join(opts.Workspace, ".claude", "settings.local.json"), "/workspace/.claude/settings.local.json"},
	)

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
```

- [ ] **Step 5: Run all secrets tests**

Run: `go test ./internal/secrets/ -v`
Expected: ALL PASS (existing tests still pass because VolumeSettingsDir is empty string by default)

- [ ] **Step 6: Commit**

```bash
git add internal/secrets/scanner.go internal/secrets/scanner_claude.go internal/secrets/scanner_claude_test.go
git commit -m "feat: add VolumeSettingsDir to scanner for reading settings from Docker volume"
```

---

### Task 8: Add ExtractVolumeSettings to orchestrator and update SessionParams

**Files:**
- Modify: `internal/orchestrator/session.go`
- Modify: `internal/orchestrator/session_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/orchestrator/session_test.go`:

```go
func TestExtractVolumeSettings(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &mockRuntime{
		readFromVolumeFunc: func(ctx context.Context, volumeName, filePath, dstPath string) error {
			if filePath == "settings.json" {
				return os.WriteFile(dstPath, []byte(`{"env":{}}`), 0644)
			}
			return os.ErrNotExist
		},
	}

	dir, err := orchestrator.ExtractVolumeSettings(context.Background(), mock, "test-vol", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// settings.json should exist
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal("settings.json should have been extracted")
	}
	if string(data) != `{"env":{}}` {
		t.Errorf("unexpected content: %s", string(data))
	}

	// settings.local.json should not exist (was ErrNotExist)
	if _, err := os.Stat(filepath.Join(dir, "settings.local.json")); !os.IsNotExist(err) {
		t.Error("settings.local.json should not exist")
	}
}

func TestExtractVolumeSettingsAllMissing(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &mockRuntime{
		readFromVolumeFunc: func(ctx context.Context, volumeName, filePath, dstPath string) error {
			return os.ErrNotExist
		},
	}

	dir, err := orchestrator.ExtractVolumeSettings(context.Background(), mock, "test-vol", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Directory should exist but be empty
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected empty dir, got %d entries", len(entries))
	}
}
```

The `mockRuntime` needs a `readFromVolumeFunc` field and corresponding methods. Update the mock in the test file to include the three new interface methods:

```go
// Add to mockRuntime struct:
type mockRuntime struct {
	// ... existing fields ...
	ensureVolumeFunc   func(ctx context.Context, name string) error
	removeVolumeFunc   func(ctx context.Context, name string) error
	readFromVolumeFunc func(ctx context.Context, volumeName, filePath, dstPath string) error
}

func (m *mockRuntime) EnsureVolume(ctx context.Context, name string) error {
	if m.ensureVolumeFunc != nil {
		return m.ensureVolumeFunc(ctx, name)
	}
	return nil
}

func (m *mockRuntime) RemoveVolume(ctx context.Context, name string) error {
	if m.removeVolumeFunc != nil {
		return m.removeVolumeFunc(ctx, name)
	}
	return nil
}

func (m *mockRuntime) ReadFromVolume(ctx context.Context, volumeName, filePath, dstPath string) error {
	if m.readFromVolumeFunc != nil {
		return m.readFromVolumeFunc(ctx, volumeName, filePath, dstPath)
	}
	return os.ErrNotExist
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/orchestrator/ -run TestExtractVolumeSettings -v`
Expected: FAIL (function does not exist)

- [ ] **Step 3: Implement ExtractVolumeSettings and update SessionParams**

In `internal/orchestrator/session.go`:

```go
type SessionParams struct {
	ID           string
	Workspace    string
	VolumeName   string // Docker named volume for .claude state
	ClaudeDir    string // Deprecated: bind mount fallback
	Config       config.Config
	TmpDir       string
	ShadowMounts []secrets.ShadowMount
	MappingPath  string
}

// ExtractVolumeSettings reads settings.json and settings.local.json from a
// Docker volume into a temporary directory on the host. Missing files are
// silently skipped. Returns the path to the directory containing extracted files.
func ExtractVolumeSettings(ctx context.Context, runtime container.ContainerRuntime, volumeName, tmpDir string) (string, error) {
	dir := filepath.Join(tmpDir, "vol-settings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create vol-settings dir: %w", err)
	}

	for _, name := range []string{"settings.json", "settings.local.json"} {
		dst := filepath.Join(dir, name)
		err := runtime.ReadFromVolume(ctx, volumeName, name, dst)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("read %s from volume: %w", name, err)
		}
	}

	return dir, nil
}
```

Add `"os"` to the imports.

- [ ] **Step 4: Update StartSession to use VolumeName**

Replace ClaudeDir with VolumeName in both `StartSession` and `StartDetachedSession`:

```go
func StartSession(ctx context.Context, runtime container.ContainerRuntime, params SessionParams) error {
	cfg := params.Config

	// Ensure volume exists
	if params.VolumeName != "" {
		if err := runtime.EnsureVolume(ctx, params.VolumeName); err != nil {
			return fmt.Errorf("ensure volume: %w", err)
		}
	}

	fmt.Println("Creating airlock network...")
	netOpts := container.NetworkConfig(cfg.NetworkName)
	_, err := runtime.EnsureNetwork(ctx, netOpts)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

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
	}

	// ... rest unchanged (proxy start, CA cert, etc.)
```

Apply the same change to `StartDetachedSession`.

- [ ] **Step 5: Run all orchestrator tests**

Run: `go test ./internal/orchestrator/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/orchestrator/session.go internal/orchestrator/session_test.go
git commit -m "feat: add ExtractVolumeSettings and VolumeName to session orchestrator"
```

---

### Task 9: Update CLI run and start commands

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/start.go`

- [ ] **Step 1: Update run.go**

Replace the `claudeDir` logic in `run.go`:

```go
// Remove these lines:
// homeDir, _ := os.UserHomeDir()
// claudeDir := filepath.Join(homeDir, ".claude")

// Replace with:
volumeName := cfg.VolumeName
if volumeName == "" {
	volumeName = "airlock-claude-home"
}

homeDir, _ := os.UserHomeDir()

tmpDir, _ := os.MkdirTemp("", "airlock-*")
defer os.RemoveAll(tmpDir)

params := orchestrator.SessionParams{
	Workspace:  workspace,
	VolumeName: volumeName,
	Config:     cfg,
	TmpDir:     tmpDir,
}
```

Update the scanner section to extract from volume first:

```go
kp, kpErr := crypto.LoadKeyPair(keysDir)
if kpErr == nil {
	// Extract settings from volume for scanning
	volSettingsDir, extractErr := orchestrator.ExtractVolumeSettings(ctx, docker, volumeName, tmpDir)
	if extractErr != nil {
		return fmt.Errorf("extract volume settings: %w", extractErr)
	}

	scanners := []secrets.Scanner{
		secrets.NewClaudeScanner(),
	}
	if runEnvFile != "" {
		scanners = append(scanners, secrets.NewEnvScanner(runEnvFile, workspace))
	}
	scanResult, err := secrets.ScanAll(scanners, secrets.ScanOpts{
		Workspace:         workspace,
		HomeDir:           homeDir,
		PublicKey:          kp.PublicKey,
		PrivateKey:         kp.PrivateKey,
		TmpDir:            tmpDir,
		VolumeSettingsDir: volSettingsDir,
	})
	// ... rest unchanged
}
```

Note: `docker` (the runtime) must be created BEFORE the scanner section because `ExtractVolumeSettings` needs it. Move the Docker init block above the scanner block.

- [ ] **Step 2: Update start.go RunStart function**

Apply the same changes to `RunStart` in `start.go`:

```go
// Replace:
// claudeDir := filepath.Join(homeDir, ".claude")

// With:
volumeName := cfg.VolumeName
if volumeName == "" {
	volumeName = "airlock-claude-home"
}

params := orchestrator.SessionParams{
	ID:         id,
	Workspace:  workspace,
	VolumeName: volumeName,
	Config:     cfg,
	TmpDir:     tmpDir,
}
```

Note: `RunStart` already receives `runtime` as a parameter. Use it for `ExtractVolumeSettings`:

```go
volSettingsDir, extractErr := orchestrator.ExtractVolumeSettings(ctx, runtime, volumeName, tmpDir)
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Run all tests**

Run: `go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/run.go internal/cli/start.go
git commit -m "feat: wire volume mount into run and start CLI commands"
```

---

### Task 10: Add `airlock volume` subcommands

**Files:**
- Create: `internal/cli/volume.go`

- [ ] **Step 1: Implement volume status and reset commands**

Create `internal/cli/volume.go`:

```go
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
)

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Manage the persistent Claude Code state volume",
}

var volumeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show volume status",
	RunE: func(cmd *cobra.Command, args []string) error {
		airlockDir := ".airlock"
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config (run 'airlock init' first): %w", err)
		}

		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		ctx := context.Background()
		err = docker.EnsureVolume(ctx, volumeName)
		if err != nil {
			fmt.Printf("Volume: %s (not available: %v)\n", volumeName, err)
			return nil
		}
		fmt.Printf("Volume: %s (ready)\n", volumeName)
		return nil
	},
}

var volumeResetConfirm bool

var volumeResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete and recreate the volume (destroys all state including OAuth tokens)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !volumeResetConfirm {
			return fmt.Errorf("this will delete all Claude Code state (OAuth tokens, history, memory).\nRe-run with --confirm to proceed")
		}

		airlockDir := ".airlock"
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		ctx := context.Background()
		fmt.Printf("Removing volume %s...\n", volumeName)
		if err := docker.RemoveVolume(ctx, volumeName); err != nil {
			fmt.Printf("Warning: remove failed (volume may not exist): %v\n", err)
		}
		if err := docker.EnsureVolume(ctx, volumeName); err != nil {
			return fmt.Errorf("recreate volume: %w", err)
		}
		fmt.Printf("Volume %s has been reset.\n", volumeName)
		return nil
	},
}

func init() {
	volumeResetCmd.Flags().BoolVar(&volumeResetConfirm, "confirm", false, "confirm destructive reset")
	volumeCmd.AddCommand(volumeStatusCmd)
	volumeCmd.AddCommand(volumeResetCmd)
	rootCmd.AddCommand(volumeCmd)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./... && go run ./cmd/airlock volume --help`
Expected: Shows `status` and `reset` subcommands

- [ ] **Step 3: Commit**

```bash
git add internal/cli/volume.go
git commit -m "feat: add airlock volume status and reset commands"
```

---

### Task 11: Add `airlock config import` command

**Files:**
- Create: `internal/cli/config_import.go`

- [ ] **Step 1: Implement the import command**

Create `internal/cli/config_import.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
)

var (
	importFrom  string
	importAll   bool
	importItems string
	importForce bool
)

var defaultImportItems = []string{"CLAUDE.md", "rules", "settings.json", "settings.local.json"}
var optionalImportItems = []string{"plugins", "skills", "history.jsonl", "projects"}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage airlock configuration",
}

var configImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import host Claude Code config into the airlock volume",
	Long: `Copies selected files from the host's ~/.claude directory into the
persistent airlock volume. By default imports: CLAUDE.md, rules/,
settings.json, settings.local.json.

Existing files in the volume are skipped unless --force is set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		airlockDir := ".airlock"
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config (run 'airlock init' first): %w", err)
		}

		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}

		srcDir := importFrom
		if srcDir == "" {
			homeDir, _ := os.UserHomeDir()
			srcDir = filepath.Join(homeDir, ".claude")
		}
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			return fmt.Errorf("source directory does not exist: %s", srcDir)
		}

		items := defaultImportItems
		if importAll {
			items = append(items, optionalImportItems...)
		} else if importItems != "" {
			items = strings.Split(importItems, ",")
			for i, item := range items {
				items[i] = strings.TrimSpace(item)
			}
		}

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		ctx := context.Background()
		if err := docker.EnsureVolume(ctx, volumeName); err != nil {
			return fmt.Errorf("ensure volume: %w", err)
		}

		// Build copy script: for each item, check existence at source,
		// optionally skip if destination exists
		var cpParts []string
		for _, item := range items {
			srcPath := filepath.Join("/src", item)
			dstPath := filepath.Join("/dst", item)
			check := fmt.Sprintf("if [ -e %s ]; then ", srcPath)
			if !importForce {
				check += fmt.Sprintf("if [ -e %s ]; then echo 'SKIP %s (exists)'; else cp -a %s %s && echo 'OK %s'; fi",
					dstPath, item, srcPath, dstPath, item)
			} else {
				check += fmt.Sprintf("cp -a %s %s && echo 'OK %s'", srcPath, dstPath, item)
			}
			check += fmt.Sprintf("; else echo 'SKIP %s (not in source)'; fi", item)
			cpParts = append(cpParts, check)
		}
		script := strings.Join(cpParts, " ; ")

		importCfg := container.ContainerConfig{
			Image: cfg.ContainerImage,
			Name:  "airlock-importer",
			Binds: []string{
				fmt.Sprintf("%s:/src:ro", srcDir),
				fmt.Sprintf("%s:/dst", volumeName),
			},
			Cmd: []string{"sh", "-c", script},
		}

		fmt.Printf("Importing from %s into volume %s...\n", srcDir, volumeName)
		if err := docker.RunAttached(ctx, importCfg); err != nil {
			// Non-zero exit from cp is not fatal; output already printed
			if !strings.Contains(err.Error(), "exited with code") {
				return fmt.Errorf("import failed: %w", err)
			}
		}

		fmt.Println("\nSettings imported. Secrets will be encrypted on next container start.")
		return nil
	},
}

func init() {
	configImportCmd.Flags().StringVar(&importFrom, "from", "", "source directory (default: ~/.claude)")
	configImportCmd.Flags().BoolVar(&importAll, "all", false, "import all items including history and projects")
	configImportCmd.Flags().StringVar(&importItems, "items", "", "comma-separated items to import")
	configImportCmd.Flags().BoolVar(&importForce, "force", false, "overwrite existing files in volume")
	configCmd.AddCommand(configImportCmd)
	rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 2: Build and verify help**

Run: `go build ./... && go run ./cmd/airlock config import --help`
Expected: Shows `--from`, `--all`, `--items`, `--force` flags

- [ ] **Step 3: Commit**

```bash
git add internal/cli/config_import.go
git commit -m "feat: add airlock config import command for host-to-volume migration"
```

---

### Task 12: Add `airlock config export` command

**Files:**
- Create: `internal/cli/config_export.go`

- [ ] **Step 1: Implement the export command**

Create `internal/cli/config_export.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
)

var (
	exportTo    string
	exportItems string
)

var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export airlock volume config to a host directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		airlockDir := ".airlock"
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}

		dstDir := exportTo
		if dstDir == "" {
			homeDir, _ := os.UserHomeDir()
			dstDir = filepath.Join(homeDir, "airlock-claude-export")
		}
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("create export directory: %w", err)
		}

		items := defaultImportItems
		if exportItems != "" {
			items = strings.Split(exportItems, ",")
			for i, item := range items {
				items[i] = strings.TrimSpace(item)
			}
		}

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		ctx := context.Background()

		var cpParts []string
		for _, item := range items {
			srcPath := filepath.Join("/src", item)
			dstPath := filepath.Join("/dst", item)
			cpParts = append(cpParts, fmt.Sprintf("if [ -e %s ]; then cp -a %s %s && echo 'OK %s'; else echo 'SKIP %s'; fi", srcPath, srcPath, dstPath, item, item))
		}
		script := strings.Join(cpParts, " ; ")

		exportCfg := container.ContainerConfig{
			Image: cfg.ContainerImage,
			Name:  "airlock-exporter",
			Binds: []string{
				fmt.Sprintf("%s:/src:ro", volumeName),
				fmt.Sprintf("%s:/dst", dstDir),
			},
			Cmd: []string{"sh", "-c", script},
		}

		fmt.Printf("Exporting from volume %s to %s...\n", volumeName, dstDir)
		if err := docker.RunAttached(ctx, exportCfg); err != nil {
			if !strings.Contains(err.Error(), "exited with code") {
				return fmt.Errorf("export failed: %w", err)
			}
		}

		fmt.Printf("\nExported to %s\n", dstDir)
		return nil
	},
}

func init() {
	configExportCmd.Flags().StringVar(&exportTo, "to", "", "destination directory (default: ~/airlock-claude-export/)")
	configExportCmd.Flags().StringVar(&exportItems, "items", "", "comma-separated items to export")
	configCmd.AddCommand(configExportCmd)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/cli/config_export.go
git commit -m "feat: add airlock config export command for volume backup"
```

---

### Task 13: Update airlock init to create volume

**Files:**
- Modify: `internal/cli/init_cmd.go`

- [ ] **Step 1: Update RunInit to create volume**

```go
func RunInit(airlockDir string) error {
	keysDir := filepath.Join(airlockDir, "keys")

	if _, err := os.Stat(airlockDir); err == nil {
		return fmt.Errorf(".airlock/ already exists; remove it first to reinitialize")
	}

	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("create .airlock/keys: %w", err)
	}

	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generate keypair: %w", err)
	}

	if err := crypto.SaveKeyPair(kp, keysDir); err != nil {
		return fmt.Errorf("save keypair: %w", err)
	}

	cfg := config.Default()
	if err := config.Save(cfg, airlockDir); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Create persistent volume for Claude Code state
	docker, dockerErr := container.NewDocker()
	if dockerErr == nil {
		defer docker.Close()
		ctx := context.Background()
		if err := docker.EnsureVolume(ctx, cfg.VolumeName); err != nil {
			fmt.Printf("Warning: could not create volume %s: %v\n", cfg.VolumeName, err)
		} else {
			fmt.Printf("  Volume:    %s\n", cfg.VolumeName)
		}
	}

	fmt.Println("Initialized .airlock/")
	fmt.Printf("  Public key: %s\n", kp.PublicKey)
	fmt.Println("  Config:     .airlock/config.yaml")
	fmt.Println()
	fmt.Println("Add .airlock/keys/ to .gitignore")
	fmt.Println()
	fmt.Println("Run 'airlock config import' to import your host Claude Code settings.")

	return nil
}
```

Add imports: `"context"` and `"github.com/taeikkim92/airlock/internal/container"`.

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/cli/init_cmd.go
git commit -m "feat: create persistent volume during airlock init"
```

---

### Task 14: Full Go test suite

**Files:** (no new files)

- [ ] **Step 1: Run all Go tests**

Run: `go test -race -cover ./...`
Expected: ALL PASS, no race conditions

- [ ] **Step 2: Fix any failures**

If any test fails, fix the root cause. Common issues:
- Tests in `session_test.go` referencing `ClaudeDir` instead of `VolumeName`
- Mock runtime missing new interface methods
- Import path issues

- [ ] **Step 3: Commit fixes if any**

```bash
git add -A
git commit -m "fix: update tests for volume mount architecture"
```

---

### Task 15: GUI -- Volume status in GlobalSettingsSheet

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`

- [ ] **Step 1: Add volume status section**

Add after the "Network Defaults" section:

```swift
Section("Claude Code State Volume") {
    HStack {
        Text("airlock-claude-home")
            .font(.system(.body, design: .monospaced))
        Spacer()
        Text(volumeStatus)
            .foregroundStyle(volumeStatus == "Ready" ? .green : .secondary)
    }

    Button("Import from Host ~/.claude...") {
        showImportSheet = true
    }

    Button("Reset Volume...") {
        showResetAlert = true
    }
    .foregroundStyle(.red)
}
```

Add state variables:

```swift
@State private var volumeStatus = "Checking..."
@State private var showImportSheet = false
@State private var showResetAlert = false
```

Add `.onAppear` modifier to check volume status and `.sheet` / `.alert` modifiers.

- [ ] **Step 2: Build GUI**

Run: `make gui-build`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift
git commit -m "feat: add volume status and import button to global settings GUI"
```

---

### Task 16: GUI -- ImportConfigSheet

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Views/Settings/ImportConfigSheet.swift`

- [ ] **Step 1: Create the import sheet view**

```swift
import SwiftUI

@MainActor
struct ImportConfigSheet: View {
    @Environment(\.dismiss) private var dismiss
    @State private var selectedItems: Set<String> = ["CLAUDE.md", "rules", "settings.json", "settings.local.json"]
    @State private var forceOverwrite = false
    @State private var isImporting = false
    @State private var result: String?

    private let allItems: [(name: String, description: String, isDefault: Bool)] = [
        ("CLAUDE.md", "Global instructions", true),
        ("rules", "Custom rules directory", true),
        ("settings.json", "Claude Code settings", true),
        ("settings.local.json", "Local settings overrides", true),
        ("plugins", "Installed plugins", false),
        ("skills", "Custom skills", false),
        ("history.jsonl", "Command history", false),
        ("projects", "Project-specific memory", false),
    ]

    var body: some View {
        VStack(spacing: 0) {
            Form {
                Section("Select items to import from ~/.claude") {
                    ForEach(allItems, id: \.name) { item in
                        Toggle(isOn: Binding(
                            get: { selectedItems.contains(item.name) },
                            set: { if $0 { selectedItems.insert(item.name) } else { selectedItems.remove(item.name) } }
                        )) {
                            VStack(alignment: .leading) {
                                Text(item.name)
                                    .font(.system(.body, design: .monospaced))
                                Text(item.description)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }

                Section {
                    Toggle("Force overwrite existing files", isOn: $forceOverwrite)
                }
            }
            .formStyle(.grouped)

            if let result {
                Text(result)
                    .font(.system(size: 11, design: .monospaced))
                    .padding(.horizontal)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button("Import") { performImport() }
                    .keyboardShortcut(.defaultAction)
                    .disabled(isImporting || selectedItems.isEmpty)
            }
            .padding()
        }
        .frame(width: 500, height: 520)
    }

    private func performImport() {
        isImporting = true
        Task {
            let cli = CLIService()
            var args = ["config", "import", "--items", selectedItems.sorted().joined(separator: ",")]
            if forceOverwrite { args.append("--force") }
            let output = try? await cli.run(args: args, workingDirectory: FileManager.default.homeDirectoryForCurrentUser.path)
            result = output?.stdout ?? "Import completed"
            isImporting = false
        }
    }
}
```

- [ ] **Step 2: Build GUI**

Run: `make gui-build`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/ImportConfigSheet.swift
git commit -m "feat: add ImportConfigSheet GUI for host-to-volume migration"
```

---

### Task 17: Final verification

- [ ] **Step 1: Run full Go test suite**

Run: `make test`
Expected: ALL PASS with `-race -cover`

- [ ] **Step 2: Run Python tests**

Run: `make test-python`
Expected: ALL PASS (no changes to proxy addon)

- [ ] **Step 3: Build Docker images**

Run: `make docker-build`
Expected: Both `airlock-claude:latest` and `airlock-proxy:latest` build successfully

- [ ] **Step 4: Build GUI**

Run: `make gui-build`
Expected: Build succeeds

- [ ] **Step 5: Build CLI**

Run: `make build`
Expected: `bin/airlock` binary created

- [ ] **Step 6: Smoke test**

```bash
cd /tmp && mkdir test-vol && cd test-vol
../../path/to/bin/airlock init
../../path/to/bin/airlock volume status
# Expected: "Volume: airlock-claude-home (ready)"
```

- [ ] **Step 7: Commit any remaining fixes**

```bash
git add -A
git commit -m "chore: final verification fixes for writable volume feature"
```
