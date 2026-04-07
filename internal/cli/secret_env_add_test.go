package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestSecretEnvAddPlaintext(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)

	err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "ghp_secret", false, airlockDir)
	if err != nil {
		t.Fatalf("RunSecretEnvAdd: %v", err)
	}

	cfg, err := config.Load(airlockDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.EnvSecrets) != 1 {
		t.Fatalf("expected 1 env secret, got %d", len(cfg.EnvSecrets))
	}
	if cfg.EnvSecrets[0].Name != "GITHUB_TOKEN" {
		t.Errorf("name = %q", cfg.EnvSecrets[0].Name)
	}
	if !crypto.IsEncrypted(cfg.EnvSecrets[0].Value) {
		t.Errorf("value not encrypted: %q", cfg.EnvSecrets[0].Value)
	}

	// Plaintext must NOT appear in the on-disk config.
	data, err := os.ReadFile(filepath.Join(airlockDir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ghp_secret") {
		t.Error("config.yaml contains plaintext value")
	}
	if !strings.Contains(string(data), "ENC[age:") {
		t.Error("config.yaml does not contain ENC[age: marker")
	}
}

func TestSecretEnvAddRoundTripsThroughDecrypt(t *testing.T) {
	_, keysDir, airlockDir := setupAirlock(t)

	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "ghp_round_trip", false, airlockDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(airlockDir)
	if err != nil {
		t.Fatal(err)
	}
	kp, err := crypto.LoadKeyPair(keysDir)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := crypto.UnwrapENC(cfg.EnvSecrets[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := crypto.Decrypt(inner, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "ghp_round_trip" {
		t.Errorf("decrypted = %q", plain)
	}
}

func TestSecretEnvAddDuplicateWithoutForce(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v1", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v2", false, airlockDir)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got %v", err)
	}
	cfg, _ := config.Load(airlockDir)
	if len(cfg.EnvSecrets) != 1 {
		t.Errorf("expected 1 entry after failed dup, got %d", len(cfg.EnvSecrets))
	}
}

func TestSecretEnvAddDuplicateWithForce(t *testing.T) {
	_, keysDir, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v1", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v2", true, airlockDir); err != nil {
		t.Fatalf("force add: %v", err)
	}
	cfg, _ := config.Load(airlockDir)
	if len(cfg.EnvSecrets) != 1 {
		t.Errorf("expected 1 entry after force overwrite, got %d", len(cfg.EnvSecrets))
	}
	kp, _ := crypto.LoadKeyPair(keysDir)
	inner, _ := crypto.UnwrapENC(cfg.EnvSecrets[0].Value)
	plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
	if plain != "v2" {
		t.Errorf("force overwrite failed: decrypted = %q", plain)
	}
}

func TestSecretEnvAddInvalidName(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	cases := []string{"1FOO", "FOO-BAR", "PATH=x", ""}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			err := cli.RunSecretEnvAdd(name, "v", false, airlockDir)
			if err == nil {
				t.Errorf("expected error for name %q", name)
			}
		})
	}
}

func TestSecretEnvAddReservedName(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	err := cli.RunSecretEnvAdd("HTTP_PROXY", "http://evil", false, airlockDir)
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected 'reserved' error, got %v", err)
	}
}

func TestSecretEnvAddPreWrappedValue(t *testing.T) {
	_, keysDir, airlockDir := setupAirlock(t)
	kp, _ := crypto.LoadKeyPair(keysDir)
	ct, _ := crypto.Encrypt("already_encrypted", kp.PublicKey)
	wrapped := crypto.WrapENC(ct)

	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", wrapped, false, airlockDir); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(airlockDir)
	if cfg.EnvSecrets[0].Value != wrapped {
		t.Errorf("pre-wrapped value was double-wrapped or modified")
	}
}

func TestSecretEnvAddMissingKeys(t *testing.T) {
	dir := t.TempDir()
	airlockDir := filepath.Join(dir, ".airlock")
	if err := os.MkdirAll(airlockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(config.Default(), airlockDir); err != nil {
		t.Fatal(err)
	}
	// no keys saved
	err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "v", false, airlockDir)
	if err == nil || !strings.Contains(err.Error(), "init") {
		t.Fatalf("expected 'init' error, got %v", err)
	}
}
