package container_test

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/secrets"
)

func TestContainerOptsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    container.RunOpts
		wantErr bool
	}{
		{
			name: "valid opts",
			opts: container.RunOpts{
				Workspace: "/tmp/workspace", Image: "airlock-claude:latest",
				ProxyImage: "airlock-proxy:latest", NetworkName: "airlock-net",
				MappingPath: "/tmp/mapping.json",
				ClaudeDir: "/home/user/.claude", ProxyPort: 8080,
			},
			wantErr: false,
		},
		{name: "missing workspace", opts: container.RunOpts{Image: "airlock-claude:latest"}, wantErr: true},
		{name: "missing image", opts: container.RunOpts{Workspace: "/tmp/workspace"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildProxyContainerConfig(t *testing.T) {
	opts := container.RunOpts{
		ProxyImage: "airlock-proxy:latest", NetworkName: "airlock-net",
		MappingPath: "/tmp/mapping.json", ProxyPort: 8080,
		PassthroughHosts: []string{"api.anthropic.com"},
	}
	cfg := container.BuildProxyConfig(opts)
	if cfg.Image != "airlock-proxy:latest" {
		t.Errorf("expected proxy image, got %s", cfg.Image)
	}
	if len(cfg.Binds) == 0 {
		t.Error("expected mapping bind mount")
	}
}

func TestBuildClaudeContainerConfig(t *testing.T) {
	opts := container.RunOpts{
		Workspace: "/home/user/project", Image: "airlock-claude:latest",
		NetworkName: "airlock-net",
		ShadowMounts: []secrets.ShadowMount{{HostPath: "/tmp/.env.enc", ContainerPath: "/run/airlock/env.enc"}},
		VolumeName: "test-volume", ProxyPort: 8080, CACertPath: "/tmp/ca.pem",
	}
	cfg := container.BuildClaudeConfig(opts)
	if cfg.Image != "airlock-claude:latest" {
		t.Errorf("expected claude image, got %s", cfg.Image)
	}
	// workspace + ca-cert + 1 shadow mount = 3 binds; .claude is a volume mount
	if len(cfg.Binds) < 3 {
		t.Errorf("expected at least 3 bind mounts, got %d", len(cfg.Binds))
	}
	hasProxy := false
	for _, env := range cfg.Env {
		if env == "HTTP_PROXY=http://airlock-proxy:8080" {
			hasProxy = true
		}
	}
	if !hasProxy {
		t.Error("expected HTTP_PROXY env var")
	}
}

func TestBuildClaudeConfigWithoutOptionalFields(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
		// ShadowMounts and CACertPath intentionally omitted
	}
	cfg := container.BuildClaudeConfig(opts)

	// Without shadow mounts and CA cert, should have exactly 1 bind mount (workspace only)
	// .claude is provided via volume mount
	if len(cfg.Binds) != 1 {
		t.Errorf("expected 1 bind mount without optional fields, got %d: %v", len(cfg.Binds), cfg.Binds)
	}

	// Verify no env.enc bind
	for _, bind := range cfg.Binds {
		if strings.Contains(bind, "env.enc") {
			t.Error("should not have env.enc bind when ShadowMounts is empty")
		}
		if strings.Contains(bind, "ca-certificates") {
			t.Error("should not have CA cert bind when CACertPath is empty")
		}
	}
}

func TestBuildClaudeConfigAllBindMounts(t *testing.T) {
	opts := container.RunOpts{
		Workspace:    "/home/user/project",
		Image:        "airlock-claude:latest",
		NetworkName:  "airlock-net",
		VolumeName:   "test-volume",
		ProxyPort:    8080,
		ShadowMounts: []secrets.ShadowMount{{HostPath: "/tmp/env.enc", ContainerPath: "/run/airlock/env.enc"}},
		CACertPath:   "/tmp/ca.pem",
	}
	cfg := container.BuildClaudeConfig(opts)

	// With VolumeName: workspace + ca-cert + 1 shadow mount = 3 binds; .claude is a volume mount
	if len(cfg.Binds) != 3 {
		t.Errorf("expected 3 bind mounts with all optional fields, got %d: %v", len(cfg.Binds), cfg.Binds)
	}

	// Verify bind mounts present with correct paths and modes
	expectations := map[string]string{
		"/workspace/": "/home/user/project:/workspace/",
		"env.enc:ro": "env.enc:ro",
		"ca-cert:ro": "ca-certificates/airlock-proxy.crt:ro",
	}
	for label, substr := range expectations {
		found := false
		for _, bind := range cfg.Binds {
			if strings.Contains(bind, substr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("bind mount %q not found in: %v", label, cfg.Binds)
		}
	}

	// .claude must be a volume mount, not a bind
	if len(cfg.Mounts) != 1 {
		t.Fatalf("expected 1 volume mount for .claude, got %d", len(cfg.Mounts))
	}
	if cfg.Mounts[0].Source != "test-volume" {
		t.Errorf("expected volume source test-volume, got %s", cfg.Mounts[0].Source)
	}
}

func TestBuildClaudeConfigProxyEnvVars(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/tmp/ws",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   9090,
	}
	cfg := container.BuildClaudeConfig(opts)

	expectedEnvs := []string{
		"HTTP_PROXY=http://airlock-proxy:9090",
		"HTTPS_PROXY=http://airlock-proxy:9090",
		"http_proxy=http://airlock-proxy:9090",
		"https_proxy=http://airlock-proxy:9090",
		"NO_PROXY=localhost,127.0.0.1",
	}

	for _, expected := range expectedEnvs {
		found := false
		for _, env := range cfg.Env {
			if env == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected env var %q not found in: %v", expected, cfg.Env)
		}
	}
}

func TestBuildClaudeConfigCommand(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/tmp/ws",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
	}
	cfg := container.BuildClaudeConfig(opts)

	if len(cfg.Cmd) != 2 {
		t.Fatalf("expected 2 cmd elements, got %d: %v", len(cfg.Cmd), cfg.Cmd)
	}
	if cfg.Cmd[0] != "claude" || cfg.Cmd[1] != "--dangerouslySkipPermissions" {
		t.Errorf("unexpected cmd: %v", cfg.Cmd)
	}
}

func TestBuildClaudeConfigSecurityDefaults(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/tmp/ws",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
	}
	cfg := container.BuildClaudeConfig(opts)

	// CapDrop must be ALL
	if len(cfg.CapDrop) != 1 || cfg.CapDrop[0] != "ALL" {
		t.Errorf("CapDrop should be [ALL], got %v", cfg.CapDrop)
	}

	// TTY and Stdin must be enabled for interactive session
	if !cfg.Tty {
		t.Error("Tty should be true")
	}
	if !cfg.Stdin {
		t.Error("Stdin should be true")
	}

	// WorkingDir
	if !strings.HasPrefix(cfg.WorkingDir, "/workspace/") {
		t.Errorf("WorkingDir should start with /workspace/, got %s", cfg.WorkingDir)
	}

	// Name
	if cfg.Name != "airlock-claude" {
		t.Errorf("Name should be airlock-claude, got %s", cfg.Name)
	}
}

