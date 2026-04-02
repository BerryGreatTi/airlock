package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/taeikkim92/airlock/internal/config"
)

// FileScanner scans registered secret files from config, encrypting
// selected keys and producing shadow mounts for the container.
type FileScanner struct {
	files     []config.SecretFileConfig
	workspace string
}

// NewFileScanner creates a FileScanner for the given registered secret files.
func NewFileScanner(files []config.SecretFileConfig, workspace string) *FileScanner {
	return &FileScanner{files: files, workspace: workspace}
}

// Name returns the scanner identifier.
func (s *FileScanner) Name() string { return "file" }

// Scan processes each registered file: parses, selectively encrypts,
// writes the encrypted version to tmpDir, and produces shadow mounts.
func (s *FileScanner) Scan(opts ScanOpts) (*ScanResult, error) {
	result := &ScanResult{Mapping: make(map[string]string)}

	for i, fc := range s.files {
		mounts, mapping, err := s.processFile(fc, opts, i)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", fc.Path, err)
		}
		result.Mounts = append(result.Mounts, mounts...)
		for k, v := range mapping {
			result.Mapping[k] = v
		}
	}

	return result, nil
}

// ContainsPath reports whether the given path matches any registered secret file.
// Both paths are resolved to absolute before comparison.
func (s *FileScanner) ContainsPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, fc := range s.files {
		absFile, err := filepath.Abs(fc.Path)
		if err != nil {
			continue
		}
		if absPath == absFile {
			return true
		}
	}
	return false
}

func (s *FileScanner) processFile(fc config.SecretFileConfig, opts ScanOpts, index int) ([]ShadowMount, map[string]string, error) {
	format := detectFileFormat(fc)

	parser := ParserFor(format)
	entries, err := parser.Parse(fc.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}

	var keys map[string]bool
	if len(fc.EncryptKeys) > 0 {
		keys = make(map[string]bool, len(fc.EncryptKeys))
		for _, k := range fc.EncryptKeys {
			keys[k] = true
		}
	}

	encrypted, mapping, err := EncryptSelected(entries, keys, opts.PublicKey, opts.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt: %w", err)
	}

	// Copy original file to tmpDir so parsers that do read-and-update-in-place
	// (YAML for comment preservation, JSON for non-string value preservation)
	// can find the original structure.
	baseName := filepath.Base(fc.Path)
	tmpPath := filepath.Join(opts.TmpDir, fmt.Sprintf("file-%d-%s", index, baseName))
	origData, err := os.ReadFile(fc.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("read original for copy: %w", err)
	}
	if err := os.WriteFile(tmpPath, origData, 0644); err != nil {
		return nil, nil, fmt.Errorf("copy to tmpdir: %w", err)
	}

	if err := parser.Write(tmpPath, encrypted); err != nil {
		return nil, nil, fmt.Errorf("write encrypted: %w", err)
	}

	// Build shadow mount: map to the container-side workspace path.
	containerPath, err := s.containerPath(fc.Path, opts, index)
	if err != nil {
		return nil, nil, fmt.Errorf("container path: %w", err)
	}

	mount := ShadowMount{
		HostPath:      tmpPath,
		ContainerPath: containerPath,
	}

	return []ShadowMount{mount}, mapping, nil
}

// containerPath computes the container-side path for a host file.
// Files inside the workspace get mapped under ContainerWorkDir.
// Files outside the workspace get mapped under /run/airlock/files/.
func (s *FileScanner) containerPath(hostPath string, opts ScanOpts, index int) (string, error) {
	absFile, err := filepath.Abs(hostPath)
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}

	absWorkspace, err := filepath.Abs(s.workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	containerWorkDir := opts.ContainerWorkDir
	if containerWorkDir == "" {
		containerWorkDir = "/workspace"
	}

	rel, err := filepath.Rel(absWorkspace, absFile)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return containerWorkDir + "/" + filepath.ToSlash(rel), nil
	}

	// File is outside workspace -- disambiguate with index to avoid collision.
	return fmt.Sprintf("/run/airlock/files/%d-%s", index, filepath.Base(hostPath)), nil
}

func detectFileFormat(fc config.SecretFileConfig) FileFormat {
	if fc.Format != "" {
		return FileFormat(strings.ToLower(fc.Format))
	}
	return DetectFormat(fc.Path)
}
