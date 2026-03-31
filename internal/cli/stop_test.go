package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
)

type stopMockRuntime struct {
	removedNames   []string
	removedNets    []string
	listContainers []container.ContainerInfo
	listErr        error
}

func (m *stopMockRuntime) EnsureNetwork(_ context.Context, opts container.NetworkOpts) (string, error) {
	return "net-" + opts.Name, nil
}
func (m *stopMockRuntime) RunDetached(_ context.Context, cfg container.ContainerConfig) (string, error) {
	return "ctr-" + cfg.Name, nil
}
func (m *stopMockRuntime) RunAttached(_ context.Context, cfg container.ContainerConfig) error {
	return nil
}
func (m *stopMockRuntime) Stop(_ context.Context, name string) error { return nil }
func (m *stopMockRuntime) Remove(_ context.Context, name string) error {
	m.removedNames = append(m.removedNames, name)
	return nil
}
func (m *stopMockRuntime) RemoveNetwork(_ context.Context, name string) error {
	m.removedNets = append(m.removedNets, name)
	return nil
}
func (m *stopMockRuntime) ConnectNetwork(_ context.Context, networkID, containerID string) error {
	return nil
}
func (m *stopMockRuntime) CopyFromContainer(_ context.Context, containerName, srcPath, dstPath string) error {
	return nil
}
func (m *stopMockRuntime) WaitForFile(_ context.Context, containerName, path string, maxRetries int) error {
	return nil
}
func (m *stopMockRuntime) ListContainers(_ context.Context, prefix string) ([]container.ContainerInfo, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listContainers, nil
}

func (m *stopMockRuntime) EnsureVolume(_ context.Context, name string) error    { return nil }
func (m *stopMockRuntime) RemoveVolume(_ context.Context, name string) error    { return nil }
func (m *stopMockRuntime) VolumeExists(_ context.Context, name string) (bool, error) {
	return true, nil
}
func (m *stopMockRuntime) ReadFromVolume(_ context.Context, volumeName, filePath, dstPath string) error {
	return nil
}

func TestStopAllWithMultipleIDs(t *testing.T) {
	mock := &stopMockRuntime{
		listContainers: []container.ContainerInfo{
			{Name: "airlock-claude-abc123", Status: "running"},
			{Name: "airlock-claude-def456", Status: "running"},
		},
	}
	cfg := config.Default()
	err := stopAll(context.Background(), mock, cfg)
	if err != nil {
		t.Fatalf("stopAll failed: %v", err)
	}

	// Each ID should produce 2 removes (claude + proxy)
	if len(mock.removedNames) != 4 {
		t.Errorf("expected 4 remove calls, got %d: %v", len(mock.removedNames), mock.removedNames)
	}

	expected := map[string]bool{
		"airlock-claude-abc123": false,
		"airlock-proxy-abc123":  false,
		"airlock-claude-def456": false,
		"airlock-proxy-def456":  false,
	}
	for _, name := range mock.removedNames {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected %s to be removed", name)
		}
	}
}

func TestStopAllFallbackToLegacy(t *testing.T) {
	mock := &stopMockRuntime{
		listContainers: []container.ContainerInfo{},
	}
	cfg := config.Default()
	err := stopAll(context.Background(), mock, cfg)
	if err != nil {
		t.Fatalf("stopAll failed: %v", err)
	}

	// Legacy fallback removes airlock-claude and airlock-proxy
	expected := map[string]bool{
		"airlock-claude": false,
		"airlock-proxy":  false,
	}
	for _, name := range mock.removedNames {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected legacy container %s to be removed", name)
		}
	}
}

func TestStopAllListContainersError(t *testing.T) {
	mock := &stopMockRuntime{
		listErr: fmt.Errorf("docker unavailable"),
	}
	cfg := config.Default()
	err := stopAll(context.Background(), mock, cfg)
	if err == nil {
		t.Fatal("expected error when ListContainers fails")
	}
}

func TestStopAllSkipsLegacyNames(t *testing.T) {
	// A container named exactly "airlock-claude-" (no ID suffix) should be
	// treated as having no ID and not cleaned up individually.
	mock := &stopMockRuntime{
		listContainers: []container.ContainerInfo{
			{Name: "airlock-claude", Status: "running"},
		},
	}
	cfg := config.Default()
	err := stopAll(context.Background(), mock, cfg)
	if err != nil {
		t.Fatalf("stopAll failed: %v", err)
	}

	// The legacy name has no extractable ID, so fallback path runs
	expected := map[string]bool{
		"airlock-claude": false,
		"airlock-proxy":  false,
	}
	for _, name := range mock.removedNames {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected legacy container %s to be removed", name)
		}
	}
}

func TestStopAllSingleID(t *testing.T) {
	mock := &stopMockRuntime{
		listContainers: []container.ContainerInfo{
			{Name: "airlock-claude-myws", Status: "running"},
		},
	}
	cfg := config.Default()
	err := stopAll(context.Background(), mock, cfg)
	if err != nil {
		t.Fatalf("stopAll failed: %v", err)
	}

	expected := map[string]bool{
		"airlock-claude-myws": false,
		"airlock-proxy-myws":  false,
	}
	for _, name := range mock.removedNames {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected %s to be removed", name)
		}
	}
}
