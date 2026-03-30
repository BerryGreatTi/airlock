package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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

func Save(cfg Config, airlockDir string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	configPath := filepath.Join(airlockDir, "config.yaml")
	return os.WriteFile(configPath, data, 0644)
}

func Load(airlockDir string) (Config, error) {
	configPath := filepath.Join(airlockDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
