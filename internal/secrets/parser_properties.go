package secrets

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PropertiesParser handles Java-style .properties files.
// Assumes UTF-8 encoding. Supports backslash line continuations.
type PropertiesParser struct{}

func (p *PropertiesParser) Format() FileFormat { return FormatProperties }

func (p *PropertiesParser) Parse(path string) ([]SecretEntry, error) {
	if err := CheckFileSize(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open properties file: %w", err)
	}
	defer f.Close()

	var entries []SecretEntry
	scanner := bufio.NewScanner(f)
	var continued string
	for scanner.Scan() {
		line := scanner.Text()
		// Handle backslash line continuation
		if continued != "" {
			line = continued + strings.TrimLeft(line, " \t")
			continued = ""
		}
		if strings.HasSuffix(line, "\\") {
			continued = line[:len(line)-1]
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}

		// Split on first = or :
		idx := strings.IndexAny(trimmed, "=:")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		value := strings.TrimSpace(trimmed[idx+1:])
		entries = append(entries, SecretEntry{Path: key, Value: value})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan properties file: %w", err)
	}
	return SetEncryptedFlags(entries), nil
}

func (p *PropertiesParser) Write(path string, entries []SecretEntry) error {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.Path)
		sb.WriteByte('=')
		sb.WriteString(e.Value)
		sb.WriteByte('\n')
	}
	return AtomicWrite(path, []byte(sb.String()), 0644)
}
