package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var encryptOutput string

// RunEncrypt reads an env file, encrypts all values with the age public key
// found in keysDir, and writes the result to outPath.
func RunEncrypt(envPath, outPath, keysDir string) error {
	kp, err := crypto.LoadKeyPair(keysDir)
	if err != nil {
		return fmt.Errorf("load keypair (run 'airlock init' first): %w", err)
	}

	entries, err := secrets.ParseEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("parse env file: %w", err)
	}

	result, err := secrets.EncryptEntries(entries, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		return fmt.Errorf("encrypt entries: %w", err)
	}

	if err := secrets.WriteEnvFile(outPath, result.Encrypted); err != nil {
		return fmt.Errorf("write encrypted file: %w", err)
	}

	fmt.Printf("Encrypted %d values -> %s\n", len(result.Encrypted), outPath)

	return nil
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt <envfile>",
	Short: "Encrypt values in an env file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		envPath := args[0]
		keysDir := filepath.Join(".airlock", "keys")

		outPath := encryptOutput
		if outPath == "" {
			outPath = envPath + ".enc"
		}

		return RunEncrypt(envPath, outPath, keysDir)
	},
}

func init() {
	encryptCmd.Flags().StringVarP(&encryptOutput, "output", "o", "", "output path (default: <input>.enc)")
	rootCmd.AddCommand(encryptCmd)
}
