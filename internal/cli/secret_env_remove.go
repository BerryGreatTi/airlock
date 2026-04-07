package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
)

// RunSecretEnvRemove unregisters an env secret. Errors if the name
// is not present.
func RunSecretEnvRemove(name, airlockDir string) error {
	cfg, err := config.Load(airlockDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	filtered := make([]config.EnvSecretConfig, 0, len(cfg.EnvSecrets))
	found := false
	for _, es := range cfg.EnvSecrets {
		if es.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, es)
	}
	if !found {
		return fmt.Errorf("no such env secret: %s", name)
	}
	cfg.EnvSecrets = filtered
	if err := config.Save(cfg, airlockDir); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Removed env secret %s\n", name)
	return nil
}

var secretEnvRemoveCmd = &cobra.Command{
	Use:   "remove <NAME>",
	Short: "Unregister an env secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunSecretEnvRemove(args[0], ".airlock")
	},
}

func init() {
	secretEnvCmd.AddCommand(secretEnvRemoveCmd)
}
