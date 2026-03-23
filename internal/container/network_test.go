package container_test

import (
	"testing"

	"github.com/taeikkim92/airlock/internal/container"
)

func TestNetworkConfig(t *testing.T) {
	cfg := container.NetworkConfig("airlock-net")
	if cfg.Name != "airlock-net" {
		t.Errorf("expected airlock-net, got %s", cfg.Name)
	}
	if cfg.Driver != "bridge" {
		t.Errorf("expected bridge, got %s", cfg.Driver)
	}
	if !cfg.Internal {
		t.Error("expected internal network")
	}
}
