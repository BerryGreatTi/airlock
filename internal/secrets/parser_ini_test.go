package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestINIParserDefaultSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	if err := os.WriteFile(path, []byte("key=value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&INIParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "key" {
		t.Errorf("path = %q, want 'key'", entries[0].Path)
	}
}

func TestINIParserNamedSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	content := "[database]\nhost = localhost\npassword = secret\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&INIParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, e := range entries {
		found[e.Path] = e.Value
	}
	if found["database/host"] != "localhost" {
		t.Errorf("database/host = %q", found["database/host"])
	}
	if found["database/password"] != "secret" {
		t.Errorf("database/password = %q", found["database/password"])
	}
}

func TestINIParserMultipleSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aws.ini")
	content := "[default]\naws_access_key_id = AKIAXXX\naws_secret_access_key = secret123\n\n[production]\naws_access_key_id = AKIAYYY\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&INIParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestINIParserBracketValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	content := "[section]\nkey = ENC[age:dGVzdA==]\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&INIParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "ENC[age:dGVzdA==]" {
		t.Errorf("bracket value not preserved: %q", entries[0].Value)
	}
	if !entries[0].Encrypted {
		t.Error("should detect encrypted value")
	}
}

func TestINIParserRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	content := "[section]\nhost = localhost\npassword = secret\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	p := &INIParser{}
	entries, err := p.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.ini")
	if err := p.Write(outPath, entries); err != nil {
		t.Fatal(err)
	}
	entries2, err := p.Parse(outPath)
	if err != nil {
		t.Fatal(err)
	}
	m1, m2 := map[string]string{}, map[string]string{}
	for _, e := range entries {
		m1[e.Path] = e.Value
	}
	for _, e := range entries2 {
		m2[e.Path] = e.Value
	}
	for k, v := range m1 {
		if m2[k] != v {
			t.Errorf("round-trip mismatch at %s: %q vs %q", k, v, m2[k])
		}
	}
}
