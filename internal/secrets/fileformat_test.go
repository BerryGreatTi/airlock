package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path   string
		expect FileFormat
	}{
		{".env", FormatDotenv},
		{".env.local", FormatDotenv},
		{".env.production", FormatDotenv},
		{"/path/to/.env", FormatDotenv},
		{"/path/to/.env.staging", FormatDotenv},
		{"config.json", FormatJSON},
		{"/home/user/credentials.json", FormatJSON},
		{"secrets.yaml", FormatYAML},
		{"values.yml", FormatYAML},
		{"config.ini", FormatINI},
		{"settings.cfg", FormatINI},
		{"app.properties", FormatProperties},
		{"id_rsa", FormatText},
		{"cert.pem", FormatText},
		{"random_file", FormatText},
		{"Makefile", FormatText},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectFormat(tt.path)
			if got != tt.expect {
				t.Errorf("DetectFormat(%q) = %q, want %q", tt.path, got, tt.expect)
			}
		})
	}
}

func TestLeafKey(t *testing.T) {
	tests := []struct {
		path   string
		expect string
	}{
		{"password", "password"},
		{"db/password", "password"},
		{"servers/0/host", "host"},
		{"a/b/c/d", "d"},
		{"mcpServers/slack/env/TOKEN", "TOKEN"},
		{"_content", "_content"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := LeafKey(tt.path)
			if got != tt.expect {
				t.Errorf("LeafKey(%q) = %q, want %q", tt.path, got, tt.expect)
			}
		})
	}
}

func TestParserFor(t *testing.T) {
	formats := []FileFormat{
		FormatDotenv, FormatJSON, FormatYAML,
		FormatINI, FormatProperties, FormatText,
	}
	for _, f := range formats {
		t.Run(string(f), func(t *testing.T) {
			p := ParserFor(f)
			if p == nil {
				t.Fatalf("ParserFor(%q) returned nil", f)
			}
			if p.Format() != f {
				t.Errorf("ParserFor(%q).Format() = %q", f, p.Format())
			}
		})
	}
}

func TestCheckFileSize(t *testing.T) {
	dir := t.TempDir()

	t.Run("small file passes", func(t *testing.T) {
		path := filepath.Join(dir, "small.txt")
		if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := CheckFileSize(path); err != nil {
			t.Errorf("unexpected error for small file: %v", err)
		}
	})

	t.Run("missing file errors", func(t *testing.T) {
		if err := CheckFileSize(filepath.Join(dir, "nonexistent")); err == nil {
			t.Error("expected error for missing file")
		}
	})
}

func TestSetEncryptedFlags(t *testing.T) {
	entries := []SecretEntry{
		{Path: "key1", Value: "plaintext"},
		{Path: "key2", Value: "ENC[age:dGVzdA==]"},
		{Path: "key3", Value: "another_plain"},
	}
	result := SetEncryptedFlags(entries)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	if result[0].Encrypted {
		t.Error("key1 should not be encrypted")
	}
	if !result[1].Encrypted {
		t.Error("key2 should be encrypted")
	}
	if result[2].Encrypted {
		t.Error("key3 should not be encrypted")
	}
	// Verify original entries are not mutated
	if entries[1].Encrypted {
		t.Error("original entry should not be mutated")
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := []byte("atomic content")
	if err := AtomicWrite(path, content, 0644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("permissions = %o, want 0644", info.Mode().Perm())
	}
}

func TestAtomicWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.txt")

	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := AtomicWrite(path, []byte("replaced"), 0644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "replaced" {
		t.Errorf("content = %q, want %q", got, "replaced")
	}
}
