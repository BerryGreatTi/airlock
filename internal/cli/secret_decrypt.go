package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var (
	secretDecryptKeys  string
	secretDecryptAll   bool
	secretDecryptFormat string
)

var secretDecryptCmd = &cobra.Command{
	Use:   "decrypt <file>",
	Short: "Decrypt keys in a secret file",
	Long:  `Decrypt specified keys in-place. Use --keys or --all to select which keys to decrypt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		keysDir := filepath.Join(".airlock", "keys")

		kp, err := crypto.LoadKeyPair(keysDir)
		if err != nil {
			return fmt.Errorf("load keypair (run 'airlock init' first): %w", err)
		}

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		var format secrets.FileFormat
		if secretDecryptFormat != "" {
			format = secrets.FileFormat(secretDecryptFormat)
		} else {
			format = secrets.DetectFormat(absPath)
		}
		parser := secrets.ParserFor(format)

		entries, err := parser.Parse(absPath)
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}

		// Determine which keys to decrypt
		var keySet map[string]bool
		if !secretDecryptAll && secretDecryptKeys != "" {
			keySet = make(map[string]bool)
			for _, k := range strings.Split(secretDecryptKeys, ",") {
				keySet[strings.TrimSpace(k)] = true
			}
		} else if !secretDecryptAll {
			return fmt.Errorf("specify --keys or --all")
		}

		decrypted := make([]secrets.SecretEntry, len(entries))
		count := 0
		for i, e := range entries {
			decrypted[i] = e
			if !crypto.IsEncrypted(e.Value) {
				continue
			}
			shouldDecrypt := keySet == nil || keySet[e.Path]
			if !shouldDecrypt {
				continue
			}
			inner, err := crypto.UnwrapENC(e.Value)
			if err != nil {
				return fmt.Errorf("unwrap %s: %w", e.Path, err)
			}
			plain, err := crypto.Decrypt(inner, kp.PrivateKey)
			if err != nil {
				return fmt.Errorf("decrypt %s: %w", e.Path, err)
			}
			decrypted[i] = secrets.SecretEntry{Path: e.Path, Value: plain}
			count++
		}

		if err := parser.Write(absPath, decrypted); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		fmt.Printf("Decrypted %d keys in %s\n", count, filepath.Base(absPath))
		return nil
	},
}

func init() {
	secretDecryptCmd.Flags().StringVar(&secretDecryptKeys, "keys", "", "comma-separated key paths to decrypt")
	secretDecryptCmd.Flags().BoolVar(&secretDecryptAll, "all", false, "decrypt all entries")
	secretDecryptCmd.Flags().StringVar(&secretDecryptFormat, "format", "", "file format override")
	secretCmd.AddCommand(secretDecryptCmd)
}
