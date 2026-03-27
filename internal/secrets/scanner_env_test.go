package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/crypto"
)

func TestEnvScannerEncryptsAndMounts(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	tmpDir := t.TempDir()
	workspace := t.TempDir()

	envPath := filepath.Join(workspace, ".env")
	os.WriteFile(envPath, []byte("STRIPE_KEY=sk_live_abcdefghijk\nLOG=debug\n"), 0644)

	scanner := NewEnvScanner(envPath, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, HomeDir: t.TempDir(),
		PublicKey: kp.PublicKey, PrivateKey: kp.PrivateKey, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(result.Mounts) != 2 {
		t.Errorf("expected 2 mounts (env.enc + shadow), got %d: %v", len(result.Mounts), result.Mounts)
	}

	if len(result.Mapping) == 0 {
		t.Error("expected mapping entries")
	}

	hasShadow := false
	for _, m := range result.Mounts {
		if m.ContainerPath == "/workspace/.env" {
			hasShadow = true
		}
	}
	if !hasShadow {
		t.Error("expected shadow mount for /workspace/.env")
	}
}

func TestEnvScannerOutsideWorkspace(t *testing.T) {
	kp, _ := crypto.GenerateKeyPair()
	tmpDir := t.TempDir()
	workspace := t.TempDir()
	externalDir := t.TempDir()

	envPath := filepath.Join(externalDir, "secrets.env")
	os.WriteFile(envPath, []byte("API_KEY=sk-ant-test-12345678\n"), 0644)

	scanner := NewEnvScanner(envPath, workspace)
	result, err := scanner.Scan(ScanOpts{
		Workspace: workspace, HomeDir: t.TempDir(),
		PublicKey: kp.PublicKey, PrivateKey: kp.PrivateKey, TmpDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(result.Mounts) != 1 {
		t.Errorf("expected 1 mount (env.enc only), got %d", len(result.Mounts))
	}
	for _, m := range result.Mounts {
		if strings.Contains(m.ContainerPath, "/workspace/") {
			t.Errorf("should not have workspace shadow for external file: %s", m.ContainerPath)
		}
	}
}

func TestEnvScannerEmptyPath(t *testing.T) {
	scanner := NewEnvScanner("", "")
	result, err := scanner.Scan(ScanOpts{TmpDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts for empty path, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected 0 mapping for empty path, got %d", len(result.Mapping))
	}
}
