package orchestrator_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/orchestrator"
	"github.com/taeikkim92/airlock/internal/secrets"
)

type MockRuntime struct {
	Networks        map[string]string
	Containers      map[string]string
	StoppedNames    []string
	RemovedNames    []string
	AttachedConfig  *container.ContainerConfig
	DetachedConfigs []container.ContainerConfig
	CopiedFiles     []string
	FailOn          string
}

func NewMockRuntime() *MockRuntime {
	return &MockRuntime{Networks: make(map[string]string), Containers: make(map[string]string)}
}

func (m *MockRuntime) EnsureNetwork(_ context.Context, opts container.NetworkOpts) (string, error) {
	if m.FailOn == "EnsureNetwork" {
		return "", fmt.Errorf("mock failure")
	}
	id := "net-" + opts.Name
	m.Networks[opts.Name] = id
	return id, nil
}

func (m *MockRuntime) RunDetached(_ context.Context, cfg container.ContainerConfig) (string, error) {
	if m.FailOn == "RunDetached" {
		return "", fmt.Errorf("mock failure")
	}
	id := "ctr-" + cfg.Name
	m.Containers[cfg.Name] = id
	m.DetachedConfigs = append(m.DetachedConfigs, cfg)
	return id, nil
}

func (m *MockRuntime) RunAttached(_ context.Context, cfg container.ContainerConfig) error {
	if m.FailOn == "RunAttached" {
		return fmt.Errorf("mock failure")
	}
	m.AttachedConfig = &cfg
	m.Containers[cfg.Name] = "ctr-" + cfg.Name
	return nil
}

func (m *MockRuntime) Stop(_ context.Context, name string) error {
	m.StoppedNames = append(m.StoppedNames, name)
	return nil
}

func (m *MockRuntime) Remove(_ context.Context, name string) error {
	m.RemovedNames = append(m.RemovedNames, name)
	delete(m.Containers, name)
	return nil
}

func (m *MockRuntime) RemoveNetwork(_ context.Context, name string) error {
	delete(m.Networks, name)
	return nil
}

func (m *MockRuntime) ConnectNetwork(_ context.Context, networkID, containerID string) error {
	return nil
}

func (m *MockRuntime) CopyFromContainer(_ context.Context, containerName, srcPath, dstPath string) error {
	if m.FailOn == "CopyFromContainer" {
		return fmt.Errorf("mock failure")
	}
	m.CopiedFiles = append(m.CopiedFiles, fmt.Sprintf("%s:%s->%s", containerName, srcPath, dstPath))
	os.MkdirAll(filepath.Dir(dstPath), 0755)
	os.WriteFile(dstPath, []byte("fake-ca-cert"), 0644)
	return nil
}

func (m *MockRuntime) WaitForFile(_ context.Context, containerName, path string, maxRetries int) error {
	if m.FailOn == "WaitForFile" {
		return fmt.Errorf("mock failure")
	}
	return nil
}

func (m *MockRuntime) ListContainers(_ context.Context, prefix string) ([]container.ContainerInfo, error) {
	return nil, nil
}

func TestStartSessionCreatesNetworkAndContainers(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	tmpDir := t.TempDir()
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test-workspace", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: tmpDir,
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if _, ok := mock.Networks[cfg.NetworkName]; !ok {
		t.Error("network not created")
	}
	if len(mock.DetachedConfigs) == 0 {
		t.Error("proxy not started")
	}
	if mock.AttachedConfig == nil {
		t.Error("Claude container not started")
	}
}

