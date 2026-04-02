package secrets

// DotenvParser wraps the existing ParseEnvFile/WriteEnvFile functions.
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
	envEntries := make([]EnvEntry, len(entries))
	for i, e := range entries {
		envEntries[i] = EnvEntry{Key: e.Path, Value: e.Value}
	}
	return WriteEnvFile(path, envEntries)
}
