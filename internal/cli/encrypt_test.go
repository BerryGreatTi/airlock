package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestRunEncrypt(t *testing.T) {
	dir := t.TempDir()
	keysDir := filepath.Join(dir, "keys")

	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if err := crypto.SaveKeyPair(kp, keysDir); err != nil {
		t.Fatalf("SaveKeyPair: %v", err)
	}

	envPath := filepath.Join(dir, ".env")
	os.WriteFile(envPath, []byte("API_KEY=secret123\nDB_HOST=localhost\n"), 0644)

	outPath := filepath.Join(dir, ".env.enc")
	err = cli.RunEncrypt(envPath, outPath, keysDir)
	if err != nil {
		t.Fatalf("RunEncrypt failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "API_KEY='ENC[age:") {
		t.Errorf("API_KEY not encrypted:\n%s", content)
	}
	if !strings.Contains(content, "DB_HOST='ENC[age:") {
		t.Errorf("DB_HOST not encrypted:\n%s", content)
	}
}

func TestRunEncryptNoKeys(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	os.WriteFile(envPath, []byte("KEY=val\n"), 0644)

	err := cli.RunEncrypt(envPath, filepath.Join(dir, ".env.enc"), filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Error("expected error when keys don't exist")
	}
}
