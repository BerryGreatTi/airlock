package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/config"
)

var secretRemoveCmd = &cobra.Command{
	Use:   "remove <file>",
	Short: "Unregister a secret file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		airlockDir := ".airlock"

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		found := false
		filtered := make([]config.SecretFileConfig, 0, len(cfg.SecretFiles))
		for _, f := range cfg.SecretFiles {
			existing, _ := filepath.Abs(f.Path)
			if existing == absPath {
				found = true
				continue
			}
			filtered = append(filtered, f)
		}

		if !found {
			return fmt.Errorf("file not registered: %s", absPath)
		}

		cfg.SecretFiles = filtered
		if err := config.Save(cfg, airlockDir); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Removed %s\n", filepath.Base(absPath))
		return nil
	},
}

func init() {
	secretCmd.AddCommand(secretRemoveCmd)
}
