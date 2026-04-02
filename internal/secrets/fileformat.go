package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/taeikkim92/airlock/internal/crypto"
)

// MaxSecretFileSize is the maximum file size (10 MB) that parsers will accept.
const MaxSecretFileSize = 10 * 1024 * 1024

// FileFormat identifies the format of a secret file.
type FileFormat string

const (
	FormatDotenv     FileFormat = "dotenv"
	FormatJSON       FileFormat = "json"
	FormatYAML       FileFormat = "yaml"
	FormatINI        FileFormat = "ini"
	FormatProperties FileFormat = "properties"
	FormatText       FileFormat = "text"
)

// SecretEntry represents a single key-value pair extracted from a secret file.
// Path uses "/" as separator for nested keys (e.g., "db/password", "servers/0/host").
type SecretEntry struct {
	Path      string
	Value     string
	Encrypted bool
}

// FileParser reads and writes secret files of a specific format.
// Write implementations must use atomic writes (temp file + rename).
type FileParser interface {
	Format() FileFormat
	Parse(path string) ([]SecretEntry, error)
	Write(path string, entries []SecretEntry) error
}

// DetectFormat returns the FileFormat based on the file extension.
func DetectFormat(path string) FileFormat {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(base))

	// Handle .env and .env.* patterns
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return FormatDotenv
	}

	switch ext {
	case ".json":
		return FormatJSON
	case ".yaml", ".yml":
		return FormatYAML
	case ".ini", ".cfg":
		return FormatINI
	case ".properties":
		return FormatProperties
	default:
		return FormatText
	}
}

// ParserFor returns a FileParser for the given format.
func ParserFor(format FileFormat) FileParser {
	switch format {
	case FormatDotenv:
		return &DotenvParser{}
	case FormatJSON:
		return &JSONParser{}
	case FormatYAML:
		return &YAMLParser{}
	case FormatINI:
		return &INIParser{}
	case FormatProperties:
		return &PropertiesParser{}
	case FormatText:
		return &TextParser{}
	default:
		return &TextParser{}
	}
}

// LeafKey returns the last segment of a slash-separated key path.
// For example, "db/password" returns "password".
func LeafKey(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// CheckFileSize returns an error if the file exceeds MaxSecretFileSize.
func CheckFileSize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > MaxSecretFileSize {
		return fmt.Errorf("file %s exceeds maximum size (%d bytes > %d bytes)", path, info.Size(), MaxSecretFileSize)
	}
	return nil
}

// SetEncryptedFlags sets the Encrypted field on each entry based on the ENC[age:...] pattern.
func SetEncryptedFlags(entries []SecretEntry) []SecretEntry {
	result := make([]SecretEntry, len(entries))
	for i, e := range entries {
		result[i] = SecretEntry{
			Path:      e.Path,
			Value:     e.Value,
			Encrypted: crypto.IsEncrypted(e.Value),
		}
	}
	return result
}

// AtomicWrite writes data to a file atomically by writing to a temporary file
// in the same directory and then renaming it to the target path.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".airlock-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
