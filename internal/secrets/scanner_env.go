package secrets

import (
	"fmt"
	"path/filepath"
	"strings"
)

type EnvScanner struct {
	envFilePath string
	workspace   string
}

func NewEnvScanner(envFilePath, workspace string) *EnvScanner {
	return &EnvScanner{envFilePath: envFilePath, workspace: workspace}
}

func (s *EnvScanner) Name() string { return "env" }

func (s *EnvScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	if s.envFilePath == "" {
		return &ScanResult{Mapping: make(map[string]string)}, nil
	}

	entries, err := ParseEnvFile(s.envFilePath)
	if err != nil {
		return nil, fmt.Errorf("parse env file: %w", err)
	}

	encResult, err := EncryptEntries(entries, opts.PublicKey, opts.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt entries: %w", err)
	}

	encPath := filepath.Join(opts.TmpDir, "env.enc")
	if err := WriteEnvFile(encPath, encResult.Encrypted); err != nil {
		return nil, fmt.Errorf("write encrypted env: %w", err)
	}

	result := &ScanResult{Mapping: encResult.Mapping}

	result.Mounts = append(result.Mounts, ShadowMount{
		HostPath:      encPath,
		ContainerPath: "/run/airlock/env.enc",
	})

	absEnvFile, err := filepath.Abs(s.envFilePath)
	if err == nil {
		absWorkspace, _ := filepath.Abs(s.workspace)
		rel, relErr := filepath.Rel(absWorkspace, absEnvFile)
		if relErr == nil && !strings.HasPrefix(rel, "..") {
			result.Mounts = append(result.Mounts, ShadowMount{
				HostPath:      encPath,
				ContainerPath: "/workspace/" + filepath.ToSlash(rel),
			})
		}
	}

	return result, nil
}