func TestBuildProxyConfigEmptyPassthroughHosts(t *testing.T) {
	opts := container.RunOpts{
		ProxyImage:       "airlock-proxy:latest",
		NetworkName:      "airlock-net",
		MappingPath:      "/tmp/mapping.json",
		ProxyPort:        8080,
		PassthroughHosts: []string{},
	}
	cfg := container.BuildProxyConfig(opts)

	// With empty passthrough, env var should be set but empty
	hasPassthrough := false
	for _, env := range cfg.Env {
		if strings.HasPrefix(env, "AIRLOCK_PASSTHROUGH_HOSTS=") {
			hasPassthrough = true
			if env != "AIRLOCK_PASSTHROUGH_HOSTS=" {
				t.Errorf("expected empty passthrough value, got: %s", env)
			}
		}
	}
	if !hasPassthrough {
		t.Error("AIRLOCK_PASSTHROUGH_HOSTS env var not found")
	}
}

func TestBuildProxyConfigSecurityDefaults(t *testing.T) {
	opts := container.RunOpts{
		ProxyImage:       "airlock-proxy:latest",
		NetworkName:      "airlock-net",
		MappingPath:      "/tmp/mapping.json",
		ProxyPort:        8080,
		PassthroughHosts: []string{"api.anthropic.com"},
	}
	cfg := container.BuildProxyConfig(opts)

	// CapDrop must be ALL
	if len(cfg.CapDrop) != 1 || cfg.CapDrop[0] != "ALL" {
		t.Errorf("proxy CapDrop should be [ALL], got %v", cfg.CapDrop)
	}

	// Name must be airlock-proxy
	if cfg.Name != "airlock-proxy" {
		t.Errorf("proxy Name should be airlock-proxy, got %s", cfg.Name)
	}

	// Network must be set
	if cfg.Network != "airlock-net" {
		t.Errorf("proxy Network should be airlock-net, got %s", cfg.Network)
	}

	// Mapping bind mount must be read-only
	if len(cfg.Binds) != 1 {
		t.Fatalf("expected 1 bind mount, got %d", len(cfg.Binds))
	}
	if !strings.HasSuffix(cfg.Binds[0], ":ro") {
		t.Errorf("mapping bind mount should be read-only, got: %s", cfg.Binds[0])
	}
}

