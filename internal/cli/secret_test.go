package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

// setupAirlock creates a temporary workspace with .airlock initialized.
func setupAirlock(t *testing.T) (workspace, keysDir, airlockDir string) {
	t.Helper()
	workspace = t.TempDir()
	airlockDir = filepath.Join(workspace, ".airlock")
	keysDir = filepath.Join(airlockDir, "keys")

	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := crypto.SaveKeyPair(kp, keysDir); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(config.Default(), airlockDir); err != nil {
		t.Fatal(err)
	}
	return workspace, keysDir, airlockDir
}

// --- secret add ---

func TestSecretAdd(t *testing.T) {
	workspace, _, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("KEY=value\n"), 0644)

	err := cli.RunSecretAdd(envPath, "", airlockDir)
	if err != nil {
		t.Fatalf("RunSecretAdd: %v", err)
	}

	cfg, _ := config.Load(airlockDir)
	if len(cfg.SecretFiles) != 1 {
		t.Fatalf("expected 1 secret file, got %d", len(cfg.SecretFiles))
	}
	if cfg.SecretFiles[0].Format != "dotenv" {
		t.Errorf("format = %q, want dotenv", cfg.SecretFiles[0].Format)
	}
}

func TestSecretAddWithFormatOverride(t *testing.T) {
	workspace, _, airlockDir := setupAirlock(t)

	path := filepath.Join(workspace, "secrets")
	os.WriteFile(path, []byte("KEY=value\n"), 0644)

	err := cli.RunSecretAdd(path, "dotenv", airlockDir)
	if err != nil {
		t.Fatalf("RunSecretAdd: %v", err)
	}

	cfg, _ := config.Load(airlockDir)
	if cfg.SecretFiles[0].Format != "dotenv" {
		t.Errorf("format = %q, want dotenv", cfg.SecretFiles[0].Format)
	}
}

func TestSecretAddDuplicate(t *testing.T) {
	workspace, _, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("KEY=value\n"), 0644)

	cli.RunSecretAdd(envPath, "", airlockDir)
	err := cli.RunSecretAdd(envPath, "", airlockDir)
	if err == nil {
		t.Error("expected error for duplicate add")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSecretAddMissingFile(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)

	err := cli.RunSecretAdd("/nonexistent/file.env", "", airlockDir)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSecretAddJSON(t *testing.T) {
	workspace, _, airlockDir := setupAirlock(t)

	jsonPath := filepath.Join(workspace, "config.json")
	os.WriteFile(jsonPath, []byte(`{"key": "value"}`), 0644)

	err := cli.RunSecretAdd(jsonPath, "", airlockDir)
	if err != nil {
		t.Fatalf("RunSecretAdd: %v", err)
	}

	cfg, _ := config.Load(airlockDir)
	if cfg.SecretFiles[0].Format != "json" {
		t.Errorf("format = %q, want json", cfg.SecretFiles[0].Format)
	}
}

// --- secret remove ---

func TestSecretRemove(t *testing.T) {
	workspace, _, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("KEY=value\n"), 0644)

	cli.RunSecretAdd(envPath, "", airlockDir)

	err := cli.RunSecretRemove(envPath, airlockDir)
	if err != nil {
		t.Fatalf("RunSecretRemove: %v", err)
	}

	cfg, _ := config.Load(airlockDir)
	if len(cfg.SecretFiles) != 0 {
		t.Errorf("expected 0 secret files, got %d", len(cfg.SecretFiles))
	}
}

func TestSecretRemoveNotRegistered(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)

	err := cli.RunSecretRemove("/some/random/path", airlockDir)
	if err == nil {
		t.Error("expected error for unregistered file")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- secret encrypt ---

func TestSecretEncryptAll(t *testing.T) {
	workspace, keysDir, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("API_KEY=sk_live_test123456\nHOST=localhost\n"), 0644)

	err := cli.RunSecretEncrypt(envPath, "all", keysDir, airlockDir)
	if err != nil {
		t.Fatalf("RunSecretEncrypt: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)
	if !strings.Contains(content, "ENC[age:") {
		t.Error("expected encrypted values")
	}
	// Both entries should be encrypted
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "ENC[age:") {
			t.Errorf("unencrypted line: %s", line)
		}
	}
}

