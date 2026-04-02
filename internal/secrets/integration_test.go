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

// TestIntegrationDotenvPipeline tests the full pipeline for dotenv files:
// create -> parse -> encrypt -> scan -> decrypt -> verify round-trip.
func TestIntegrationDotenvPipeline(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	tmpDir := t.TempDir()

	// Create sample file
	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("API_KEY=sk_live_test_12345678\nDB_HOST=localhost\n"), 0644)

	// Parse
	parser := ParserFor(FormatDotenv)
	entries, err := parser.Parse(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Encrypt selected
	keys := map[string]bool{"API_KEY": true}
	encrypted, mapping, err := EncryptSelected(entries, keys, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapping) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mapping))
	}

	// Write encrypted
	if err := parser.Write(envPath, encrypted); err != nil {
		t.Fatal(err)
	}

	// Scan via FileScanner
	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: envPath, EncryptKeys: []string{"API_KEY"}},
	}, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(result.Mapping))
	}

	// Decrypt
	encEntries, _ := parser.Parse(envPath)
	decrypted := make([]SecretEntry, len(encEntries))
	for i, e := range encEntries {
		if crypto.IsEncrypted(e.Value) {
			inner, _ := crypto.UnwrapENC(e.Value)
			plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
			decrypted[i] = SecretEntry{Path: e.Path, Value: plain}
		} else {
			decrypted[i] = e
		}
	}
	parser.Write(envPath, decrypted)

	// Verify round-trip
	final, _ := parser.Parse(envPath)
	found := map[string]string{}
	for _, e := range final {
		found[e.Path] = e.Value
	}
	if found["API_KEY"] != "sk_live_test_12345678" {
		t.Errorf("round-trip failed: API_KEY = %q", found["API_KEY"])
	}
	if found["DB_HOST"] != "localhost" {
		t.Errorf("round-trip failed: DB_HOST = %q", found["DB_HOST"])
	}
}

func TestIntegrationJSONPipeline(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	tmpDir := t.TempDir()

	jsonData := map[string]interface{}{
		"database": map[string]interface{}{
			"host":     "db.example.com",
			"password": "super_secret_password",
		},
		"app_name": "my-app",
	}
	data, _ := json.MarshalIndent(jsonData, "", "  ")
	jsonPath := filepath.Join(workspace, "config.json")
	os.WriteFile(jsonPath, data, 0644)

	parser := ParserFor(FormatJSON)
	entries, err := parser.Parse(jsonPath)
	if err != nil {
		t.Fatal(err)
	}

	keys := map[string]bool{"database/password": true}
	encrypted, mapping, err := EncryptSelected(entries, keys, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapping) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mapping))
	}

	parser.Write(jsonPath, encrypted)

	// Verify encrypted file is valid JSON with ENC token
	encData, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(encData), "ENC[age:") {
		t.Error("encrypted file should contain ENC token")
	}

	// Scan
	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: jsonPath, Format: "json", EncryptKeys: []string{"database/password"}},
	}, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace:  workspace,
		PublicKey:  kp.PublicKey,
		PrivateKey: kp.PrivateKey,
		TmpDir:     tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(result.Mapping))
	}

	// Decrypt and verify round-trip
	encEntries, _ := parser.Parse(jsonPath)
	decrypted := make([]SecretEntry, len(encEntries))
	for i, e := range encEntries {
		if crypto.IsEncrypted(e.Value) {
			inner, _ := crypto.UnwrapENC(e.Value)
			plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
			decrypted[i] = SecretEntry{Path: e.Path, Value: plain}
		} else {
			decrypted[i] = e
		}
	}
	parser.Write(jsonPath, decrypted)

	final, _ := parser.Parse(jsonPath)
	found := map[string]string{}
	for _, e := range final {
		found[e.Path] = e.Value
	}
	if found["database/password"] != "super_secret_password" {
		t.Errorf("JSON round-trip failed: database/password = %q", found["database/password"])
	}
}

