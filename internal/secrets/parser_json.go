package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// JSONParser handles JSON secret files with nested key flattening.
type JSONParser struct{}

func (p *JSONParser) Format() FileFormat { return FormatJSON }

func (p *JSONParser) Parse(path string) ([]SecretEntry, error) {
	if err := CheckFileSize(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read json file: %w", err)
	}
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	var entries []SecretEntry
	flattenJSON("", root, &entries)
	return SetEncryptedFlags(entries), nil
}

func flattenJSON(prefix string, v interface{}, entries *[]SecretEntry) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "/" + k
			}
			flattenJSON(path, child, entries)
		}
	case []interface{}:
		for i, child := range val {
			path := strconv.Itoa(i)
			if prefix != "" {
				path = prefix + "/" + path
			}
			flattenJSON(path, child, entries)
		}
	case string:
		*entries = append(*entries, SecretEntry{Path: prefix, Value: val})
	// Skip non-string leaves (numbers, booleans, null)
	default:
	}
}

func (p *JSONParser) Write(path string, entries []SecretEntry) error {
	// Build entry map for lookup
	entryMap := make(map[string]string, len(entries))
	for _, e := range entries {
		entryMap[e.Path] = e.Value
	}

	// Read original file to preserve non-string values (numbers, booleans, nulls)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read json for write: %w", err)
	}

	if err == nil {
		// Update string values in the original structure
		var root interface{}
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse json for write: %w", err)
		}
		updateJSON("", root, entryMap)
		out, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		out = append(out, '\n')
		return AtomicWrite(path, out, 0644)
	}

	// No existing file -- rebuild from entries only (string values)
	root := rebuildJSON(entries)
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	out = append(out, '\n')
	return AtomicWrite(path, out, 0644)
}

// updateJSON updates string values in the parsed JSON tree using the entry map.
func updateJSON(prefix string, v interface{}, entryMap map[string]string) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "/" + k
			}
			if s, ok := child.(string); ok {
				if newVal, found := entryMap[path]; found && newVal != s {
					val[k] = newVal
				}
			} else {
				updateJSON(path, child, entryMap)
			}
		}
	case []interface{}:
		for i, child := range val {
			path := strconv.Itoa(i)
			if prefix != "" {
				path = prefix + "/" + path
			}
			if s, ok := child.(string); ok {
				if newVal, found := entryMap[path]; found && newVal != s {
					val[i] = newVal
				}
			} else {
				updateJSON(path, child, entryMap)
			}
		}
	}
}

// rebuildJSON reconstructs a nested JSON structure from flat SecretEntry paths.
// Used only when no original file exists.
func rebuildJSON(entries []SecretEntry) interface{} {
	root := make(map[string]interface{})
	for _, e := range entries {
		setNestedJSON(root, strings.Split(e.Path, "/"), e.Value)
	}
	return root
}

func setNestedJSON(node map[string]interface{}, parts []string, value string) {
	if len(parts) == 1 {
		node[parts[0]] = value
		return
	}
	key := parts[0]
	child, ok := node[key].(map[string]interface{})
	if !ok {
		child = make(map[string]interface{})
		node[key] = child
	}
	setNestedJSON(child, parts[1:], value)
}
