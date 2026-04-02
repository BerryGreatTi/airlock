package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// INIParser handles INI/CFG secret files.
// Paths use "section/key" format. Default section keys have no prefix.
type INIParser struct{}

func (p *INIParser) Format() FileFormat { return FormatINI }

func (p *INIParser) Parse(path string) ([]SecretEntry, error) {
	if err := CheckFileSize(path); err != nil {
		return nil, err
	}
	cfg, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("parse ini: %w", err)
	}
	var entries []SecretEntry
	for _, section := range cfg.Sections() {
		sectionName := section.Name()
		for _, key := range section.Keys() {
			entryPath := key.Name()
			if sectionName != ini.DefaultSection {
				entryPath = sectionName + "/" + key.Name()
			}
			entries = append(entries, SecretEntry{Path: entryPath, Value: key.String()})
		}
	}
	return SetEncryptedFlags(entries), nil
}

func (p *INIParser) Write(path string, entries []SecretEntry) error {
	cfg := ini.Empty()
	for _, e := range entries {
		parts := strings.SplitN(e.Path, "/", 2)
		if len(parts) == 1 {
			cfg.Section(ini.DefaultSection).Key(parts[0]).SetValue(e.Value)
		} else {
			cfg.Section(parts[0]).Key(parts[1]).SetValue(e.Value)
		}
	}
	// Write to temp file then rename for atomic write
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".airlock-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()

	if err := cfg.SaveTo(tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("save ini: %w", err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

