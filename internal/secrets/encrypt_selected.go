package secrets

import (
	"fmt"

	"github.com/taeikkim92/airlock/internal/crypto"
)

// EncryptSelected encrypts only entries whose Path is in the keys set.
// If keys is nil, encrypts all entries (backward compat for dotenv).
// Returns modified entries (new slice -- input is not mutated) and a
// mapping from ENC[age:...] tokens to their plaintext values.
func EncryptSelected(entries []SecretEntry, keys map[string]bool, pubKey, privKey string) ([]SecretEntry, map[string]string, error) {
	result := make([]SecretEntry, 0, len(entries))
	mapping := make(map[string]string)

	for _, entry := range entries {
		if crypto.IsEncrypted(entry.Value) {
			// Already encrypted: keep as-is but build mapping by decrypting.
			inner, err := crypto.UnwrapENC(entry.Value)
			if err != nil {
				return nil, nil, fmt.Errorf("unwrap %s: %w", entry.Path, err)
			}
			plain, err := crypto.Decrypt(inner, privKey)
			if err != nil {
				return nil, nil, fmt.Errorf("decrypt %s for mapping: %w", entry.Path, err)
			}
			result = append(result, SecretEntry{Path: entry.Path, Value: entry.Value})
			mapping[entry.Value] = plain
			continue
		}

		shouldEncrypt := keys == nil || keys[entry.Path]
		if !shouldEncrypt {
			// Keep plaintext, don't add to mapping.
			result = append(result, SecretEntry{Path: entry.Path, Value: entry.Value})
			continue
		}

		ciphertext, err := crypto.Encrypt(entry.Value, pubKey)
		if err != nil {
			return nil, nil, fmt.Errorf("encrypt %s: %w", entry.Path, err)
		}

		wrapped := crypto.WrapENC(ciphertext)
		result = append(result, SecretEntry{Path: entry.Path, Value: wrapped})
		mapping[wrapped] = entry.Value
	}

	return result, mapping, nil
}
