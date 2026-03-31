package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestClaudeScannerReadsFromVolumeSettingsDir(t *testing.T) {
	tmpDir := t.TempDir()
	volSettingsDir := filepath.Join(tmpDir, "vol-settings")
	os.MkdirAll(volSettingsDir, 0755)
	settings := `{"env":{"MY_SECRET_KEY":"sk-secret-value-12345678"}}`
	os.WriteFile(filepath.Join(volSettingsDir, "settings.json"), []byte(settings), 0644)

	kp, _ := crypto.GenerateKeyPair()
	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace:         t.TempDir(),
		HomeDir:           t.TempDir(),
		PublicKey:         kp.PublicKey,
		PrivateKey:        kp.PrivateKey,
		TmpDir:            tmpDir,
		VolumeSettingsDir: volSettingsDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mounts) == 0 {
		t.Fatal("expected shadow mounts for volume settings")
	}
	if result.Mounts[0].ContainerPath != "/home/airlock/.claude/settings.json" {
		t.Errorf("unexpected container path: %s", result.Mounts[0].ContainerPath)
	}
	if len(result.Mapping) == 0 {
		t.Error("expected mapping entries for encrypted secrets")
	}
}

func TestClaudeScannerSkipsVolumeWhenDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	volSettingsDir := filepath.Join(tmpDir, "vol-settings")
	os.MkdirAll(volSettingsDir, 0755)

	kp, _ := crypto.GenerateKeyPair()
	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace:         t.TempDir(),
		HomeDir:           t.TempDir(),
		PublicKey:         kp.PublicKey,
		PrivateKey:        kp.PrivateKey,
		TmpDir:            tmpDir,
		VolumeSettingsDir: volSettingsDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected no mounts for empty volume settings, got %d", len(result.Mounts))
	}
}

func setupTestKeys(t *testing.T) (string, string) {
	t.Helper()
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return kp.PublicKey, kp.PrivateKey
}

func TestClaudeScannerMCPEnvSecrets(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	claudeDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	settings := map[string]any{
		"mcpServers": map[string]any{
			"slack": map[string]any{
				"command": "npx",
				"env": map[string]any{
					"SLACK_TOKEN": "xoxb-1234567890-abcdef",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(result.Mounts))
	}
	if result.Mounts[0].ContainerPath != "/home/airlock/.claude/settings.json" {
		t.Errorf("unexpected container path: %s", result.Mounts[0].ContainerPath)
	}
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping entry, got %d", len(result.Mapping))
	}
	for enc, plain := range result.Mapping {
		if !crypto.IsEncrypted(enc) {
			t.Errorf("mapping key should be ENC pattern: %s", enc)
		}
		if plain != "xoxb-1234567890-abcdef" {
			t.Errorf("mapping value should be plaintext, got: %s", plain)
		}
	}
	processed, _ := os.ReadFile(result.Mounts[0].HostPath)
	if strings.Contains(string(processed), "xoxb-1234567890") {
		t.Error("processed file should not contain plaintext secret")
	}
	if !strings.Contains(string(processed), "ENC[age:") {
		t.Error("processed file should contain ENC pattern")
	}
	if !strings.Contains(string(processed), "npx") {
		t.Error("processed file should preserve non-secret values")
	}
}

func TestClaudeScannerTopLevelEnv(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	claudeDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	settings := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_API_KEY": "sk-ant-api03-realkey12345",
			"FEATURE_FLAG":     "1",
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Mapping) != 1 {
		t.Fatalf("expected 1 mapping entry, got %d: %v", len(result.Mapping), result.Mapping)
	}
	for _, plain := range result.Mapping {
		if plain != "sk-ant-api03-realkey12345" {
			t.Errorf("unexpected plaintext: %s", plain)
		}
	}
}

func TestClaudeScannerNoSecretsNoMount(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	claudeDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	settings := map[string]any{
		"env": map[string]any{
			"FEATURE_FLAG": "1",
			"LOG_LEVEL":    "debug",
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts when no secrets, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected 0 mapping when no secrets, got %d", len(result.Mapping))
	}
}

func TestClaudeScannerMissingFile(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: tmpDir, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan should not fail on missing files: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts for missing files, got %d", len(result.Mounts))
	}
}

func TestClaudeScannerProjectAndGlobal(t *testing.T) {
	pub, priv := setupTestKeys(t)
	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	workspace := t.TempDir()

	globalClaude := filepath.Join(homeDir, ".claude")
	os.MkdirAll(globalClaude, 0755)
	globalSettings := map[string]any{
		"env": map[string]any{"ANTHROPIC_API_KEY": "sk-ant-api03-globalkey123"},
	}
	data, _ := json.MarshalIndent(globalSettings, "", "  ")
	os.WriteFile(filepath.Join(globalClaude, "settings.json"), data, 0644)

	projClaude := filepath.Join(workspace, ".claude")
	os.MkdirAll(projClaude, 0755)
	projSettings := map[string]any{
		"mcpServers": map[string]any{
			"github": map[string]any{
				"env": map[string]any{"GITHUB_TOKEN": "ghp_abcdefghijklmnopqrst"},
			},
		},
	}
	data, _ = json.MarshalIndent(projSettings, "", "  ")
	os.WriteFile(filepath.Join(projClaude, "settings.json"), data, 0644)

	scanner := NewClaudeScanner()
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, HomeDir: homeDir,
		PublicKey: pub, PrivateKey: priv, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Mounts) != 2 {
		t.Errorf("expected 2 mounts (global + project), got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 2 {
		t.Errorf("expected 2 mapping entries, got %d", len(result.Mapping))
	}

	paths := make(map[string]bool)
	for _, m := range result.Mounts {
		paths[m.ContainerPath] = true
	}
	if !paths["/home/airlock/.claude/settings.json"] {
		t.Error("missing global settings mount")
	}
	if !paths["/workspace/.claude/settings.json"] {
		t.Error("missing project settings mount")
	}
}