func TestIntegrationYAMLPipeline(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()

	yamlContent := "# Database config\ndb:\n  host: localhost\n  password: yaml_secret_123\napp: myapp\n"
	yamlPath := filepath.Join(workspace, "secrets.yaml")
	os.WriteFile(yamlPath, []byte(yamlContent), 0644)

	parser := ParserFor(FormatYAML)
	entries, err := parser.Parse(yamlPath)
	if err != nil {
		t.Fatal(err)
	}

	keys := map[string]bool{"db/password": true}
	encrypted, _, err := EncryptSelected(entries, keys, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	parser.Write(yamlPath, encrypted)

	data, _ := os.ReadFile(yamlPath)
	if !strings.Contains(string(data), "ENC[age:") {
		t.Error("YAML should contain ENC token")
	}

	// Decrypt round-trip
	encEntries, _ := parser.Parse(yamlPath)
	decrypted := make([]SecretEntry, len(encEntries))
	for i, e := range encEntries {
		if crypto.IsEncrypted(e.Value) {
			inner, _ := crypto.UnwrapENC(e.Value)
			plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
			decrypted[i] = SecretEntry{Path: e.Path, Value: plain}
		} else {
			decrypted[i] = e
		}
	}
	parser.Write(yamlPath, decrypted)

	final, _ := parser.Parse(yamlPath)
	found := map[string]string{}
	for _, e := range final {
		found[e.Path] = e.Value
	}
	if found["db/password"] != "yaml_secret_123" {
		t.Errorf("YAML round-trip failed: db/password = %q", found["db/password"])
	}
}

func TestIntegrationINIPipeline(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	tmpDir := t.TempDir()

	iniContent := "[default]\naws_access_key_id = AKIAIOSFODNN7EXAMPLE\naws_secret_access_key = wJalrXUtnFEMI_K7MDENG\n"
	iniPath := filepath.Join(workspace, "credentials.ini")
	os.WriteFile(iniPath, []byte(iniContent), 0644)

	parser := ParserFor(FormatINI)
	entries, err := parser.Parse(iniPath)
	if err != nil {
		t.Fatal(err)
	}

	keys := map[string]bool{"default/aws_secret_access_key": true}
	encrypted, _, err := EncryptSelected(entries, keys, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	parser.Write(iniPath, encrypted)

	data, _ := os.ReadFile(iniPath)
	if !strings.Contains(string(data), "ENC[age:") {
		t.Error("INI should contain ENC token")
	}

	// Scan via FileScanner
	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: iniPath, Format: "ini", EncryptKeys: []string{"default/aws_secret_access_key"}},
	}, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, PublicKey: kp.PublicKey, PrivateKey: kp.PrivateKey, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(result.Mapping))
	}

	// Decrypt round-trip
	encEntries, _ := parser.Parse(iniPath)
	decrypted := make([]SecretEntry, len(encEntries))
	for i, e := range encEntries {
		if crypto.IsEncrypted(e.Value) {
			inner, _ := crypto.UnwrapENC(e.Value)
			plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
			decrypted[i] = SecretEntry{Path: e.Path, Value: plain}
		} else {
			decrypted[i] = e
		}
	}
	parser.Write(iniPath, decrypted)

	final, _ := parser.Parse(iniPath)
	found := map[string]string{}
	for _, e := range final {
		found[e.Path] = e.Value
	}
	if found["default/aws_secret_access_key"] != "wJalrXUtnFEMI_K7MDENG" {
		t.Errorf("INI round-trip failed: %q", found["default/aws_secret_access_key"])
	}
}

func TestIntegrationPropertiesPipeline(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	tmpDir := t.TempDir()

	propContent := "spring.datasource.password=db_secret_123\nspring.datasource.url=jdbc:postgresql://localhost/mydb\n"
	propPath := filepath.Join(workspace, "application.properties")
	os.WriteFile(propPath, []byte(propContent), 0644)

	parser := ParserFor(FormatProperties)
	entries, err := parser.Parse(propPath)
	if err != nil {
		t.Fatal(err)
	}

	keys := map[string]bool{"spring.datasource.password": true}
	encrypted, _, err := EncryptSelected(entries, keys, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	parser.Write(propPath, encrypted)

	data, _ := os.ReadFile(propPath)
	if !strings.Contains(string(data), "ENC[age:") {
		t.Error("properties should contain ENC token")
	}

	// Scan via FileScanner
	scanner := NewFileScanner([]config.SecretFileConfig{
		{Path: propPath, Format: "properties", EncryptKeys: []string{"spring.datasource.password"}},
	}, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, PublicKey: kp.PublicKey, PrivateKey: kp.PrivateKey, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(result.Mapping))
	}

	// Decrypt round-trip
	encEntries, _ := parser.Parse(propPath)
	decrypted := make([]SecretEntry, len(encEntries))
	for i, e := range encEntries {
		if crypto.IsEncrypted(e.Value) {
			inner, _ := crypto.UnwrapENC(e.Value)
			plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
			decrypted[i] = SecretEntry{Path: e.Path, Value: plain}
		} else {
			decrypted[i] = e
		}
	}
	parser.Write(propPath, decrypted)

	final, _ := parser.Parse(propPath)
	found := map[string]string{}
	for _, e := range final {
		found[e.Path] = e.Value
	}
	if found["spring.datasource.password"] != "db_secret_123" {
		t.Errorf("properties round-trip failed: %q", found["spring.datasource.password"])
	}
}

func TestIntegrationTextPipeline(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()

	keyContent := "-----BEGIN PRIVATE KEY-----\nMIIBVgIBADANBgkqhkiG9w0BAGEAA\n-----END PRIVATE KEY-----\n"
	keyPath := filepath.Join(workspace, "server.key")
	os.WriteFile(keyPath, []byte(keyContent), 0644)

	parser := ParserFor(FormatText)
	entries, err := parser.Parse(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	encrypted, _, err := EncryptSelected(entries, nil, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	parser.Write(keyPath, encrypted)

	data, _ := os.ReadFile(keyPath)
	if !strings.Contains(string(data), "ENC[age:") {
		t.Error("text file should be fully encrypted")
	}

	// Decrypt
	encEntries, _ := parser.Parse(keyPath)
	inner, _ := crypto.UnwrapENC(encEntries[0].Value)
	plain, _ := crypto.Decrypt(inner, kp.PrivateKey)
	parser.Write(keyPath, []SecretEntry{{Path: "_content", Value: plain}})

	finalData, _ := os.ReadFile(keyPath)
	if string(finalData) != keyContent {
		t.Error("text round-trip failed")
	}
}
