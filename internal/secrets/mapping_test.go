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
	result, err := secrets.EncryptEntries(entries, kp.PublicKey)
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
