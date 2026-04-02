package secrets

import (
	"fmt"
	"os"
)

// TextParser treats the entire file content as a single secret.
// The entry path is always "_content".
type TextParser struct{}

func (p *TextParser) Format() FileFormat { return FormatText }

func (p *TextParser) Parse(path string) ([]SecretEntry, error) {
	if err := CheckFileSize(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read text file: %w", err)
	}
	entries := []SecretEntry{
		{Path: "_content", Value: string(data)},
	}
	return SetEncryptedFlags(entries), nil
}

func (p *TextParser) Write(path string, entries []SecretEntry) error {
	if len(entries) == 0 {
		return AtomicWrite(path, nil, 0644)
	}
	return AtomicWrite(path, []byte(entries[0].Value), 0644)
}
