package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDotenvParserBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("KEY1=value1\nKEY2=value2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &DotenvParser{}
	entries, err := p.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Path != "KEY1" || entries[0].Value != "value1" {
		t.Errorf("entry 0: got %+v", entries[0])
	}
}

func TestDotenvParserCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment\n\nKEY=value\n# another\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&DotenvParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestDotenvParserQuotedValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "SINGLE='value1'\nDOUBLE=\"value2\"\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&DotenvParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Value != "value1" {
		t.Errorf("single quote not stripped: %q", entries[0].Value)
	}
	if entries[1].Value != "value2" {
		t.Errorf("double quote not stripped: %q", entries[1].Value)
	}
}

func TestDotenvParserEncryptedFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "PLAIN=hello\nENCRYPTED=ENC[age:dGVzdA==]\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&DotenvParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Encrypted {
		t.Error("PLAIN should not be encrypted")
	}
	if !entries[1].Encrypted {
		t.Error("ENCRYPTED should be encrypted")
	}
}

func TestDotenvParserRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("A=1\nB=2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &DotenvParser{}
	entries, err := p.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, ".env.out")
	if err := p.Write(outPath, entries); err != nil {
		t.Fatal(err)
	}
	entries2, err := p.Parse(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(entries2) {
		t.Fatalf("round-trip: %d != %d entries", len(entries), len(entries2))
	}
	for i := range entries {
		if entries[i].Path != entries2[i].Path || entries[i].Value != entries2[i].Value {
			t.Errorf("round-trip mismatch at %d: %+v vs %+v", i, entries[i], entries2[i])
		}
	}
}
