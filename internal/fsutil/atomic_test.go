package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("atomic content")

	if err := AtomicWrite(path, content, 0644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", got, content)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0644 {
		t.Errorf("permissions = %o, want 0644", info.Mode().Perm())
	}
}

func TestAtomicWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.txt")
	os.WriteFile(path, []byte("original"), 0644)

	if err := AtomicWrite(path, []byte("replaced"), 0600); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "replaced" {
		t.Errorf("got %q, want %q", got, "replaced")
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestAtomicWriteBadDir(t *testing.T) {
	err := AtomicWrite("/nonexistent/dir/file.txt", []byte("data"), 0644)
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
