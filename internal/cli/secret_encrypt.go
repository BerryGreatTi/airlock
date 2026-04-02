package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var (
	secretEncryptKeys   string
	secretEncryptAll    bool
	secretEncryptAuto   bool
	secretEncryptFormat string
)

// RunSecretEncrypt encrypts selected keys in a secret file in-place.
// mode: "all", "auto", or comma-separated key paths.
func RunSecretEncrypt(filePath, mode, formatOverride, keysDir, airlockDir string) error {
	kp, err := crypto.LoadKeyPair(keysDir)
	if err != nil {
		return fmt.Errorf("load keypair (run 'airlock init' first): %w", err)
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	_, parser, err := secrets.ResolveParser(absPath, formatOverride)
	if err != nil {
		return err
	}

	entries, err := parser.Parse(absPath)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	var keySet map[string]bool
	switch mode {
	case "all":
		keySet = nil
	case "auto":
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
	default:
		keySet = make(map[string]bool)
		for _, k := range strings.Split(mode, ",") {
			keySet[strings.TrimSpace(k)] = true
		}
	}

	encrypted, _, err := secrets.EncryptSelected(entries, keySet, kp.PublicKey, kp.PrivateKey)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := parser.Write(absPath, encrypted); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	cfg, loadErr := config.Load(airlockDir)
	if loadErr == nil && keySet != nil {
		for i, f := range cfg.SecretFiles {
			existing, _ := filepath.Abs(f.Path)
			if existing == absPath {
				keys := make([]string, 0, len(keySet))
				for k := range keySet {
					keys = append(keys, k)
				}
				updated := cfg.SecretFiles[i]
				updated.EncryptKeys = keys
				cfg.SecretFiles[i] = updated
				if err := config.Save(cfg, airlockDir); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save encrypt_keys to config: %v\n", err)
				}
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
}

var secretEncryptCmd = &cobra.Command{
	Use:   "encrypt <file>",
	Short: "Encrypt keys in a secret file",
	Long:  `Encrypt specified keys in-place. Use --keys, --all, or --auto to select which keys to encrypt.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var mode string
		switch {
		case secretEncryptAll:
			mode = "all"
		case secretEncryptAuto:
			mode = "auto"
		case secretEncryptKeys != "":
			mode = secretEncryptKeys
		default:
			return fmt.Errorf("specify --keys, --all, or --auto")
		}
		return RunSecretEncrypt(args[0], mode, secretEncryptFormat, filepath.Join(".airlock", "keys"), ".airlock")
	},
}

func init() {
	secretEncryptCmd.Flags().StringVar(&secretEncryptKeys, "keys", "", "comma-separated key paths to encrypt")
	secretEncryptCmd.Flags().BoolVar(&secretEncryptAll, "all", false, "encrypt all entries")
	secretEncryptCmd.Flags().BoolVar(&secretEncryptAuto, "auto", false, "auto-detect secrets using heuristic")
	secretEncryptCmd.Flags().StringVar(&secretEncryptFormat, "format", "", "file format override")
	secretCmd.AddCommand(secretEncryptCmd)
}
