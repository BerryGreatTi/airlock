package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPropertiesParserBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.properties")
	if err := os.WriteFile(path, []byte("key=value\nname=app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&PropertiesParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Path != "key" || entries[0].Value != "value" {
		t.Errorf("entry 0: %+v", entries[0])
	}
}

func TestPropertiesParserColonSeparator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.properties")
	if err := os.WriteFile(path, []byte("key:value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&PropertiesParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "value" {
		t.Errorf("value = %q", entries[0].Value)
	}
}

func TestPropertiesParserComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.properties")
	content := "# hash comment\n! bang comment\nkey=value\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&PropertiesParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestPropertiesParserContinuation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.properties")
	content := "key=value\\\ncontinued\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&PropertiesParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "valuecontinued" {
		t.Errorf("continuation value = %q", entries[0].Value)
	}
}

func TestPropertiesParserEncryptedFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.properties")
	content := "plain=hello\nsecret=ENC[age:dGVzdA==]\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&PropertiesParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Encrypted {
		t.Error("plain should not be encrypted")
	}
	if !entries[1].Encrypted {
		t.Error("secret should be encrypted")
	}
}

func TestPropertiesParserRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.properties")
	if err := os.WriteFile(path, []byte("a=1\nb=2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &PropertiesParser{}
	entries, err := p.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.properties")
	if err := p.Write(outPath, entries); err != nil {
		t.Fatal(err)
	}
	entries2, err := p.Parse(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(entries2) {
		t.Fatalf("round-trip: %d != %d", len(entries), len(entries2))
	}
	for i := range entries {
		if entries[i].Path != entries2[i].Path || entries[i].Value != entries2[i].Value {
			t.Errorf("mismatch at %d: %+v vs %+v", i, entries[i], entries2[i])
		}
	}
}

func TestPropertiesParserEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.properties")
	content := "\n\nkey=value\n\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&PropertiesParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}