func TestStartSessionWithEnvFile(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.enc")
	os.WriteFile(envPath, []byte("KEY='ENC[age:xxx]'\n"), 0644)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, []byte(`{"ENC[age:xxx]":"secret"}`), 0600)
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test-workspace", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: tmpDir,
		ShadowMounts: []secrets.ShadowMount{{HostPath: envPath, ContainerPath: "/run/airlock/env.enc"}},
		MappingPath:  mappingPath,
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if len(mock.DetachedConfigs) == 0 {
		t.Fatal("proxy not started")
	}
	proxyCfg := mock.DetachedConfigs[0]
	hasMappingBind := false
	for _, bind := range proxyCfg.Binds {
		if strings.Contains(bind, "mapping.json") {
			hasMappingBind = true
		}
	}
	if !hasMappingBind {
		t.Error("proxy missing mapping bind mount")
	}

	// Verify Claude container received the shadow mount bind
	if mock.AttachedConfig == nil {
		t.Fatal("claude container not started")
	}
	hasEnvBind := false
	for _, bind := range mock.AttachedConfig.Binds {
		if strings.Contains(bind, "env.enc") && strings.Contains(bind, ":ro") {
			hasEnvBind = true
		}
	}
	if !hasEnvBind {
		t.Error("claude container missing env.enc read-only bind mount")
	}
}

func TestCleanupSession(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	orchestrator.CleanupSession(context.Background(), mock, cfg, "")
	if len(mock.RemovedNames) < 2 {
		t.Error("expected at least 2 containers removed")
	}
}

func TestCleanupSessionRemovesBothContainers(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	orchestrator.CleanupSession(context.Background(), mock, cfg, "")

	hasClaudeRemove := false
	hasProxyRemove := false
	for _, name := range mock.RemovedNames {
		if name == "airlock-claude" {
			hasClaudeRemove = true
		}
		if name == "airlock-proxy" {
			hasProxyRemove = true
		}
	}
	if !hasClaudeRemove {
		t.Error("airlock-claude not removed during cleanup")
	}
	if !hasProxyRemove {
		t.Error("airlock-proxy not removed during cleanup")
	}
}

func TestCleanupSessionRemovesNetwork(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	mock.Networks[cfg.NetworkName] = "net-id"
	orchestrator.CleanupSession(context.Background(), mock, cfg, "")
	if _, exists := mock.Networks[cfg.NetworkName]; exists {
		t.Error("network should be removed during cleanup")
	}
}

type FailingMockRuntime struct {
	MockRuntime
	RemoveFailNames   map[string]bool
	NetworkRemoveFail bool
	RemoveCalls       []string
	NetworkRemoved    bool
}

func NewFailingMockRuntime() *FailingMockRuntime {
	return &FailingMockRuntime{
		MockRuntime:     *NewMockRuntime(),
		RemoveFailNames: make(map[string]bool),
	}
}

func (m *FailingMockRuntime) Remove(_ context.Context, name string) error {
	m.RemoveCalls = append(m.RemoveCalls, name)
	if m.RemoveFailNames[name] {
		return fmt.Errorf("remove failed for %s", name)
	}
	delete(m.Containers, name)
	return nil
}

func (m *FailingMockRuntime) RemoveNetwork(_ context.Context, name string) error {
	m.NetworkRemoved = true
	if m.NetworkRemoveFail {
		return fmt.Errorf("network remove failed")
	}
	delete(m.Networks, name)
	return nil
}

func TestCleanupSessionContinuesOnRemoveFailure(t *testing.T) {
	mock := NewFailingMockRuntime()
	mock.RemoveFailNames["airlock-claude"] = true
	cfg := config.Default()

	// CleanupSession should not panic even when Remove fails
	orchestrator.CleanupSession(context.Background(), mock, cfg, "")

	// Both containers should have been attempted
	if len(mock.RemoveCalls) < 2 {
		t.Errorf("expected 2 remove calls, got %d: %v", len(mock.RemoveCalls), mock.RemoveCalls)
	}

	// Network removal should still have been attempted
	if !mock.NetworkRemoved {
		t.Error("network removal should be attempted even when container remove fails")
	}
}

func TestCleanupSessionContinuesOnNetworkRemoveFailure(t *testing.T) {
	mock := NewFailingMockRuntime()
	mock.NetworkRemoveFail = true
	cfg := config.Default()

	// Should not panic
	orchestrator.CleanupSession(context.Background(), mock, cfg, "")

	// Both containers should still have been removed
	if len(mock.RemoveCalls) < 2 {
		t.Errorf("expected 2 remove calls even with network failure, got %d", len(mock.RemoveCalls))
	}
}

func TestStartSessionNetworkFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "EnsureNetwork"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when network fails")
	}
}

func TestStartSessionProxyFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "RunDetached"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when proxy start fails")
	}
	if !strings.Contains(err.Error(), "start proxy") {
		t.Errorf("expected 'start proxy' in error, got: %v", err)
	}
}

func TestStartSessionRunAttachedFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "RunAttached"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when RunAttached fails")
	}
	if !strings.Contains(err.Error(), "run claude") {
		t.Errorf("expected 'run claude' in error, got: %v", err)
	}
	// Verify proxy and network were still created before claude failed
	if len(mock.DetachedConfigs) == 0 {
		t.Error("proxy should have been started before claude failure")
	}
}

func TestStartSessionWaitForFileFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "WaitForFile"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when WaitForFile fails")
	}
	if !strings.Contains(err.Error(), "proxy CA cert") {
		t.Errorf("expected 'proxy CA cert' in error, got: %v", err)
	}
}

func TestStartSessionCopyFromContainerFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "CopyFromContainer"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when CopyFromContainer fails")
	}
	if !strings.Contains(err.Error(), "extract proxy CA cert") {
		t.Errorf("expected 'extract proxy CA cert' in error, got: %v", err)
	}
}

func TestStartSessionVerifiesProxyConfig(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	cfg.PassthroughHosts = []string{"api.anthropic.com", "custom.example.com"}
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if len(mock.DetachedConfigs) == 0 {
		t.Fatal("proxy not started")
	}
	proxyCfg := mock.DetachedConfigs[0]

	// Verify CapDrop is set
	if len(proxyCfg.CapDrop) == 0 || proxyCfg.CapDrop[0] != "ALL" {
		t.Error("proxy CapDrop should be [ALL]")
	}

	// Verify passthrough hosts propagated to env
	hasPassthrough := false
	for _, env := range proxyCfg.Env {
		if strings.Contains(env, "custom.example.com") {
			hasPassthrough = true
		}
	}
	if !hasPassthrough {
		t.Error("custom passthrough host not propagated to proxy env")
	}
}

func TestStartSessionVerifiesClaudeConfig(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	params := orchestrator.SessionParams{
		Workspace: "/home/user/project", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if mock.AttachedConfig == nil {
		t.Fatal("claude container not started")
	}
	cc := mock.AttachedConfig

	// Verify TTY and stdin enabled
	if !cc.Tty {
		t.Error("claude container should have TTY enabled")
	}
	if !cc.Stdin {
		t.Error("claude container should have stdin enabled")
	}

	// Verify CapDrop
	if len(cc.CapDrop) == 0 || cc.CapDrop[0] != "ALL" {
		t.Error("claude CapDrop should be [ALL]")
	}

	// Verify command
	if len(cc.Cmd) < 2 || cc.Cmd[0] != "claude" {
		t.Errorf("unexpected claude cmd: %v", cc.Cmd)
	}

	// Verify proxy env vars
	proxyEnvCount := 0
	for _, env := range cc.Env {
		if strings.Contains(env, "PROXY") || strings.Contains(env, "proxy") {
			proxyEnvCount++
		}
	}
	if proxyEnvCount < 5 {
		t.Errorf("expected at least 5 proxy env vars, got %d", proxyEnvCount)
	}

	// Verify workspace bind
	hasWorkspace := false
	for _, bind := range cc.Binds {
		if strings.Contains(bind, "/home/user/project:/workspace") {
			hasWorkspace = true
		}
	}
	if !hasWorkspace {
		t.Error("workspace bind mount not found")
	}

	// Verify .claude bind (read-only)
	hasClaude := false
	for _, bind := range cc.Binds {
		if strings.Contains(bind, ".claude") && strings.Contains(bind, ":ro") {
			hasClaude = true
		}
	}
	if !hasClaude {
		t.Error(".claude read-only bind mount not found")
	}
}

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
	if len(mock.DetachedConfigs) < 2 {
		t.Errorf("expected 2 detached containers, got %d", len(mock.DetachedConfigs))
	}
	if mock.AttachedConfig != nil {
		t.Error("detached session should not attach any container")
	}
	for _, cfg := range mock.DetachedConfigs {
		if !strings.Contains(cfg.Name, "test123") {
			t.Errorf("container name %s should contain workspace ID", cfg.Name)
		}
	}
}

