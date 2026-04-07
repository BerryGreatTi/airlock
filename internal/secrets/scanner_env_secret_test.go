package secrets_test

import (
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/secrets"
)

func newKeyPairForTest(t *testing.T) crypto.KeyPair {
	t.Helper()
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return kp
}

func encryptForTest(t *testing.T, plaintext string, kp crypto.KeyPair) string {
	t.Helper()
	ct, err := crypto.Encrypt(plaintext, kp.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return crypto.WrapENC(ct)
}

func TestEnvSecretScannerEmptyEntries(t *testing.T) {
	scanner := secrets.NewEnvSecretScanner(nil)
	result, err := scanner.Scan(secrets.ScanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Env) != 0 {
		t.Errorf("expected empty Env, got %d", len(result.Env))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected empty Mapping, got %d", len(result.Mapping))
	}
}

func TestEnvSecretScannerOneEntryRoundTrip(t *testing.T) {
	kp := newKeyPairForTest(t)
	wrapped := encryptForTest(t, "ghp_secret_value", kp)
	scanner := secrets.NewEnvSecretScanner([]config.EnvSecretConfig{
		{Name: "GITHUB_TOKEN", Value: wrapped},
	})
	result, err := scanner.Scan(secrets.ScanOpts{
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Env) != 1 {
		t.Fatalf("expected 1 Env entry, got %d", len(result.Env))
	}
	if result.Env[0].Name != "GITHUB_TOKEN" {
		t.Errorf("name = %q", result.Env[0].Name)
	}
	if !crypto.IsEncrypted(result.Env[0].Value) {
		t.Errorf("Env[0].Value is not encrypted: %q", result.Env[0].Value)
	}
	if result.Env[0].Value != wrapped {
		t.Errorf("Env[0].Value = %q, want %q (must be ciphertext, not plaintext)", result.Env[0].Value, wrapped)
	}
	if result.Mapping[wrapped] != "ghp_secret_value" {
		t.Errorf("mapping[ciphertext] = %q, want ghp_secret_value", result.Mapping[wrapped])
	}
}

func TestEnvSecretScannerWrongKeyFails(t *testing.T) {
	encryptKey := newKeyPairForTest(t)
	wrongKey := newKeyPairForTest(t)
	wrapped := encryptForTest(t, "secret", encryptKey)
	scanner := secrets.NewEnvSecretScanner([]config.EnvSecretConfig{
		{Name: "GITHUB_TOKEN", Value: wrapped},
	})
	_, err := scanner.Scan(secrets.ScanOpts{
		PublicKey:  wrongKey.PublicKey,
		PrivateKey: wrongKey.PrivateKey,
	})
	if err == nil {
		t.Fatal("expected decrypt error with wrong key")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("error %q does not include entry name", err.Error())
	}
}

func TestEnvSecretScannerNonEncryptedValueFails(t *testing.T) {
	kp := newKeyPairForTest(t)
	scanner := secrets.NewEnvSecretScanner([]config.EnvSecretConfig{
		{Name: "GITHUB_TOKEN", Value: "plaintext"},
	})
	_, err := scanner.Scan(secrets.ScanOpts{
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
	})
	if err == nil {
		t.Fatal("expected error for non-encrypted value")
	}
}
