package secrets

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// YAMLParser handles YAML secret files using the Node API for comment preservation.
type YAMLParser struct{}

func (p *YAMLParser) Format() FileFormat { return FormatYAML }

func (p *YAMLParser) Parse(path string) ([]SecretEntry, error) {
	if err := CheckFileSize(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml file: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil
	}
	var entries []SecretEntry
	flattenYAMLNode("", doc.Content[0], &entries)
	return SetEncryptedFlags(entries), nil
}

func flattenYAMLNode(prefix string, node *yaml.Node, entries *[]SecretEntry) {
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			path := keyNode.Value
			if prefix != "" {
				path = prefix + "/" + path
			}
			flattenYAMLNode(path, valNode, entries)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			path := strconv.Itoa(i)
			if prefix != "" {
				path = prefix + "/" + path
			}
			flattenYAMLNode(path, child, entries)
		}
	case yaml.ScalarNode:
		if node.Tag == "!!str" || node.Tag == "" {
			*entries = append(*entries, SecretEntry{Path: prefix, Value: node.Value})
		}
		// Skip non-string scalars (!!int, !!bool, !!float, !!null)
	}
}

func (p *YAMLParser) Write(path string, entries []SecretEntry) error {
	// Build entry map for lookup
	entryMap := make(map[string]string, len(entries))
	for _, e := range entries {
		entryMap[e.Path] = e.Value
	}

	// Read original to preserve structure and comments
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read yaml for write: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse yaml for write: %w", err)
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		updateYAMLNode("", doc.Content[0], entryMap)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	return AtomicWrite(path, out, 0644)
}

func updateYAMLNode(prefix string, node *yaml.Node, entryMap map[string]string) {
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			path := keyNode.Value
			if prefix != "" {
				path = prefix + "/" + path
			}
			updateYAMLNode(path, valNode, entryMap)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			path := strconv.Itoa(i)
			if prefix != "" {
				path = prefix + "/" + path
			}
			updateYAMLNode(path, child, entryMap)
		}
	case yaml.ScalarNode:
		if newVal, ok := entryMap[prefix]; ok {
			node.Value = newVal
			// Ensure ENC values are quoted in YAML
			if len(newVal) > 0 && (newVal[0] == 'E' || newVal[0] == '{' || newVal[0] == '[') {
				node.Style = yaml.DoubleQuotedStyle
			}
		}
	}
}
