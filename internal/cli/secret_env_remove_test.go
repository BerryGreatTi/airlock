package cli_test

import (
	"strings"
	"testing"

	"github.com/taeikkim92/airlock/internal/cli"
	"github.com/taeikkim92/airlock/internal/config"
)

func TestSecretEnvRemoveKnown(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	if err := cli.RunSecretEnvAdd("FOO", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvAdd("BAR", "v", false, airlockDir); err != nil {
		t.Fatal(err)
	}
	if err := cli.RunSecretEnvRemove("FOO", airlockDir); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(airlockDir)
	if len(cfg.EnvSecrets) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(cfg.EnvSecrets))
	}
	if cfg.EnvSecrets[0].Name != "BAR" {
		t.Errorf("BAR was removed instead of FOO")
	}
}

func TestSecretEnvRemoveUnknown(t *testing.T) {
	_, _, airlockDir := setupAirlock(t)
	err := cli.RunSecretEnvRemove("NOPE", airlockDir)
	if err == nil || !strings.Contains(err.Error(), "no such env secret") {
		t.Fatalf("expected 'no such env secret' error, got %v", err)
	}
}
