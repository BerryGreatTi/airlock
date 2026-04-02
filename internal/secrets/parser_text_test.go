package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTextParserBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.pem")
	content := "-----BEGIN PRIVATE KEY-----\nMIIBVg==\n-----END PRIVATE KEY-----\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&TextParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "_content" {
		t.Errorf("path = %q, want _content", entries[0].Path)
	}
	if entries[0].Value != content {
		t.Errorf("value mismatch")
	}
}

func TestTextParserEncryptedFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("ENC[age:dGVzdA==]"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&TextParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if !entries[0].Encrypted {
		t.Error("should detect encrypted content")
	}
}

func TestTextParserRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	content := "some secret data\nwith multiple lines\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	p := &TextParser{}
	entries, err := p.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.txt")
	if err := p.Write(outPath, entries); err != nil {
		t.Fatal(err)
	}
	entries2, err := p.Parse(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Value != entries2[0].Value {
		t.Errorf("round-trip mismatch")
	}
}

func TestTextParserEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&TextParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "" {
		t.Errorf("expected empty value, got %q", entries[0].Value)
	}
}
