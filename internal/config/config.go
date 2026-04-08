package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/taeikkim92/airlock/internal/fsutil"
	"gopkg.in/yaml.v3"
)

// SecretFileConfig describes a user-registered secret file for encryption.
type SecretFileConfig struct {
	Path        string   `yaml:"path"`
	Format      string   `yaml:"format"`                 // "dotenv", "json", "yaml", "ini", "properties", "text"
	EncryptKeys []string `yaml:"encrypt_keys,omitempty"` // keys to encrypt; empty = encrypt all
}

// EnvSecretConfig is a single encrypted environment variable that
// airlock injects into the agent container as NAME=ENC[age:...].
// Value is always an ENC[age:...] ciphertext; plaintext is never
// persisted in config.yaml.
type EnvSecretConfig struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Config struct {
	ContainerImage   string             `yaml:"container_image"`
	ProxyImage       string             `yaml:"proxy_image"`
	NetworkName      string             `yaml:"network_name"`
	ProxyPort        int                `yaml:"proxy_port"`
	PassthroughHosts []string           `yaml:"passthrough_hosts"`
	VolumeName       string             `yaml:"volume_name"`
	SecretFiles      []SecretFileConfig `yaml:"secret_files,omitempty"`
	EnvSecrets       []EnvSecretConfig  `yaml:"env_secrets,omitempty"`
	// EnabledMCPServers, when non-nil, restricts the agent container to only
	// the named MCP servers. Entries from the user's settings.json that are
	// not in this list are removed before the file is shadow-mounted into the
	// container. nil = no filtering (default behavior, all MCPs enabled).
	// Empty slice = filter out all MCP servers.
	//
	// NOTE: `omitempty` is intentionally absent. yaml.v3's omitempty collapses
	// both nil and empty slices into "field absent on Load", which would
	// silently flip the security-relevant "filter all" state ([]) to
	// "no filtering" (nil) on a save/load round-trip.
	EnabledMCPServers []string `yaml:"enabled_mcp_servers"`
	// NetworkAllowlist restricts outbound HTTP/HTTPS traffic from the agent
	// container to a user-defined list of hosts, enforced in the mitmproxy
	// addon. Supported patterns: exact host (`api.stripe.com`) and suffix
	// wildcard (`*.stripe.com`, which matches subdomains but NOT the bare
	// `stripe.com`). Empty list / nil = allow all HTTP traffic (back-compat).
	// Only HTTP/HTTPS protocols are enforced; non-HTTP traffic is already
	// blocked by the internal Docker network.
	NetworkAllowlist []string `yaml:"network_allowlist,omitempty"`
}

// EnvVarNamePattern is the POSIX env var identifier pattern. Exported so
// error messages can reference the canonical rule instead of hardcoding it.
const EnvVarNamePattern = `^[A-Za-z_][A-Za-z0-9_]*$`

var envNameRegex = regexp.MustCompile(EnvVarNamePattern)

// IsValidEnvVarName reports whether name is a valid POSIX env var identifier.
// Exported so CLI validation can reuse the canonical rule instead of
// reimplementing it.
func IsValidEnvVarName(name string) bool {
	return envNameRegex.MatchString(name)
}

// validateEnvSecrets enforces invariants on cfg.EnvSecrets:
// 1. Name matches POSIX env var format ^[A-Za-z_][A-Za-z0-9_]*$
// 2. Name is not in ReservedEnvNames
// 3. Names are unique within env_secrets[]
// 4. Value starts with "ENC[age:" and ends with "]"
//
// Any violation is a hard error so the user discovers misconfigurations
// at load time, not at session start.
func validateEnvSecrets(cfg *Config) error {
	seen := make(map[string]bool, len(cfg.EnvSecrets))
	for i, es := range cfg.EnvSecrets {
		if !IsValidEnvVarName(es.Name) {
			return fmt.Errorf("env secret at index %d: invalid name %q: must match %s", i, es.Name, EnvVarNamePattern)
		}
		if ReservedEnvNames[es.Name] {
			return fmt.Errorf("env secret name %q is reserved by airlock", es.Name)
		}
		if seen[es.Name] {
			return fmt.Errorf("duplicate env secret name %q", es.Name)
		}
		seen[es.Name] = true
		if !strings.HasPrefix(es.Value, "ENC[age:") || !strings.HasSuffix(es.Value, "]") {
			return fmt.Errorf("env secret %q: value is not an ENC[age:...] ciphertext", es.Name)
		}
	}
	return nil
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
	return fsutil.AtomicWrite(configPath, data, 0o600)
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
	if err := validateEnvSecrets(&cfg); err != nil {
		return Config{}, fmt.Errorf("config load: %w", err)
	}
	return cfg, nil
}
