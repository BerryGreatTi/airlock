package crypto_test

import (
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	if kp.PublicKey == "" {
		t.Error("public key is empty")
	}
	if kp.PrivateKey == "" {
		t.Error("private key is empty")
	}

	if kp.PublicKey[:4] != "age1" {
		t.Errorf("public key should start with age1, got %s", kp.PublicKey[:4])
	}

	if len(kp.PrivateKey) < 16 || kp.PrivateKey[:16] != "AGE-SECRET-KEY-1" {
		t.Errorf("private key should start with AGE-SECRET-KEY-1")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	plaintext := "sk_live_abc123_very_secret_key"

	ciphertext, err := crypto.Encrypt(plaintext, kp.PublicKey)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := crypto.Decrypt(ciphertext, kp.PrivateKey)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	kp1, _ := crypto.GenerateKeyPair()
	kp2, _ := crypto.GenerateKeyPair()

	ciphertext, _ := crypto.Encrypt("secret", kp1.PublicKey)

	_, err := crypto.Decrypt(ciphertext, kp2.PrivateKey)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestSaveAndLoadKeyPair(t *testing.T) {
	dir := t.TempDir()

	kp, _ := crypto.GenerateKeyPair()

	err := crypto.SaveKeyPair(kp, dir)
	if err != nil {
		t.Fatalf("failed to save key pair: %v", err)
	}

	loaded, err := crypto.LoadKeyPair(dir)
	if err != nil {
		t.Fatalf("failed to load key pair: %v", err)
	}

	if loaded.PublicKey != kp.PublicKey {
		t.Error("public keys don't match")
	}
	if loaded.PrivateKey != kp.PrivateKey {
		t.Error("private keys don't match")
	}
}
