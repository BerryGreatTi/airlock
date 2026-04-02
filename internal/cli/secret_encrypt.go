package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var (
	secretEncryptKeys string
	secretEncryptAll  bool
	secretEncryptAuto bool
)

var secretEncryptCmd = &cobra.Command{
	Use:   "encrypt <file>",
	Short: "Encrypt keys in a secret file",
	Long:  `Encrypt specified keys in-place. Use --keys, --all, or --auto to select which keys to encrypt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		keysDir := filepath.Join(".airlock", "keys")
		airlockDir := ".airlock"

		kp, err := crypto.LoadKeyPair(keysDir)
		if err != nil {
			return fmt.Errorf("load keypair (run 'airlock init' first): %w", err)
		}

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		format := secrets.DetectFormat(absPath)
		parser := secrets.ParserFor(format)

		entries, err := parser.Parse(absPath)
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}

		// Determine which keys to encrypt
		var keySet map[string]bool
		switch {
		case secretEncryptAll:
			keySet = nil // nil = encrypt all
		case secretEncryptAuto:
			keySet = make(map[string]bool)
			for _, e := range entries {
				if !e.Encrypted && secrets.IsSecret(secrets.LeafKey(e.Path), e.Value) {
					keySet[e.Path] = true
				}
			}
			if len(keySet) == 0 {
				fmt.Println("No secrets detected by heuristic.")
				return nil
			}
		case secretEncryptKeys != "":
			keySet = make(map[string]bool)
			for _, k := range strings.Split(secretEncryptKeys, ",") {
				keySet[strings.TrimSpace(k)] = true
			}
		default:
			return fmt.Errorf("specify --keys, --all, or --auto")
		}

		encrypted, _, err := secrets.EncryptSelected(entries, keySet, kp.PublicKey, kp.PrivateKey)
		if err != nil {
			return fmt.Errorf("encrypt: %w", err)
		}

		if err := parser.Write(absPath, encrypted); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		// Update config with encrypt_keys
		cfg, loadErr := config.Load(airlockDir)
		if loadErr == nil && keySet != nil {
			for i, f := range cfg.SecretFiles {
				existing, _ := filepath.Abs(f.Path)
				if existing == absPath {
					keys := make([]string, 0, len(keySet))
					for k := range keySet {
						keys = append(keys, k)
					}
					cfg.SecretFiles[i].EncryptKeys = keys
					config.Save(cfg, airlockDir)
					break
				}
			}
		}

		encCount := 0
		if keySet == nil {
			encCount = len(entries)
		} else {
			encCount = len(keySet)
		}
		fmt.Printf("Encrypted %d keys in %s\n", encCount, filepath.Base(absPath))
		return nil
	},
}

func init() {
	secretEncryptCmd.Flags().StringVar(&secretEncryptKeys, "keys", "", "comma-separated key paths to encrypt")
	secretEncryptCmd.Flags().BoolVar(&secretEncryptAll, "all", false, "encrypt all entries")
	secretEncryptCmd.Flags().BoolVar(&secretEncryptAuto, "auto", false, "auto-detect secrets using heuristic")
	secretCmd.AddCommand(secretEncryptCmd)
}