func TestStartDetachedSessionNetworkName(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	params := orchestrator.SessionParams{
		ID: "ws42", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartDetachedSession failed: %v", err)
	}
	expectedNet := cfg.NetworkName + "-ws42"
	if _, ok := mock.Networks[expectedNet]; !ok {
		t.Errorf("expected network %s, got networks: %v", expectedNet, mock.Networks)
	}
}

func TestStartDetachedSessionNetworkFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "EnsureNetwork"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		ID: "test1", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when network fails")
	}
	if !strings.Contains(err.Error(), "create network") {
		t.Errorf("expected 'create network' in error, got: %v", err)
	}
}

func TestStartDetachedSessionProxyFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "RunDetached"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		ID: "test1", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when proxy start fails")
	}
	if !strings.Contains(err.Error(), "start proxy") {
		t.Errorf("expected 'start proxy' in error, got: %v", err)
	}
}

func TestStartDetachedSessionWaitForFileFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "WaitForFile"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		ID: "test1", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when WaitForFile fails")
	}
	if !strings.Contains(err.Error(), "proxy CA cert") {
		t.Errorf("expected 'proxy CA cert' in error, got: %v", err)
	}
}

func TestStartDetachedSessionCopyFailure(t *testing.T) {
	mock := NewMockRuntime()
	mock.FailOn = "CopyFromContainer"
	cfg := config.Default()
	params := orchestrator.SessionParams{
		ID: "test1", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err == nil {
		t.Error("expected error when CopyFromContainer fails")
	}
	if !strings.Contains(err.Error(), "extract proxy CA cert") {
		t.Errorf("expected 'extract proxy CA cert' in error, got: %v", err)
	}
}

func TestStartDetachedSessionUsesKeepAliveEntrypoint(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	params := orchestrator.SessionParams{
		ID: "keep1", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartDetachedSession failed: %v", err)
	}

	// Find the claude container config (second detached)
	var claudeCfg *container.ContainerConfig
	for i := range mock.DetachedConfigs {
		if strings.Contains(mock.DetachedConfigs[i].Name, "airlock-claude") {
			claudeCfg = &mock.DetachedConfigs[i]
		}
	}
	if claudeCfg == nil {
		t.Fatal("claude detached config not found")
	}
	if len(claudeCfg.Cmd) != 1 || claudeCfg.Cmd[0] != "/usr/local/bin/entrypoint-keepalive.sh" {
		t.Errorf("expected keepalive entrypoint, got cmd: %v", claudeCfg.Cmd)
	}
	if claudeCfg.Tty {
		t.Error("detached claude should not have TTY")
	}
	if claudeCfg.Stdin {
		t.Error("detached claude should not have Stdin")
	}
}

func TestStartDetachedSessionWithEnvFile(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.enc")
	os.WriteFile(envPath, []byte("KEY='ENC[age:xxx]'\n"), 0644)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, []byte(`{"ENC[age:xxx]":"secret"}`), 0600)
	params := orchestrator.SessionParams{
		ID: "env1", Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: tmpDir,
		ShadowMounts: []secrets.ShadowMount{{HostPath: envPath, ContainerPath: "/run/airlock/env.enc"}},
		MappingPath:  mappingPath,
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartDetachedSession failed: %v", err)
	}

	// Verify proxy got mapping bind
	if len(mock.DetachedConfigs) < 1 {
		t.Fatal("proxy not started")
	}
	proxyCfg := mock.DetachedConfigs[0]
	hasMappingBind := false
	for _, bind := range proxyCfg.Binds {
		if strings.Contains(bind, "mapping.json") {
			hasMappingBind = true
		}
	}
	if !hasMappingBind {
		t.Error("proxy missing mapping bind mount")
	}

	// Verify claude container got env.enc bind
	var claudeCfg *container.ContainerConfig
	for i := range mock.DetachedConfigs {
		if strings.Contains(mock.DetachedConfigs[i].Name, "airlock-claude") {
			claudeCfg = &mock.DetachedConfigs[i]
		}
	}
	if claudeCfg == nil {
		t.Fatal("claude container not found")
	}
	hasEnvBind := false
	for _, bind := range claudeCfg.Binds {
		if strings.Contains(bind, "env.enc") && strings.Contains(bind, ":ro") {
			hasEnvBind = true
		}
	}
	if !hasEnvBind {
		t.Error("claude container missing env.enc read-only bind mount")
	}
}