func TestBuildProxyConfigWithID(t *testing.T) {
	opts := container.RunOpts{
		ID:               "abc123",
		ProxyImage:       "airlock-proxy:latest",
		NetworkName:      "airlock-net",
		MappingPath:      "/tmp/mapping.json",
		ProxyPort:        8080,
		PassthroughHosts: []string{"api.anthropic.com"},
	}
	cfg := container.BuildProxyConfig(opts)
	if cfg.Name != "airlock-proxy-abc123" {
		t.Errorf("expected name airlock-proxy-abc123, got %s", cfg.Name)
	}
}

func TestBuildClaudeConfigWithID(t *testing.T) {
	opts := container.RunOpts{
		ID:          "abc123",
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
	}
	cfg := container.BuildClaudeConfig(opts)
	if cfg.Name != "airlock-claude-abc123" {
		t.Errorf("expected name airlock-claude-abc123, got %s", cfg.Name)
	}
	// HTTP_PROXY should reference the ID-suffixed proxy container
	expectedProxy := "http://airlock-proxy-abc123:8080"
	hasProxy := false
	for _, env := range cfg.Env {
		if env == "HTTP_PROXY="+expectedProxy {
			hasProxy = true
		}
	}
	if !hasProxy {
		t.Errorf("expected HTTP_PROXY referencing airlock-proxy-abc123, got env: %v", cfg.Env)
	}
}

func TestBuildProxyConfigWithoutID(t *testing.T) {
	opts := container.RunOpts{
		ProxyImage:       "airlock-proxy:latest",
		NetworkName:      "airlock-net",
		MappingPath:      "/tmp/mapping.json",
		ProxyPort:        8080,
		PassthroughHosts: []string{},
	}
	cfg := container.BuildProxyConfig(opts)
	if cfg.Name != "airlock-proxy" {
		t.Errorf("expected name airlock-proxy (no suffix), got %s", cfg.Name)
	}
}

