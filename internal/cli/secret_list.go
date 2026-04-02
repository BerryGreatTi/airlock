package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var secretListJSON bool

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered secret files",
	RunE: func(cmd *cobra.Command, args []string) error {
		airlockDir := ".airlock"

		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if secretListJSON {
			return listJSON(cfg)
		}

		if len(cfg.SecretFiles) == 0 {
			fmt.Println("No secret files registered.")
			return nil
		}

		for _, f := range cfg.SecretFiles {
			format := f.Format
			if format == "" {
				format = string(secrets.DetectFormat(f.Path))
			}
			keyCount := len(f.EncryptKeys)
			name := filepath.Base(f.Path)
			if keyCount > 0 {
				fmt.Printf("  %s  (%s, %d keys selected)\n", name, format, keyCount)
			} else {
				fmt.Printf("  %s  (%s, all keys)\n", name, format)
			}
		}
		return nil
	},
}

type secretFileInfo struct {
	Path        string   `json:"path"`
	Format      string   `json:"format"`
	EncryptKeys []string `json:"encrypt_keys,omitempty"`
}

func listJSON(cfg config.Config) error {
	files := make([]secretFileInfo, len(cfg.SecretFiles))
	for i, f := range cfg.SecretFiles {
		format := f.Format
		if format == "" {
			format = string(secrets.DetectFormat(f.Path))
		}
		files[i] = secretFileInfo{
			Path:        f.Path,
			Format:      format,
			EncryptKeys: f.EncryptKeys,
		}
	}
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func init() {
	secretListCmd.Flags().BoolVar(&secretListJSON, "json", false, "output as JSON")
	secretCmd.AddCommand(secretListCmd)
}
