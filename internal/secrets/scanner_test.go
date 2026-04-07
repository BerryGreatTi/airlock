package secrets

import (
	"fmt"
	"testing"
)

type mockScanner struct {
	name   string
	result *ScanResult
	err    error
}

func (m *mockScanner) Name() string                            { return m.name }
func (m *mockScanner) Scan(opts ScanOpts) (*ScanResult, error) { return m.result, m.err }

func TestScanAllMergesResults(t *testing.T) {
	s1 := &mockScanner{
		name: "s1",
		result: &ScanResult{
			Mounts:  []ShadowMount{{HostPath: "/tmp/a.json", ContainerPath: "/workspace/a.json"}},
			Mapping: map[string]string{"ENC[age:aaa]": "secret_a"},
		},
	}
	s2 := &mockScanner{
		name: "s2",
		result: &ScanResult{
			Mounts:  []ShadowMount{{HostPath: "/tmp/b.json", ContainerPath: "/workspace/b.json"}},
			Mapping: map[string]string{"ENC[age:bbb]": "secret_b"},
		},
	}

	result, err := ScanAll([]Scanner{s1, s2}, ScanOpts{})
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}
	if len(result.Mounts) != 2 {
		t.Errorf("expected 2 mounts, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 2 {
		t.Errorf("expected 2 mapping entries, got %d", len(result.Mapping))
	}
	if result.Mapping["ENC[age:aaa]"] != "secret_a" {
		t.Error("missing mapping from s1")
	}
	if result.Mapping["ENC[age:bbb]"] != "secret_b" {
		t.Error("missing mapping from s2")
	}
}

func TestScanAllPropagatesError(t *testing.T) {
	s1 := &mockScanner{name: "good", result: &ScanResult{Mapping: map[string]string{}}}
	s2 := &mockScanner{name: "bad", err: fmt.Errorf("parse failed")}

	_, err := ScanAll([]Scanner{s1, s2}, ScanOpts{})
	if err == nil {
		t.Fatal("expected error from failing scanner")
	}
	if got := err.Error(); got != "scanner bad: parse failed" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestScanAllEmptyScanners(t *testing.T) {
	result, err := ScanAll([]Scanner{}, ScanOpts{})
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(result.Mounts))
	}
	if len(result.Mapping) != 0 {
		t.Errorf("expected 0 mapping entries, got %d", len(result.Mapping))
	}
}

func TestScanAllNilResult(t *testing.T) {
	s := &mockScanner{name: "nil", result: nil}
	result, err := ScanAll([]Scanner{s}, ScanOpts{})
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}
	if len(result.Mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(result.Mounts))
	}
}

func TestScanAllMergesEnv(t *testing.T) {
	a := &mockScanner{
		name: "a",
		result: &ScanResult{
			Env: []EnvVar{{Name: "FOO", Value: "ENC[age:AAA]"}},
		},
	}
	b := &mockScanner{
		name: "b",
		result: &ScanResult{
			Env: []EnvVar{{Name: "BAR", Value: "ENC[age:BBB]"}},
		},
	}
	merged, err := ScanAll([]Scanner{a, b}, ScanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Env) != 2 {
		t.Fatalf("expected 2 env entries, got %d", len(merged.Env))
	}
	names := map[string]string{}
	for _, e := range merged.Env {
		names[e.Name] = e.Value
	}
	if names["FOO"] != "ENC[age:AAA]" || names["BAR"] != "ENC[age:BBB]" {
		t.Errorf("unexpected merged env: %v", names)
	}
}
