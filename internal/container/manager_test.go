package container_test

import (
	"testing"

	"github.com/taeikkim92/airlock/internal/container"
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
				EnvFilePath: "/tmp/.env.enc", MappingPath: "/tmp/mapping.json",
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
		NetworkName: "airlock-net", EnvFilePath: "/tmp/.env.enc",
		ClaudeDir: "/home/user/.claude", ProxyPort: 8080, CACertPath: "/tmp/ca.pem",
	}
	cfg := container.BuildClaudeConfig(opts)
	if cfg.Image != "airlock-claude:latest" {
		t.Errorf("expected claude image, got %s", cfg.Image)
	}
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
