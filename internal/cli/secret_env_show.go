package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
)

const showValuePrefixLen = 16

// RunSecretEnvShow returns metadata about a single env secret.
// It NEVER decrypts. The ciphertext is truncated to the first
// showValuePrefixLen characters of the wrapped form.
func RunSecretEnvShow(name, airlockDir string, asJSON bool) ([]byte, error) {
	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	for _, es := range cfg.EnvSecrets {
		if es.Name != name {
			continue
		}
		prefix := es.Value
		if len(prefix) > showValuePrefixLen {
			prefix = prefix[:showValuePrefixLen]
		}
		if asJSON {
			return json.MarshalIndent(map[string]interface{}{
				"name":         es.Name,
				"encrypted":    true,
				"value_prefix": prefix,
			}, "", "  ")
		}
		out := fmt.Sprintf("name: %s\nencrypted: true\nvalue: %s...\n", es.Name, prefix)
		return []byte(out), nil
	}
	return nil, fmt.Errorf("no such env secret: %q", name)
}

var secretEnvShowJSON bool

var secretEnvShowCmd = &cobra.Command{
	Use:   "show <NAME>",
	Short: "Show env secret metadata (never decrypts)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := RunSecretEnvShow(args[0], ".airlock", secretEnvShowJSON)
		if err != nil {
			return err
		}
		printWithTrailingNewline(out)
		return nil
	},
}

func init() {
	secretEnvShowCmd.Flags().BoolVar(&secretEnvShowJSON, "json", false, "output as JSON")
	secretEnvCmd.AddCommand(secretEnvShowCmd)
}
