package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var secretAddFormat string

var secretAddCmd = &cobra.Command{
	Use:   "add <file>",
	Short: "Register a secret file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		airlockDir := ".airlock"

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		// Detect or use explicit format
		var format secrets.FileFormat
		if secretAddFormat != "" {
			format = secrets.FileFormat(secretAddFormat)
		} else {
			format = secrets.DetectFormat(absPath)
		}

		// Validate by parsing
		parser := secrets.ParserFor(format)
		entries, err := parser.Parse(absPath)
		if err != nil {
			return fmt.Errorf("parse file: %w", err)
		}

		// Load config and check for duplicates
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config (run 'airlock init' first): %w", err)
		}

		for _, f := range cfg.SecretFiles {
			existing, _ := filepath.Abs(f.Path)
			if existing == absPath {
				return fmt.Errorf("file already registered: %s", absPath)
			}
		}

		cfg.SecretFiles = append(cfg.SecretFiles, config.SecretFileConfig{
			Path:   absPath,
			Format: string(format),
		})

		if err := config.Save(cfg, airlockDir); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Registered %s (%s, %d entries)\n", filepath.Base(absPath), format, len(entries))
		return nil
	},
}

func init() {
	secretAddCmd.Flags().StringVar(&secretAddFormat, "format", "", "file format (dotenv, json, yaml, ini, properties, text)")
	secretCmd.AddCommand(secretAddCmd)
}