func TestStartSessionShadowBindPropagated(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.enc")
	os.WriteFile(envPath, []byte("KEY='ENC[age:xxx]'\n"), 0644)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, []byte(`{"ENC[age:xxx]":"secret"}`), 0600)

	params := orchestrator.SessionParams{
		Workspace: "/tmp/test-workspace",
		ClaudeDir: tmpDir,
		Config:    cfg,
		TmpDir:    tmpDir,
		ShadowMounts: []secrets.ShadowMount{
			{HostPath: envPath, ContainerPath: "/run/airlock/env.enc"},
			{HostPath: envPath, ContainerPath: "/workspace/.env"},
		},
		MappingPath: mappingPath,
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if mock.AttachedConfig == nil {
		t.Fatal("claude container not started")
	}
	hasShadow := false
	for _, bind := range mock.AttachedConfig.Binds {
		if strings.Contains(bind, "/workspace/.env") && strings.Contains(bind, ":ro") {
			hasShadow = true
		}
	}
	if !hasShadow {
		t.Errorf("shadow bind not propagated to claude config: %v", mock.AttachedConfig.Binds)
	}
}

func TestStartDetachedSessionShadowBindPropagated(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.enc")
	os.WriteFile(envPath, []byte("KEY='ENC[age:xxx]'\n"), 0644)
	mappingPath := filepath.Join(tmpDir, "mapping.json")
	os.WriteFile(mappingPath, []byte(`{"ENC[age:xxx]":"secret"}`), 0600)

	params := orchestrator.SessionParams{
		ID:        "shadow-test",
		Workspace: "/tmp/test-workspace",
		ClaudeDir: tmpDir,
		Config:    cfg,
		TmpDir:    tmpDir,
		ShadowMounts: []secrets.ShadowMount{
			{HostPath: envPath, ContainerPath: "/run/airlock/env.enc"},
			{HostPath: envPath, ContainerPath: "/workspace/.env"},
		},
		MappingPath: mappingPath,
	}
	err := orchestrator.StartDetachedSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartDetachedSession failed: %v", err)
	}
	if len(mock.DetachedConfigs) < 2 {
		t.Fatalf("expected 2 detached configs, got %d", len(mock.DetachedConfigs))
	}
	claudeCfg := mock.DetachedConfigs[1]
	hasShadow := false
	for _, bind := range claudeCfg.Binds {
		if strings.Contains(bind, "/workspace/.env") && strings.Contains(bind, ":ro") {
			hasShadow = true
		}
	}
	if !hasShadow {
		t.Errorf("shadow bind not propagated to detached claude: %v", claudeCfg.Binds)
	}
}

func TestStartSessionCACertMountedInClaude(t *testing.T) {
	mock := NewMockRuntime()
	cfg := config.Default()
	params := orchestrator.SessionParams{
		Workspace: "/tmp/test", ClaudeDir: "/home/user/.claude",
		Config: cfg, TmpDir: t.TempDir(),
	}
	err := orchestrator.StartSession(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if mock.AttachedConfig == nil {
		t.Fatal("claude container not started")
	}

	// Verify CA cert bind mount exists
	hasCACert := false
	for _, bind := range mock.AttachedConfig.Binds {
		if strings.Contains(bind, "ca-certificates") && strings.Contains(bind, ":ro") {
			hasCACert = true
		}
	}
	if !hasCACert {
		t.Error("CA cert should be mounted read-only in claude container")
	}

	// Verify CopyFromContainer was called for proxy CA cert
	if len(mock.CopiedFiles) == 0 {
		t.Error("CopyFromContainer should have been called for CA cert")
	}
}
