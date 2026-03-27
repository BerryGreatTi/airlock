package secrets

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type EnvEntry struct {
	Key   string
	Value string
}

func ParseEnvFile(path string) ([]EnvEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer f.Close()

	var entries []EnvEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		value := line[idx+1:]
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		entries = append(entries, EnvEntry{Key: line[:idx], Value: value})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan env file: %w", err)
	}
	return entries, nil
}

// WriteEnvFile writes entries with single-quoted values to prevent shell injection.
func WriteEnvFile(path string, entries []EnvEntry) error {
	var sb strings.Builder
	for _, entry := range entries {
		sb.WriteString(entry.Key)
		sb.WriteString("='")
		sb.WriteString(entry.Value)
		sb.WriteString("'\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}
