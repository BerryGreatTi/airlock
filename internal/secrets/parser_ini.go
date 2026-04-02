package secrets

import (
	"bytes"
	"fmt"
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
	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return fmt.Errorf("marshal ini: %w", err)
	}
	return AtomicWrite(path, buf.Bytes(), 0644)
}

