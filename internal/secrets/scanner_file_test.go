package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestFileScannerSingleDotenv(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workspace := t.TempDir()
	tmpDir := t.TempDir()

	envPath := filepath.Join(workspace, "secrets.env")
	if err := os.WriteFile(envPath, []byte("API_KEY=sk_live_abcdefghijk\nLOG=debug\n"), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: envPath},
	}, workspace)

	if scanner.Name() != "file" {
		t.Errorf("expected name 'file', got %q", scanner.Name())
	}

	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(result.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(result.Mounts))
	}
	if !strings.HasSuffix(result.Mounts[0].ContainerPath, "/secrets.env") {
		t.Errorf("unexpected container path: %s", result.Mounts[0].ContainerPath)
	}

	// All entries encrypted (keys=nil)
	if len(result.Mapping) != 2 {
		t.Errorf("expected 2 mapping entries (encrypt all), got %d", len(result.Mapping))
	}
}

func TestFileScannerMultipleFiles(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workspace := t.TempDir()
	tmpDir := t.TempDir()

	// Create dotenv file
	envPath := filepath.Join(workspace, ".env")
	if err := os.WriteFile(envPath, []byte("TOKEN=sk_live_abc123def456\n"), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	// Create JSON file
	jsonData := map[string]any{
		"database": map[string]any{
			"password": "super_secret_password_123",
		},
		"app": map[string]any{
			"name": "my-app",
		},
	}
	jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
	jsonPath := filepath.Join(workspace, "config.json")
	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		t.Fatalf("write json file: %v", err)
	}

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: envPath},
		{Path: jsonPath, Format: "json"},
	}, workspace)

	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(result.Mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(result.Mounts))
	}

	// dotenv: 1 entry (TOKEN) + json: 2 string entries (database.password + app.name)
	if len(result.Mapping) != 3 {
		t.Errorf("expected 3 mapping entries, got %d", len(result.Mapping))
	}
}

func TestFileScannerSelectiveEncryption(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workspace := t.TempDir()
	tmpDir := t.TempDir()

	envPath := filepath.Join(workspace, "secrets.env")
	if err := os.WriteFile(envPath, []byte("API_KEY=sk_live_secret123\nDB_HOST=localhost\nLOG=debug\n"), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: envPath, EncryptKeys: []string{"API_KEY"}},
	}, workspace)

	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Only API_KEY should be encrypted
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping entry (selective), got %d", len(result.Mapping))
	}

	// Verify the encrypted file content
	encData, err := os.ReadFile(result.Mounts[0].HostPath)
	if err != nil {
		t.Fatalf("read encrypted file: %v", err)
	}
	content := string(encData)
	if !strings.Contains(content, "ENC[age:") {
		t.Error("encrypted file should contain ENC[age:...] token")
	}
	if !strings.Contains(content, "DB_HOST='localhost'") {
		t.Error("DB_HOST should remain plaintext")
	}
	if !strings.Contains(content, "LOG='debug'") {
		t.Error("LOG should remain plaintext")
	}
}

func TestFileScannerContainsPathRegistered(t *testing.T) {
	workspace := t.TempDir()
	envPath := filepath.Join(workspace, ".env")

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: envPath},
	}, workspace)

	if !scanner.ContainsPath(envPath) {
		t.Error("ContainsPath should return true for registered file")
	}
}

func TestFileScannerContainsPathUnregistered(t *testing.T) {
	workspace := t.TempDir()
	envPath := filepath.Join(workspace, ".env")

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: envPath},
	}, workspace)

	otherPath := filepath.Join(workspace, "other.env")
	if scanner.ContainsPath(otherPath) {
		t.Error("ContainsPath should return false for unregistered file")
	}
}

func TestFileScannerEmptyFileList(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	scanner := NewFileScanner(nil, t.TempDir())

	result, err := scanner.Scan(ScanOpts{
		Workspace:  t.TempDir(),
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected 0 mapping entries, got %d", len(result.Mapping))
	}
}

func TestFileScannerMissingFile(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workspace := t.TempDir()
	missingPath := filepath.Join(workspace, "nonexistent.env")

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: missingPath},
	}, workspace)

	_, err = scanner.Scan(ScanOpts{
		Workspace:  workspace,
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFileScannerJSONSelective(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workspace := t.TempDir()
	tmpDir := t.TempDir()

	jsonData := map[string]any{
		"database": map[string]any{
			"host":     "localhost",
			"password": "super_secret_db_pass",
		},
		"api_key": "sk-ant-test-12345678",
	}
	jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
	jsonPath := filepath.Join(workspace, "config.json")
	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: jsonPath, Format: "json", EncryptKeys: []string{"database/password"}},
	}, workspace)

	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Only database.password should be encrypted
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping entry, got %d", len(result.Mapping))
	}

	// Read back the encrypted JSON and verify structure
	encData, err := os.ReadFile(result.Mounts[0].HostPath)
	if err != nil {
		t.Fatalf("read encrypted json: %v", err)
	}

	var encRoot map[string]any
	if err := json.Unmarshal(encData, &encRoot); err != nil {
		t.Fatalf("parse encrypted json: %v", err)
	}

	db, ok := encRoot["database"].(map[string]any)
	if !ok {
		t.Fatal("expected database object in output")
	}
	if host, ok := db["host"].(string); !ok || host != "localhost" {
		t.Errorf("database.host should be plaintext 'localhost', got %v", db["host"])
	}
	if pw, ok := db["password"].(string); !ok || !crypto.IsEncrypted(pw) {
		t.Errorf("database.password should be encrypted, got %v", db["password"])
	}
	if apiKey, ok := encRoot["api_key"].(string); !ok || apiKey != "sk-ant-test-12345678" {
		t.Errorf("api_key should be plaintext, got %v", encRoot["api_key"])
	}
}

func TestFileScannerOutsideWorkspace(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workspace := t.TempDir()
	externalDir := t.TempDir()
	tmpDir := t.TempDir()

	envPath := filepath.Join(externalDir, "external.env")
	if err := os.WriteFile(envPath, []byte("SECRET=sk_live_external_key\n"), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: envPath},
	}, workspace)

	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(result.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(result.Mounts))
	}
	// External files go under /run/airlock/files/
	if !strings.HasPrefix(result.Mounts[0].ContainerPath, "/run/airlock/files/") {
		t.Errorf("external file should be under /run/airlock/files/, got %s", result.Mounts[0].ContainerPath)
	}
}

func TestFileScannerAutoDetectFormat(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	workspace := t.TempDir()
	tmpDir := t.TempDir()

	// JSON file with no explicit format in config -- should auto-detect
	jsonData := map[string]any{"key": "value_for_test"}
	jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
	jsonPath := filepath.Join(workspace, "secrets.json")
	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: jsonPath}, // no Format specified
	}, workspace)

	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		HomeDir:    t.TempDir(),
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(result.Mapping) != 1 {
		t.Errorf("expected 1 mapping entry, got %d", len(result.Mapping))
	}

	// Read back and verify it's valid JSON (not dotenv)
	encData, err := os.ReadFile(result.Mounts[0].HostPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(encData, &parsed); err != nil {
		t.Errorf("output should be valid JSON: %v", err)
	}
}
