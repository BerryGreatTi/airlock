package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taeikkim92/airlock/internal/crypto"
)

type claudeSettingsFile struct {
	hostPath      string
	containerPath string
}

type ClaudeScanner struct{}

func NewClaudeScanner() *ClaudeScanner {
	return &ClaudeScanner{}
}

func (s *ClaudeScanner) Name() string { return "claude" }

func (s *ClaudeScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	var files []claudeSettingsFile

	if opts.VolumeSettingsDir != "" {
		files = append(files,
			claudeSettingsFile{filepath.Join(opts.VolumeSettingsDir, "settings.json"), "/home/airlock/.claude/settings.json"},
			claudeSettingsFile{filepath.Join(opts.VolumeSettingsDir, "settings.local.json"), "/home/airlock/.claude/settings.local.json"},
		)
	} else {
		files = append(files,
			claudeSettingsFile{filepath.Join(opts.HomeDir, ".claude", "settings.json"), "/home/airlock/.claude/settings.json"},
			claudeSettingsFile{filepath.Join(opts.HomeDir, ".claude", "settings.local.json"), "/home/airlock/.claude/settings.local.json"},
		)
	}

	files = append(files,
		claudeSettingsFile{filepath.Join(opts.Workspace, ".claude", "settings.json"), "/workspace/.claude/settings.json"},
		claudeSettingsFile{filepath.Join(opts.Workspace, ".claude", "settings.local.json"), "/workspace/.claude/settings.local.json"},
	)

	result := &ScanResult{Mapping: make(map[string]string)}

	for _, f := range files {
		mounts, mapping, err := s.processFile(f, opts)
		if err != nil {
			return nil, fmt.Errorf("process %s: %w", f.hostPath, err)
		}
		result.Mounts = append(result.Mounts, mounts...)
		for k, v := range mapping {
			result.Mapping[k] = v
		}
	}

	return result, nil
}

func (s *ClaudeScanner) processFile(f claudeSettingsFile, opts ScanOpts) ([]ShadowMount, map[string]string, error) {
	data, err := os.ReadFile(f.hostPath)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("parse JSON: %w", err)
	}

	mapping := make(map[string]string)
	modified := false

	if envBlock, ok := root["env"].(map[string]any); ok {
		if encryptEnvBlock(envBlock, opts.PublicKey, mapping) {
			modified = true
		}
	}

	if mcpServers, ok := root["mcpServers"].(map[string]any); ok {
		for _, serverVal := range mcpServers {
			server, ok := serverVal.(map[string]any)
			if !ok {
				continue
			}
			if envBlock, ok := server["env"].(map[string]any); ok {
				if encryptEnvBlock(envBlock, opts.PublicKey, mapping) {
					modified = true
				}
			}
		}
	}

	if !modified {
		return nil, nil, nil
	}

	processed, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal: %w", err)
	}

	baseName := filepath.Base(f.hostPath)
	prefix := "global-"
	if len(f.containerPath) >= 10 && f.containerPath[:10] == "/workspace" {
		prefix = "proj-"
	}
	tmpPath := filepath.Join(opts.TmpDir, prefix+baseName)
	if err := os.WriteFile(tmpPath, processed, 0644); err != nil {
		return nil, nil, fmt.Errorf("write: %w", err)
	}

	mount := ShadowMount{HostPath: tmpPath, ContainerPath: f.containerPath}
	return []ShadowMount{mount}, mapping, nil
}

func encryptEnvBlock(envBlock map[string]any, publicKey string, mapping map[string]string) bool {
	modified := false
	for key, val := range envBlock {
		value, ok := val.(string)
		if !ok {
			continue
		}
		if crypto.IsEncrypted(value) {
			continue
		}
		if !IsSecret(key, value) {
			continue
		}
		ciphertext, err := crypto.Encrypt(value, publicKey)
		if err != nil {
			continue
		}
		wrapped := crypto.WrapENC(ciphertext)
		envBlock[key] = wrapped
		mapping[wrapped] = value
		modified = true
	}
	return modified
}
