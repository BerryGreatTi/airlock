package config_test

import (
	"os"
	"path/filepath"
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
	if loaded.VolumeName != "" {
		t.Errorf("expected empty VolumeName for old config, got %s", loaded.VolumeName)
	}
}