func TestBuildClaudeDetachedConfig(t *testing.T) {
	opts := container.RunOpts{
		ID:          "sess1",
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
		CACertPath:  "/tmp/ca.pem",
	}
	cfg := container.BuildClaudeDetachedConfig(opts)

	// Must use keepalive entrypoint
	if len(cfg.Cmd) != 1 || cfg.Cmd[0] != "/usr/local/bin/entrypoint-keepalive.sh" {
		t.Errorf("expected keepalive entrypoint cmd, got %v", cfg.Cmd)
	}

	// TTY and Stdin must be disabled for detached mode
	if cfg.Tty {
		t.Error("detached config should have Tty=false")
	}
	if cfg.Stdin {
		t.Error("detached config should have Stdin=false")
	}

	// Name should still include ID
	if cfg.Name != "airlock-claude-sess1" {
		t.Errorf("expected name airlock-claude-sess1, got %s", cfg.Name)
	}

	// Should preserve all other fields from BuildClaudeConfig
	if cfg.Image != "airlock-claude:latest" {
		t.Errorf("expected image airlock-claude:latest, got %s", cfg.Image)
	}
	if !strings.HasPrefix(cfg.WorkingDir, "/workspace/") {
		t.Errorf("expected working dir /workspace, got %s", cfg.WorkingDir)
	}
	if len(cfg.CapDrop) != 1 || cfg.CapDrop[0] != "ALL" {
		t.Errorf("expected CapDrop=[ALL], got %v", cfg.CapDrop)
	}
}

func TestBuildClaudeDetachedConfigPreservesBinds(t *testing.T) {
	opts := container.RunOpts{
		Workspace:    "/home/user/project",
		Image:        "airlock-claude:latest",
		NetworkName:  "airlock-net",
		VolumeName:   "test-volume",
		ProxyPort:    8080,
		ShadowMounts: []secrets.ShadowMount{{HostPath: "/tmp/env.enc", ContainerPath: "/run/airlock/env.enc"}},
		CACertPath:   "/tmp/ca.pem",
	}
	cfg := container.BuildClaudeDetachedConfig(opts)

	// With VolumeName: workspace + ca-cert + 1 shadow mount = 3 binds; .claude is a volume mount
	if len(cfg.Binds) != 3 {
		t.Errorf("expected 3 bind mounts, got %d: %v", len(cfg.Binds), cfg.Binds)
	}
	if len(cfg.Mounts) != 1 {
		t.Errorf("expected 1 volume mount, got %d", len(cfg.Mounts))
	}
}

func TestBuildClaudeDetachedConfigPreservesEnv(t *testing.T) {
	opts := container.RunOpts{
		ID:          "xyz",
		Workspace:   "/tmp/ws",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   9090,
	}
	cfg := container.BuildClaudeDetachedConfig(opts)

	expectedProxy := "http://airlock-proxy-xyz:9090"
	hasProxy := false
	for _, env := range cfg.Env {
		if env == "HTTP_PROXY="+expectedProxy {
			hasProxy = true
		}
	}
	if !hasProxy {
		t.Errorf("expected HTTP_PROXY referencing airlock-proxy-xyz, got env: %v", cfg.Env)
	}
}

func TestBuildProxyConfigNoMappingPath(t *testing.T) {
	opts := container.RunOpts{
		ProxyImage:       "airlock-proxy:latest",
		NetworkName:      "airlock-net",
		MappingPath:      "",
		ProxyPort:        8080,
		PassthroughHosts: []string{"api.anthropic.com"},
	}
	cfg := container.BuildProxyConfig(opts)

	// With empty MappingPath, there should be no bind mounts
	if len(cfg.Binds) != 0 {
		t.Errorf("expected 0 bind mounts when MappingPath is empty, got %d: %v", len(cfg.Binds), cfg.Binds)
	}
}

func TestBuildClaudeConfigLocaleEnv(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/tmp/ws",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
	}
	cfg := container.BuildClaudeConfig(opts)
	found := false
	for _, env := range cfg.Env {
		if env == "LANG=C.UTF-8" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected LANG=C.UTF-8 in Env, got: %v", cfg.Env)
	}
}

