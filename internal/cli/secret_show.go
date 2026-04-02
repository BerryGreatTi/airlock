package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var secretShowJSON bool
var secretShowFormat string

var secretShowCmd = &cobra.Command{
	Use:   "show <file>",
	Short: "Show entries in a secret file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		var format secrets.FileFormat
		if secretShowFormat != "" {
			format = secrets.FileFormat(secretShowFormat)
		} else {
			format = secrets.DetectFormat(filePath)
		}

		parser := secrets.ParserFor(format)
		entries, err := parser.Parse(filePath)
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}

		if secretShowJSON {
			return showJSON(format, entries)
		}

		for _, e := range entries {
			status := "plain"
			if e.Encrypted {
				status = "encrypted"
			}
			display := e.Value
			if !e.Encrypted && len(display) > 40 {
				display = display[:40] + "..."
			}
			if e.Encrypted {
				display = "ENC[age:...]"
			}
			fmt.Printf("  %-30s  %-10s  %s\n", e.Path, status, display)
		}
		return nil
	},
}

type showOutput struct {
	Format  string      `json:"format"`
	Entries []showEntry `json:"entries"`
}

type showEntry struct {
	Path      string `json:"path"`
	Value     string `json:"value"`
	Encrypted bool   `json:"encrypted"`
	IsSecret  bool   `json:"is_secret"`
}

func showJSON(format secrets.FileFormat, entries []secrets.SecretEntry) error {
	out := showOutput{
		Format:  string(format),
		Entries: make([]showEntry, len(entries)),
	}
	for i, e := range entries {
		value := e.Value
		if !e.Encrypted && len(value) > 0 {
			value = "***"
		}
		if e.Encrypted {
			value = e.Value
		}
		out.Entries[i] = showEntry{
			Path:      e.Path,
			Value:     value,
			Encrypted: e.Encrypted,
			IsSecret:  e.Encrypted || secrets.IsSecret(secrets.LeafKey(e.Path), e.Value),
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func init() {
	secretShowCmd.Flags().BoolVar(&secretShowJSON, "json", false, "output as JSON")
	secretShowCmd.Flags().StringVar(&secretShowFormat, "format", "", "file format override")
	secretCmd.AddCommand(secretShowCmd)
}
