package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
)

func TestSecretEnvShowJSONNoPlaintext(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "ghp_super_secret_xyz", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	out, err := cli.RunSecretEnvShow("GITHUB_TOKEN", airlockDir, true)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(out), "ghp_super_secret_xyz") {
		t.Errorf("show output contains plaintext: %s", out)
	}

	var info struct {
		Name        string `json:"name"`
		Encrypted   bool   `json:"encrypted"`
		ValuePrefix string `json:"value_prefix"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.Name != "GITHUB_TOKEN" {
		t.Errorf("name = %q", info.Name)
	}
	if !info.Encrypted {
		t.Error("encrypted = false")
	}
	if !strings.HasPrefix(info.ValuePrefix, "ENC[age:") {
		t.Errorf("value_prefix = %q", info.ValuePrefix)
	}
}

func TestSecretEnvShowHumanNoPlaintext(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("GITHUB_TOKEN", "ghp_super_secret_xyz", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	out, err := cli.RunSecretEnvShow("GITHUB_TOKEN", airlockDir, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "ghp_super_secret_xyz") {
		t.Errorf("human show output contains plaintext: %s", s)
	}
	if !strings.Contains(s, "name: GITHUB_TOKEN") {
		t.Errorf("human output missing name: %s", s)
	}
	if !strings.Contains(s, "encrypted: true") {
		t.Errorf("human output missing encrypted flag: %s", s)
	}
	if !strings.Contains(s, "ENC[age:") {
		t.Errorf("human output missing ciphertext prefix: %s", s)
	}
}

func TestSecretEnvShowUnknown(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	_, err := cli.RunSecretEnvShow("NOPE", airlockDir, true)
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
}
