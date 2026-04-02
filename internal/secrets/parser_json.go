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
	root := rebuildJSON(entries)
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')
	return AtomicWrite(path, data, 0644)
}

// rebuildJSON reconstructs a nested JSON structure from flat SecretEntry paths.
func rebuildJSON(entries []SecretEntry) interface{} {
	root := make(map[string]interface{})
	for _, e := range entries {
		setNestedJSON(root, strings.Split(e.Path, "/"), e.Value)
	}
	return simplifyJSON(root)
}

func setNestedJSON(node map[string]interface{}, parts []string, value string) {
	if len(parts) == 1 {
		node[parts[0]] = value
		return
	}
	key := parts[0]
	next := parts[1]
	if _, isIdx := strconv.Atoi(next); isIdx == nil {
		// Next segment is an array index
		arr, ok := node[key].([]interface{})
		if !ok {
			arr = []interface{}{}
		}
		idx, _ := strconv.Atoi(next)
		for len(arr) <= idx {
			arr = append(arr, make(map[string]interface{}))
		}
		if len(parts) == 2 {
			arr[idx] = value
		} else {
			child, ok := arr[idx].(map[string]interface{})
			if !ok {
				child = make(map[string]interface{})
				arr[idx] = child
			}
			setNestedJSON(child, parts[2:], value)
		}
		node[key] = arr
	} else {
		child, ok := node[key].(map[string]interface{})
		if !ok {
			child = make(map[string]interface{})
			node[key] = child
		}
		setNestedJSON(child, parts[1:], value)
	}
}

// simplifyJSON converts numeric-keyed maps to arrays where appropriate.
func simplifyJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			val[k] = simplifyJSON(child)
		}
		return val
	case []interface{}:
		for i, child := range val {
			val[i] = simplifyJSON(child)
		}
		return val
	default:
		return v
	}
}