func TestBuildProxyConfigMappingEnv(t *testing.T) {
	opts := container.RunOpts{
		ProxyImage:       "airlock-proxy:latest",
		NetworkName:      "airlock-net",
		MappingPath:      "/custom/path/mapping.json",
		ProxyPort:        8080,
		PassthroughHosts: []string{},
	}
	cfg := container.BuildProxyConfig(opts)

	hasMappingEnv := false
	for _, env := range cfg.Env {
		if env == "AIRLOCK_MAPPING_PATH=/run/airlock/mapping.json" {
			hasMappingEnv = true
		}
	}
	if !hasMappingEnv {
		t.Error("AIRLOCK_MAPPING_PATH env var not found or incorrect")
	}

	// Bind mount should map custom host path to container path
	if !strings.Contains(cfg.Binds[0], "/custom/path/mapping.json:/run/airlock/mapping.json:ro") {
		t.Errorf("expected custom mapping path in bind, got: %s", cfg.Binds[0])
	}
}

func TestBuildClaudeConfigEnvShadowBind(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
		ShadowMounts: []secrets.ShadowMount{
			{HostPath: "/tmp/airlock-xyz/env.enc", ContainerPath: "/run/airlock/env.enc"},
			{HostPath: "/tmp/airlock-xyz/env.enc", ContainerPath: "/workspace/.env"},
		},
	}
	cfg := container.BuildClaudeConfig(opts)

	// workspace + 2 shadow mounts = 3 binds; .claude is a volume mount
	if len(cfg.Binds) != 3 {
		t.Errorf("expected 3 binds with shadow, got %d: %v", len(cfg.Binds), cfg.Binds)
	}
	foundShadow := false
	for _, bind := range cfg.Binds {
		if strings.Contains(bind, "/workspace/.env") && strings.HasSuffix(bind, ":ro") {
			foundShadow = true
			if !strings.HasPrefix(bind, "/tmp/airlock-xyz/env.enc:") {
				t.Errorf("shadow bind should map env.enc, got: %s", bind)
			}
		}
	}
	if !foundShadow {
		t.Errorf("shadow bind not found in: %v", cfg.Binds)
	}
}

func TestBuildClaudeConfigNoShadowWhenEmpty(t *testing.T) {
	opts := container.RunOpts{
		Workspace:    "/home/user/project",
		Image:        "airlock-claude:latest",
		NetworkName:  "airlock-net",
		VolumeName:   "test-volume",
		ProxyPort:    8080,
		ShadowMounts: []secrets.ShadowMount{},
	}
	cfg := container.BuildClaudeConfig(opts)

	// workspace only = 1 bind; .claude is a volume mount
	if len(cfg.Binds) != 1 {
		t.Errorf("expected 1 bind without shadow mounts, got %d: %v", len(cfg.Binds), cfg.Binds)
	}
	for _, bind := range cfg.Binds {
		if strings.Contains(bind, "/workspace/.env") {
			t.Errorf("should not have shadow bind: %s", bind)
		}
	}
}

func TestBuildClaudeConfigEnvShadowSubdirectory(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "test-volume",
		ProxyPort:   8080,
		ShadowMounts: []secrets.ShadowMount{
			{HostPath: "/tmp/airlock-xyz/env.enc", ContainerPath: "/workspace/config/.env.production"},
		},
	}
	cfg := container.BuildClaudeConfig(opts)

	foundShadow := false
	for _, bind := range cfg.Binds {
		if strings.Contains(bind, "/workspace/config/.env.production:ro") {
			foundShadow = true
		}
	}
	if !foundShadow {
		t.Errorf("subdirectory shadow bind not found in: %v", cfg.Binds)
	}
}

func TestBuildClaudeConfigWithVolume(t *testing.T) {
	opts := container.RunOpts{
		Workspace:   "/home/user/project",
		Image:       "airlock-claude:latest",
		NetworkName: "airlock-net",
		VolumeName:  "airlock-claude-home",
		ProxyPort:   8080,
	}
	cfg := container.BuildClaudeConfig(opts)
	for _, bind := range cfg.Binds {
		if strings.Contains(bind, ".claude") {
			t.Errorf("should not have .claude bind mount when VolumeName is set, got: %s", bind)
		}
	}
	if len(cfg.Binds) != 1 {
		t.Errorf("expected 1 bind mount (workspace only), got %d: %v", len(cfg.Binds), cfg.Binds)
	}
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
