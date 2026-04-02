package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLParserFlat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("key: value\nname: app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&YAMLParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, e := range entries {
		found[e.Path] = e.Value
	}
	if found["key"] != "value" {
		t.Errorf("key = %q", found["key"])
	}
	if found["name"] != "app" {
		t.Errorf("name = %q", found["name"])
	}
}

func TestYAMLParserNested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "db:\n  host: localhost\n  password: secret\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&YAMLParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, e := range entries {
		found[e.Path] = e.Value
	}
	if found["db/password"] != "secret" {
		t.Errorf("db/password = %q", found["db/password"])
	}
	if found["db/host"] != "localhost" {
		t.Errorf("db/host = %q", found["db/host"])
	}
}

func TestYAMLParserArrays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "servers:\n  - host: alpha\n  - host: beta\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&YAMLParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, e := range entries {
		found[e.Path] = e.Value
	}
	if found["servers/0/host"] != "alpha" {
		t.Errorf("servers/0/host = %q", found["servers/0/host"])
	}
	if found["servers/1/host"] != "beta" {
		t.Errorf("servers/1/host = %q", found["servers/1/host"])
	}
}

func TestYAMLParserSkipsNonString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "port: 5432\ndebug: true\nname: app\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&YAMLParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (string only), got %d", len(entries))
	}
	if entries[0].Path != "name" {
		t.Errorf("expected name, got %s", entries[0].Path)
	}
}

func TestYAMLParserEncryptedQuoting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "secret: plaintext\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	p := &YAMLParser{}
	entries := []SecretEntry{
		{Path: "secret", Value: "ENC[age:dGVzdA==]"},
	}
	if err := p.Write(path, entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// ENC value should be quoted in YAML
	if !strings.Contains(string(data), "\"ENC[age:") {
		t.Errorf("ENC value should be double-quoted in YAML, got: %s", data)
	}
}

func TestYAMLParserRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "db:\n  password: secret\nname: app\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	p := &YAMLParser{}
	entries, err := p.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.yaml")
	// Write needs original file for structure preservation; copy first
	data, _ := os.ReadFile(path)
	os.WriteFile(outPath, data, 0644)
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

func TestYAMLParserEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&YAMLParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
