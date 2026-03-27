package secrets_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/secrets"
)

func TestEncryptEnvEntries(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	entries := []secrets.EnvEntry{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "API_KEY", Value: "sk_live_secret"},
	}
	result, err := secrets.EncryptEntries(entries, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("encrypt entries failed: %v", err)
	}
	if len(result.Encrypted) != 2 {
		t.Fatalf("expected 2 encrypted entries, got %d", len(result.Encrypted))
	}
	for _, e := range result.Encrypted {
		if !crypto.IsEncrypted(e.Value) {
			t.Errorf("entry %s not encrypted: %s", e.Key, e.Value)
		}
	}
	if len(result.Mapping) != 2 {
		t.Fatalf("expected 2 mapping entries, got %d", len(result.Mapping))
	}
	for _, orig := range entries {
		for _, enc := range result.Encrypted {
			if enc.Key == orig.Key {
				plaintext, ok := result.Mapping[enc.Value]
				if !ok {
					t.Errorf("no mapping for %s", enc.Value)
				}
				if plaintext != orig.Value {
					t.Errorf("mapping mismatch for %s", orig.Key)
				}
			}
		}
	}
}

func TestEncryptEntriesSkipsAlreadyEncrypted(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()

	// First encrypt plaintext entries
	entries := []secrets.EnvEntry{
		{Key: "SECRET", Value: "my_secret_value"},
	}
	first, err := secrets.EncryptEntries(entries, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}

	// Now pass the already-encrypted entries through again
	second, err := secrets.EncryptEntries(first.Encrypted, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	// Should not double-encrypt: the ENC token should be identical
	if second.Encrypted[0].Value != first.Encrypted[0].Value {
		t.Errorf("double encryption detected:\n  first:  %s\n  second: %s",
			first.Encrypted[0].Value, second.Encrypted[0].Value)
	}

	// Mapping should still resolve to plaintext
	if second.Mapping[second.Encrypted[0].Value] != "my_secret_value" {
		t.Errorf("mapping should resolve to plaintext, got: %s",
			second.Mapping[second.Encrypted[0].Value])
	}
}

func TestEncryptEntriesPreEncryptedRoundTrip(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()

	// Simulate "Encrypt All": encrypt plaintext
	plain := []secrets.EnvEntry{{Key: "KEY", Value: "my_secret"}}
	first, err := secrets.EncryptEntries(plain, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}
	t.Logf("First encrypted value (len=%d): %s", len(first.Encrypted[0].Value), first.Encrypted[0].Value[:60])
	t.Logf("First mapping value: %s", first.Mapping[first.Encrypted[0].Value])

	// Simulate "Activate": pass already-encrypted entries
	second, err := secrets.EncryptEntries(first.Encrypted, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}
	t.Logf("Second encrypted value (len=%d): %s", len(second.Encrypted[0].Value), second.Encrypted[0].Value[:60])
	t.Logf("Second mapping value: %s", second.Mapping[second.Encrypted[0].Value])

	// Verify no double encryption
	if first.Encrypted[0].Value != second.Encrypted[0].Value {
		t.Errorf("double encryption! first len=%d, second len=%d",
			len(first.Encrypted[0].Value), len(second.Encrypted[0].Value))
	}
	if second.Mapping[second.Encrypted[0].Value] != "my_secret" {
		t.Errorf("mapping should be plaintext, got: %s", second.Mapping[second.Encrypted[0].Value])
	}
}

func TestSaveMapping(t *testing.T) {
	dir := t.TempDir()
	mapping := map[string]string{"ENC[age:abc]": "secret1", "ENC[age:def]": "secret2"}
	path, err := secrets.SaveMapping(mapping, dir)
	if err != nil {
		t.Fatalf("save mapping failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	var loaded map[string]string
	json.Unmarshal(data, &loaded)
	if loaded["ENC[age:abc]"] != "secret1" {
		t.Error("mapping not preserved")
	}
}

func TestSaveMappingPermissions(t *testing.T) {
	dir := t.TempDir()
	mapping := map[string]string{"k": "v"}
	path, _ := secrets.SaveMapping(mapping, dir)
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}
