package secrets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/taeikkim92/airlock/internal/secrets"
)

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# Database\nDB_HOST=localhost\nDB_PASS=supersecret\n\n# API\nSTRIPE_KEY=sk_live_abc123\n# Comment line\nEMPTY_VAL=\n"
	os.WriteFile(envPath, []byte(content), 0644)

	entries, err := secrets.ParseEnvFile(envPath)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Key != "DB_HOST" || entries[0].Value != "localhost" {
		t.Errorf("unexpected entry 0: %+v", entries[0])
	}
	if entries[2].Key != "STRIPE_KEY" || entries[2].Value != "sk_live_abc123" {
		t.Errorf("unexpected entry 2: %+v", entries[2])
	}
}

func TestWriteEnvFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, ".env.enc")
	entries := []secrets.EnvEntry{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "STRIPE_KEY", Value: "ENC[age:encrypted_data]"},
	}
	err := secrets.WriteEnvFile(outPath, entries)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	data, _ := os.ReadFile(outPath)
	content := string(data)
	if content != "DB_HOST='localhost'\nSTRIPE_KEY='ENC[age:encrypted_data]'\n" {
		t.Errorf("unexpected output:\n%s", content)
	}
}

func TestParseEnvFileNotExist(t *testing.T) {
	_, err := secrets.ParseEnvFile("/nonexistent/.env")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
