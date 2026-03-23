package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
)

func TestRunInit(t *testing.T) {
	dir := t.TempDir()
	airlockDir := filepath.Join(dir, ".airlock")

	err := cli.RunInit(airlockDir)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	keysDir := filepath.Join(airlockDir, "keys")
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		t.Error("keys directory not created")
	}

	configPath := filepath.Join(airlockDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config.yaml not created")
	}

	privKey := filepath.Join(keysDir, "age.key")
	info, err := os.Stat(privKey)
	if err != nil {
		t.Fatalf("stat age.key: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("private key should be 0600, got %o", info.Mode().Perm())
	}
}

func TestRunInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	airlockDir := filepath.Join(dir, ".airlock")
	os.MkdirAll(airlockDir, 0755)

	err := cli.RunInit(airlockDir)
	if err == nil {
		t.Error("expected error when .airlock already exists")
	}
}
