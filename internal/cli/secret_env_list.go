package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
)

// RunSecretEnvList returns a representation of the registered env
// secrets. When asJSON is true the return is a sorted JSON array of
// {"name": ...} objects (the GUI contract). Otherwise the return is
// a human-readable table as bytes.
//
// The plaintext value is never returned. The ciphertext is also not
// returned by 'list'; use 'show' for a (truncated) ciphertext fingerprint.
func RunSecretEnvList(airlockDir string, asJSON bool) ([]byte, error) {
	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	type entry struct {
		Name string `json:"name"`
	}
	entries := make([]entry, 0, len(cfg.EnvSecrets))
	for _, es := range cfg.EnvSecrets {
		entries = append(entries, entry{Name: es.Name})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	if asJSON {
		return json.MarshalIndent(entries, "", "  ")
	}

	if len(entries) == 0 {
		return []byte("No env secrets registered.\n"), nil
	}
	out := "NAME\n"
	for _, e := range entries {
		out += "  " + e.Name + "\n"
	}
	return []byte(out), nil
}

var secretEnvListJSON bool

var secretEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered env secrets (names only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := RunSecretEnvList(".airlock", secretEnvListJSON)
		if err != nil {
			return err
		}
		printWithTrailingNewline(out)
		return nil
	},
}

func init() {
	secretEnvListCmd.Flags().BoolVar(&secretEnvListJSON, "json", false, "output as JSON")
	secretEnvCmd.AddCommand(secretEnvListCmd)
}