func TestSecretEncryptSelectedKeys(t *testing.T) {
	workspace, keysDir, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("API_KEY=sk_live_test123456\nHOST=localhost\n"), 0644)

	err := cli.RunSecretEncrypt(envPath, "API_KEY", keysDir, airlockDir)
	if err != nil {
		t.Fatalf("RunSecretEncrypt: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)
	if !strings.Contains(content, "API_KEY='ENC[age:") {
		t.Error("API_KEY should be encrypted")
	}
	if !strings.Contains(content, "HOST='localhost'") {
		t.Error("HOST should remain plaintext")
	}
}

func TestSecretEncryptAuto(t *testing.T) {
	workspace, keysDir, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("SECRET_TOKEN=sk_live_longvalue123\nDEBUG=true_value_long_enough\n"), 0644)

	err := cli.RunSecretEncrypt(envPath, "auto", keysDir, airlockDir)
	if err != nil {
		t.Fatalf("RunSecretEncrypt: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)
	// SECRET_TOKEN should be encrypted (key contains "secret" + "token")
	if !strings.Contains(content, "SECRET_TOKEN='ENC[age:") {
		t.Error("SECRET_TOKEN should be auto-encrypted")
	}
}

func TestSecretEncryptJSON(t *testing.T) {
	workspace, keysDir, airlockDir := setupAirlock(t)

	jsonPath := filepath.Join(workspace, "config.json")
	os.WriteFile(jsonPath, []byte(`{"db":{"password":"secret123","host":"localhost"},"port":5432}`), 0644)

	err := cli.RunSecretEncrypt(jsonPath, "db/password", keysDir, airlockDir)
	if err != nil {
		t.Fatalf("RunSecretEncrypt: %v", err)
	}

	data, _ := os.ReadFile(jsonPath)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	db := parsed["db"].(map[string]interface{})
	pw := db["password"].(string)
	if !strings.HasPrefix(pw, "ENC[age:") {
		t.Errorf("password should be encrypted, got: %s", pw)
	}
	host := db["host"].(string)
	if host != "localhost" {
		t.Errorf("host should be plaintext, got: %s", host)
	}
	// Non-string values preserved
	port, ok := parsed["port"].(float64)
	if !ok || port != 5432 {
		t.Errorf("port should be preserved as number 5432, got: %v", parsed["port"])
	}
}

func TestSecretEncryptNoKeys(t *testing.T) {
	workspace, _, _ := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("KEY=val\n"), 0644)

	err := cli.RunSecretEncrypt(envPath, "all", "/nonexistent/keys", "")
	if err == nil {
		t.Error("expected error when keys don't exist")
	}
}

// --- secret decrypt ---

func TestSecretDecryptAll(t *testing.T) {
	workspace, keysDir, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("API_KEY=sk_live_test123456\nHOST=myhost\n"), 0644)

	// Encrypt first
	cli.RunSecretEncrypt(envPath, "all", keysDir, airlockDir)

	// Decrypt
	err := cli.RunSecretDecrypt(envPath, "all", "", keysDir)
	if err != nil {
		t.Fatalf("RunSecretDecrypt: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)
	if strings.Contains(content, "ENC[age:") {
		t.Error("should not contain encrypted values after decrypt")
	}
	if !strings.Contains(content, "sk_live_test123456") {
		t.Error("API_KEY should be decrypted back to original")
	}
}

func TestSecretDecryptSelectedKeys(t *testing.T) {
	workspace, keysDir, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("A=secret_a_12345678\nB=secret_b_12345678\n"), 0644)

	cli.RunSecretEncrypt(envPath, "all", keysDir, airlockDir)

	// Decrypt only A
	err := cli.RunSecretDecrypt(envPath, "A", "", keysDir)
	if err != nil {
		t.Fatalf("RunSecretDecrypt: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)
	if !strings.Contains(content, "A='secret_a_12345678'") {
		t.Error("A should be decrypted")
	}
	if !strings.Contains(content, "B='ENC[age:") {
		t.Error("B should remain encrypted")
	}
}

func TestSecretDecryptWithFormatOverride(t *testing.T) {
	workspace, keysDir, _ := setupAirlock(t)

	// File with .ini extension containing a secret
	path := filepath.Join(workspace, "creds.ini")
	os.WriteFile(path, []byte("[default]\naws_secret=longvalue12345678\n"), 0644)

	cli.RunSecretEncrypt(path, "all", keysDir, "")

	// Verify encrypted
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "ENC[age:") {
		t.Fatal("should be encrypted")
	}

	// Decrypt with explicit format override
	err := cli.RunSecretDecrypt(path, "all", "ini", keysDir)
	if err != nil {
		t.Fatalf("RunSecretDecrypt: %v", err)
	}

	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "longvalue12345678") {
		t.Error("should decrypt back to original value")
	}
}

func TestSecretDecryptNoKeys(t *testing.T) {
	workspace, _, _ := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("KEY=val\n"), 0644)

	err := cli.RunSecretDecrypt(envPath, "all", "", "/nonexistent/keys")
	if err == nil {
		t.Error("expected error when keys don't exist")
	}
}

// --- encrypt/decrypt round-trip ---

func TestSecretEncryptDecryptRoundTrip(t *testing.T) {
	workspace, keysDir, airlockDir := setupAirlock(t)

	envPath := filepath.Join(workspace, ".env")
	original := "TOKEN=sk_live_abc123def456\nDATABASE_URL=postgres://localhost/mydb\n"
	os.WriteFile(envPath, []byte(original), 0644)

	if err := cli.RunSecretEncrypt(envPath, "all", keysDir, airlockDir); err != nil {
		t.Fatal(err)
	}

	// Verify encrypted
	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "ENC[age:") {
		t.Fatal("should be encrypted")
	}

	if err := cli.RunSecretDecrypt(envPath, "all", "", keysDir); err != nil {
		t.Fatal(err)
	}

	// Verify decrypted matches original values
	data, _ = os.ReadFile(envPath)
	if !strings.Contains(string(data), "sk_live_abc123def456") {
		t.Error("TOKEN value should match original")
	}
	if !strings.Contains(string(data), "postgres://localhost/mydb") {
		t.Error("DATABASE_URL value should match original")
	}
}
