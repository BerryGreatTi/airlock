package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONParserFlat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"API_KEY": "sk-test", "HOST": "localhost"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&JSONParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	found := map[string]string{}
	for _, e := range entries {
		found[e.Path] = e.Value
	}
	if found["API_KEY"] != "sk-test" {
		t.Errorf("API_KEY = %q", found["API_KEY"])
	}
}

func TestJSONParserNested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"db": {"host": "localhost", "password": "secret"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&JSONParser{}).Parse(path)
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

func TestJSONParserArrays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"servers": [{"host": "a"}, {"host": "b"}]}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&JSONParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, e := range entries {
		found[e.Path] = e.Value
	}
	if found["servers/0/host"] != "a" {
		t.Errorf("servers/0/host = %q", found["servers/0/host"])
	}
	if found["servers/1/host"] != "b" {
		t.Errorf("servers/1/host = %q", found["servers/1/host"])
	}
}

func TestJSONParserSkipsNonString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"port": 5432, "debug": true, "name": "app", "extra": null}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&JSONParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (only string), got %d", len(entries))
	}
	if entries[0].Path != "name" || entries[0].Value != "app" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

func TestJSONParserEncryptedFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"plain": "hello", "secret": "ENC[age:dGVzdA==]"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&JSONParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Path == "plain" && e.Encrypted {
			t.Error("plain should not be encrypted")
		}
		if e.Path == "secret" && !e.Encrypted {
			t.Error("secret should be encrypted")
		}
	}
}

func TestJSONParserRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"db": {"password": "secret"}, "name": "app"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	p := &JSONParser{}
	entries, err := p.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.json")
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
	m1 := map[string]string{}
	m2 := map[string]string{}
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

func TestJSONParserWriteValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	entries := []SecretEntry{
		{Path: "a/b", Value: "1"},
		{Path: "a/c", Value: "2"},
		{Path: "d", Value: "3"},
	}
	if err := (&JSONParser{}).Write(path, entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}

func TestJSONParserEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&JSONParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestJSONParserDeeplyNested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep.json")
	data := `{"a": {"b": {"c": {"d": "deep_value"}}}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := (&JSONParser{}).Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "a/b/c/d" || entries[0].Value != "deep_value" {
		t.Errorf("unexpected: %+v", entries[0])
	}
}
