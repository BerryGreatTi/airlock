package crypto

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

// KeyPair holds an age X25519 public/private key pair.
type KeyPair struct {
	PublicKey  string
	PrivateKey string
}

// GenerateKeyPair creates a new age X25519 identity and returns
// the public and private keys as strings.
func GenerateKeyPair() (KeyPair, error) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate identity: %w", err)
	}

	return KeyPair{
		PublicKey:  identity.Recipient().String(),
		PrivateKey: identity.String(),
	}, nil
}

// Encrypt encrypts plaintext using the given age public key and returns
// the ciphertext as a base64-encoded string.
func Encrypt(plaintext string, publicKey string) (string, error) {
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		return "", fmt.Errorf("parse recipient: %w", err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return "", fmt.Errorf("create encryptor: %w", err)
	}

	if _, err := io.WriteString(w, plaintext); err != nil {
		return "", fmt.Errorf("write plaintext: %w", err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close encryptor: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Decrypt decodes a base64-encoded ciphertext and decrypts it using
// the given age private key, returning the original plaintext.
func Decrypt(ciphertext string, privateKey string) (string, error) {
	identity, err := age.ParseX25519Identity(privateKey)
	if err != nil {
		return "", fmt.Errorf("parse identity: %w", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	r, err := age.Decrypt(bytes.NewReader(raw), identity)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	plainBytes, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read plaintext: %w", err)
	}

	return string(plainBytes), nil
}

// SaveKeyPair writes the key pair to keysDir as age.key (private)
// and age.pub (public). The private key file includes the public key
// as a comment, following the standard age key file format.
func SaveKeyPair(kp KeyPair, keysDir string) error {
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("create keys dir: %w", err)
	}

	privPath := filepath.Join(keysDir, "age.key")
	content := fmt.Sprintf("# public key: %s\n%s\n", kp.PublicKey, kp.PrivateKey)
	if err := os.WriteFile(privPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}

	pubPath := filepath.Join(keysDir, "age.pub")
	if err := os.WriteFile(pubPath, []byte(kp.PublicKey+"\n"), 0644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	return nil
}

// LoadKeyPair reads a key pair from the age.key file in keysDir.
// It expects the standard age key file format with the public key
// in a comment line and the private key on its own line.
func LoadKeyPair(keysDir string) (KeyPair, error) {
	privPath := filepath.Join(keysDir, "age.key")
	data, err := os.ReadFile(privPath)
	if err != nil {
		return KeyPair{}, fmt.Errorf("read private key: %w", err)
	}

	var kp KeyPair
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# public key: ") {
			kp.PublicKey = strings.TrimPrefix(line, "# public key: ")
		} else if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
			kp.PrivateKey = line
		}
	}

	if kp.PublicKey == "" || kp.PrivateKey == "" {
		return KeyPair{}, fmt.Errorf("invalid key file: missing public or private key")
	}

	return kp, nil
}
