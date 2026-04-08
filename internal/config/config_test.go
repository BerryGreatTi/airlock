package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.Default()

	if cfg.ContainerImage != "airlock-claude:latest" {
		t.Errorf("expected default image airlock-claude:latest, got %s", cfg.ContainerImage)
	}
	if cfg.ProxyImage != "airlock-proxy:latest" {
		t.Errorf("expected default proxy image airlock-proxy:latest, got %s", cfg.ProxyImage)
	}
	if cfg.NetworkName != "airlock-net" {
		t.Errorf("expected default network airlock-net, got %s", cfg.NetworkName)
	}
	if cfg.ProxyPort != 8080 {
		t.Errorf("expected default proxy port 8080, got %d", cfg.ProxyPort)
	}
	if len(cfg.PassthroughHosts) != 0 {
		t.Errorf("expected empty passthrough hosts, got %v", cfg.PassthroughHosts)
	}
	if cfg.VolumeName != "airlock-claude-home" {
		t.Errorf("expected default volume name airlock-claude-home, got %s", cfg.VolumeName)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Default()
	cfg.ContainerImage = "custom-image:v1"

	err := config.Save(cfg, dir)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	configPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.yaml not created")
	}

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.ContainerImage != "custom-image:v1" {
		t.Errorf("expected custom-image:v1, got %s", loaded.ContainerImage)
	}
}

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()

	_, err := config.Load(dir)
	if err == nil {
		t.Error("expected error loading non-existent config")
	}
}

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
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Load applies defaults for missing fields, so VolumeName gets the default
	if loaded.VolumeName != "airlock-claude-home" {
		t.Errorf("expected default VolumeName for old config, got %s", loaded.VolumeName)
	}
}

func TestSecretFilesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.SecretFiles = []config.SecretFileConfig{
		{Path: "/path/to/.env", Format: "dotenv"},
		{Path: "/path/to/config.json", Format: "json", EncryptKeys: []string{"db/password", "api/key"}},
		{Path: "/path/to/secrets.yaml", Format: "yaml", EncryptKeys: []string{"data/password"}},
	}
	if err := config.Save(cfg, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.SecretFiles) != 3 {
		t.Fatalf("expected 3 secret files, got %d", len(loaded.SecretFiles))
	}
	if loaded.SecretFiles[0].Path != "/path/to/.env" {
		t.Errorf("expected /path/to/.env, got %s", loaded.SecretFiles[0].Path)
	}
	if loaded.SecretFiles[0].Format != "dotenv" {
		t.Errorf("expected dotenv, got %s", loaded.SecretFiles[0].Format)
	}
	if len(loaded.SecretFiles[1].EncryptKeys) != 2 {
		t.Errorf("expected 2 encrypt keys, got %d", len(loaded.SecretFiles[1].EncryptKeys))
	}
}

func TestSecretFilesBackwardsCompat(t *testing.T) {
	dir := t.TempDir()
	// Config without secret_files field (old format)
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\nvolume_name: airlock-claude-home\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SecretFiles != nil {
		t.Errorf("expected nil SecretFiles for old config, got %v", loaded.SecretFiles)
	}
}

func TestSecretFilesEmptyEncryptKeys(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.SecretFiles = []config.SecretFileConfig{
		{Path: ".env", Format: "dotenv"},
	}
	if err := config.Save(cfg, dir); err != nil {
		t.Fatal(err)
	}

	// Verify omitempty: encrypt_keys should not appear in YAML
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if contains := string(data); len(data) > 0 {
		for _, line := range splitLines(contains) {
			if line == "  encrypt_keys:" || line == "    encrypt_keys:" {
				t.Error("encrypt_keys should be omitted when empty")
			}
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func TestEnabledMCPServersRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.EnabledMCPServers = []string{"slack", "github"}
	if err := config.Save(cfg, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.EnabledMCPServers) != 2 {
		t.Fatalf("expected 2 enabled MCP servers, got %d", len(loaded.EnabledMCPServers))
	}
	if loaded.EnabledMCPServers[0] != "slack" || loaded.EnabledMCPServers[1] != "github" {
		t.Errorf("unexpected MCP server list: %v", loaded.EnabledMCPServers)
	}
}

func TestEnabledMCPServersBackwardsCompat(t *testing.T) {
	dir := t.TempDir()
	data := []byte("container_image: airlock-claude:latest\nproxy_image: airlock-proxy:latest\nnetwork_name: airlock-net\nproxy_port: 8080\nvolume_name: airlock-claude-home\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EnabledMCPServers != nil {
		t.Errorf("expected nil EnabledMCPServers for old config (no filtering), got %v", loaded.EnabledMCPServers)
	}
}

func TestEnabledMCPServersEmptyRoundTripPreserved(t *testing.T) {
	// Empty slice means "filter out all MCPs" — the security-relevant state.
	// It must round-trip through Save/Load as a non-nil empty slice, NOT
	// collapse to nil (which would mean "no filtering").
	dir := t.TempDir()
	cfg := config.Default()
	cfg.EnabledMCPServers = []string{}
	if err := config.Save(cfg, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.EnabledMCPServers == nil {
		t.Fatal("expected non-nil empty slice (filter all), got nil (no filtering) — security regression")
	}
	if len(loaded.EnabledMCPServers) != 0 {
		t.Errorf("expected empty slice, got %v", loaded.EnabledMCPServers)
	}
}

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
