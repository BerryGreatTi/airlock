package secrets

import (
	"fmt"
	"strings"
)

// DotenvParser wraps the existing ParseEnvFile for parsing, uses AtomicWrite for writing.
type DotenvParser struct{}

func (p *DotenvParser) Format() FileFormat { return FormatDotenv }

func (p *DotenvParser) Parse(path string) ([]SecretEntry, error) {
	if err := CheckFileSize(path); err != nil {
		return nil, err
	}
	entries, err := ParseEnvFile(path)
	if err != nil {
		return nil, err
	}
	result := make([]SecretEntry, len(entries))
	for i, e := range entries {
		result[i] = SecretEntry{Path: e.Key, Value: e.Value}
	}
	return SetEncryptedFlags(result), nil
}

func (p *DotenvParser) Write(path string, entries []SecretEntry) error {
	var sb strings.Builder
	for _, e := range entries {
		if strings.ContainsAny(e.Value, "\n\r") {
			return fmt.Errorf("dotenv format does not support multiline values (key %q)", e.Path)
		}
		sb.WriteString(e.Path)
		sb.WriteString("='")
		sb.WriteString(e.Value)
		sb.WriteString("'\n")
	}
	return AtomicWrite(path, []byte(sb.String()), 0644)
}
