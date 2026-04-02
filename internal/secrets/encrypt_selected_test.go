package secrets

import (
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestEncryptSelectedAll(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	entries := []SecretEntry{
		{Path: "DB_HOST", Value: "localhost"},
		{Path: "API_KEY", Value: "sk_live_secret123"},
	}

	result, mapping, err := EncryptSelected(entries, nil, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("EncryptSelected: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if len(mapping) != 2 {
		t.Fatalf("expected 2 mapping entries, got %d", len(mapping))
	}

	for _, e := range result {
		if !crypto.IsEncrypted(e.Value) {
			t.Errorf("entry %s not encrypted: %s", e.Path, e.Value)
		}
		plain, ok := mapping[e.Value]
		if !ok {
			t.Errorf("no mapping for %s", e.Path)
		}
		// Find original value
		for _, orig := range entries {
			if orig.Path == e.Path && plain != orig.Value {
				t.Errorf("mapping mismatch for %s: got %q, want %q", e.Path, plain, orig.Value)
			}
		}
	}
}

func TestEncryptSelectedKeysOnly(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	entries := []SecretEntry{
		{Path: "DB_HOST", Value: "localhost"},
		{Path: "API_KEY", Value: "sk_live_secret123"},
		{Path: "LOG_LEVEL", Value: "debug"},
	}

	keys := map[string]bool{"API_KEY": true}

	result, mapping, err := EncryptSelected(entries, keys, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("EncryptSelected: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	if len(mapping) != 1 {
		t.Fatalf("expected 1 mapping entry, got %d", len(mapping))
	}

	// DB_HOST should remain plaintext
	if result[0].Value != "localhost" {
		t.Errorf("DB_HOST should be plaintext, got %s", result[0].Value)
	}
	// API_KEY should be encrypted
	if !crypto.IsEncrypted(result[1].Value) {
		t.Errorf("API_KEY should be encrypted, got %s", result[1].Value)
	}
	// LOG_LEVEL should remain plaintext
	if result[2].Value != "debug" {
		t.Errorf("LOG_LEVEL should be plaintext, got %s", result[2].Value)
	}
}

func TestEncryptSelectedAlreadyEncrypted(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	// Pre-encrypt a value
	ciphertext, err := crypto.Encrypt("my_secret", kp.PublicKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	wrapped := crypto.WrapENC(ciphertext)

	entries := []SecretEntry{
		{Path: "SECRET", Value: wrapped},
	}

	result, mapping, err := EncryptSelected(entries, nil, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("EncryptSelected: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	// Should keep the same encrypted value (no double encryption)
	if result[0].Value != wrapped {
		t.Errorf("already-encrypted value should not be re-encrypted")
	}
	// Mapping should resolve to plaintext
	if mapping[wrapped] != "my_secret" {
		t.Errorf("mapping should resolve to plaintext, got %q", mapping[wrapped])
	}
}

func TestEncryptSelectedMixed(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	// Pre-encrypt one value
	ciphertext, err := crypto.Encrypt("already_secret", kp.PublicKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	wrapped := crypto.WrapENC(ciphertext)

	entries := []SecretEntry{
		{Path: "ENCRYPTED", Value: wrapped},
		{Path: "SELECTED", Value: "to_encrypt"},
		{Path: "PLAIN", Value: "keep_plain"},
	}

	keys := map[string]bool{"SELECTED": true}

	result, mapping, err := EncryptSelected(entries, keys, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("EncryptSelected: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	// 2 mapping entries: the already-encrypted one + the newly encrypted one
	if len(mapping) != 2 {
		t.Fatalf("expected 2 mapping entries, got %d", len(mapping))
	}

	// ENCRYPTED: kept as-is, in mapping
	if result[0].Value != wrapped {
		t.Errorf("ENCRYPTED should keep original value")
	}
	if mapping[wrapped] != "already_secret" {
		t.Errorf("ENCRYPTED mapping incorrect")
	}

	// SELECTED: newly encrypted
	if !crypto.IsEncrypted(result[1].Value) {
		t.Errorf("SELECTED should be encrypted")
	}
	if mapping[result[1].Value] != "to_encrypt" {
		t.Errorf("SELECTED mapping incorrect")
	}

	// PLAIN: kept as plaintext, no mapping entry
	if result[2].Value != "keep_plain" {
		t.Errorf("PLAIN should keep original value, got %s", result[2].Value)
	}
}

func TestEncryptSelectedEmpty(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	result, mapping, err := EncryptSelected(nil, nil, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("EncryptSelected: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
	if len(mapping) != 0 {
		t.Errorf("expected 0 mapping entries, got %d", len(mapping))
	}
}

func TestEncryptSelectedDoesNotMutateInput(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	entries := []SecretEntry{
		{Path: "SECRET", Value: "plaintext_value"},
	}

	originalValue := entries[0].Value

	_, _, err = EncryptSelected(entries, nil, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatalf("EncryptSelected: %v", err)
	}

	// Input slice should not be mutated
	if entries[0].Value != originalValue {
		t.Errorf("input was mutated: got %q, want %q", entries[0].Value, originalValue)
	}
}
