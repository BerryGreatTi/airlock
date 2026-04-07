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

func TestSecretEnvListHumanEmpty(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	out, err := cli.RunSecretEnvList(airlockDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "No env secrets registered") {
		t.Errorf("human empty output = %q, want a 'No env secrets' message", out)
	}
}

func TestSecretEnvListHumanWithEntries(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("BETA", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("ALPHA", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	out, err := cli.RunSecretEnvList(airlockDir, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "NAME") {
		t.Errorf("human output missing NAME header: %q", s)
	}
	// Sorted: ALPHA must appear before BETA.
	alphaIdx := strings.Index(s, "ALPHA")
	betaIdx := strings.Index(s, "BETA")
	if alphaIdx == -1 || betaIdx == -1 {
		t.Fatalf("both names should appear in output: %q", s)
	}
	if alphaIdx > betaIdx {
		t.Errorf("human output not sorted: ALPHA at %d, BETA at %d", alphaIdx, betaIdx)
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
