package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SecretFileConfig describes a user-registered secret file for encryption.
type SecretFileConfig struct {
	Path        string   `yaml:"path"`
	Format      string   `yaml:"format"`               // "dotenv", "json", "yaml", "ini", "properties", "text"
	EncryptKeys []string `yaml:"encrypt_keys,omitempty"` // keys to encrypt; empty = encrypt all
}

type Config struct {
	ContainerImage   string             `yaml:"container_image"`
	ProxyImage       string             `yaml:"proxy_image"`
	NetworkName      string             `yaml:"network_name"`
	ProxyPort        int                `yaml:"proxy_port"`
	PassthroughHosts []string           `yaml:"passthrough_hosts"`
	VolumeName       string             `yaml:"volume_name"`
	SecretFiles      []SecretFileConfig `yaml:"secret_files,omitempty"`
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

func Save(cfg Config, airlockDir string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	configPath := filepath.Join(airlockDir, "config.yaml")
	return atomicWrite(configPath, data, 0o600)
}

// atomicWrite writes data to a file atomically by writing to a temp file
// in the same directory and then renaming.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".airlock-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

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
	return cfg, nil
}
