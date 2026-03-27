package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taeikkim92/airlock/internal/crypto"
)

// EncryptResult holds the encrypted environment entries and
// a mapping from each ENC[age:...] token to its plaintext value.
type EncryptResult struct {
	Encrypted []EnvEntry
	Mapping   map[string]string // ENC[age:...] -> plaintext
}

// EncryptEntries encrypts each entry's value using the given age public key,
// wraps it in the ENC[age:...] pattern, and builds a reverse mapping
// for the proxy to use at decryption time.
func EncryptEntries(entries []EnvEntry, publicKey, privateKey string) (EncryptResult, error) {
	result := EncryptResult{
		Encrypted: make([]EnvEntry, 0, len(entries)),
		Mapping:   make(map[string]string, len(entries)),
	}

	for _, entry := range entries {
		if crypto.IsEncrypted(entry.Value) {
			// Already encrypted: keep as-is and build mapping by decrypting
			inner, err := crypto.UnwrapENC(entry.Value)
			if err != nil {
				return EncryptResult{}, fmt.Errorf("unwrap %s: %w", entry.Key, err)
			}
			plain, err := crypto.Decrypt(inner, privateKey)
			if err != nil {
				return EncryptResult{}, fmt.Errorf("decrypt %s for mapping: %w", entry.Key, err)
			}
			result.Encrypted = append(result.Encrypted, EnvEntry{Key: entry.Key, Value: entry.Value})
			result.Mapping[entry.Value] = plain
			continue
		}

		ciphertext, err := crypto.Encrypt(entry.Value, publicKey)
		if err != nil {
			return EncryptResult{}, fmt.Errorf("encrypt %s: %w", entry.Key, err)
		}

		wrapped := crypto.WrapENC(ciphertext)
		result.Encrypted = append(result.Encrypted, EnvEntry{Key: entry.Key, Value: wrapped})
		result.Mapping[wrapped] = entry.Value
	}

	return result, nil
}

// SaveMapping writes the encrypted-to-plaintext mapping as JSON to the
// given directory. The file is created with 0600 permissions to protect
// the plaintext secrets it contains.
func SaveMapping(mapping map[string]string, dir string) (string, error) {
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal mapping: %w", err)
	}

	path := filepath.Join(dir, "mapping.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("write mapping: %w", err)
	}

	return path, nil
}
