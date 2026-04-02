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
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", configPath, err)
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
