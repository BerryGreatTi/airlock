package secrets

import (
	"fmt"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

// EnvSecretScanner produces ScanResult.Env and Mapping entries for
// environment-variable secrets registered in config.yaml. Each entry
// is decrypted at scan time so the proxy mapping is populated, but
// the value injected into the agent container remains ciphertext.
//
// Distinct from EnvScanner, which handles whole .env files.
type EnvSecretScanner struct {
	entries []config.EnvSecretConfig
}

// NewEnvSecretScanner returns a scanner over the given env secret entries.
func NewEnvSecretScanner(entries []config.EnvSecretConfig) *EnvSecretScanner {
	return &EnvSecretScanner{entries: entries}
}

// Name returns the scanner identifier.
func (s *EnvSecretScanner) Name() string { return "env-secret" }

// Scan decrypts each entry to populate the proxy mapping and emits
// Env entries containing the original ciphertext for container injection.
func (s *EnvSecretScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	result := &ScanResult{Mapping: make(map[string]string)}
	for _, entry := range s.entries {
		if !crypto.IsEncrypted(entry.Value) {
			return nil, fmt.Errorf("env secret %q: value is not an ENC[age:...] ciphertext", entry.Name)
		}
		inner, err := crypto.UnwrapENC(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("env secret %q: unwrap: %w", entry.Name, err)
		}
		plain, err := crypto.Decrypt(inner, opts.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("env secret %q: decrypt: %w", entry.Name, err)
		}
		result.Mapping[entry.Value] = plain
		result.Env = append(result.Env, EnvVar{
			Name:  entry.Name,
			Value: entry.Value,
		})
	}
	return result, nil
}
