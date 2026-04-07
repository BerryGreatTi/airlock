package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
)

func TestSecretEnvListEmpty(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	out, err := cli.RunSecretEnvList(airlockDir, true)
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]string
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %v", entries)
	}
}

func TestSecretEnvListJSONSorted(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("ZULU", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("ALPHA", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("MIKE", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}

	out, err := cli.RunSecretEnvList(airlockDir, true)
	if err != nil {
		t.Fatal(err)
	}

	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	want := []string{"ALPHA", "MIKE", "ZULU"}
	for i, w := range want {
		if entries[i].Name != w {
			t.Errorf("entries[%d].Name = %q, want %q", i, entries[i].Name, w)
		}
	}
	// Plaintext must not appear in JSON output.
	if strings.Contains(string(out), "\"value\"") {
		t.Error("list JSON should not include value field")
	}
}
