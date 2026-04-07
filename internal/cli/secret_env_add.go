package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/crypto"
)

// RunSecretEnvAdd registers an env secret. The plaintext value is
// encrypted with the workspace age public key and stored as
// ENC[age:...] ciphertext in config.yaml. If value is already wrapped
// in ENC[age:...] it is stored as-is (idempotent round-tripping).
func RunSecretEnvAdd(name, value string, force bool, airlockDir string) error {
	keysDir := filepath.Join(airlockDir, "keys")
	kp, err := crypto.LoadKeyPair(keysDir)
	if err != nil {
		return fmt.Errorf("no encryption keys found; run 'airlock init' first: %w", err)
	}

	cfg, err := config.Load(airlockDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !config.IsValidEnvVarName(name) {
		return fmt.Errorf("invalid name %q: must match %s", name, config.EnvVarNamePattern)
	}
	if config.ReservedEnvNames[name] {
		return fmt.Errorf("env secret name %q is reserved by airlock", name)
	}

	var stored string
	if crypto.IsEncrypted(value) {
		stored = value
	} else {
		ct, encErr := crypto.Encrypt(value, kp.PublicKey)
		if encErr != nil {
			return fmt.Errorf("encrypt: %w", encErr)
		}
		stored = crypto.WrapENC(ct)
	}

	// Upsert: in-place update if the name exists, otherwise append.
	action := "Added"
	found := false
	for i, es := range cfg.EnvSecrets {
		if es.Name != name {
			continue
		}
		if !force {
			return fmt.Errorf("env secret %q already exists; use --force to overwrite", name)
		}
		cfg.EnvSecrets[i].Value = stored
		action = "Updated"
		found = true
		break
	}
	if !found {
		cfg.EnvSecrets = append(cfg.EnvSecrets, config.EnvSecretConfig{
			Name:  name,
			Value: stored,
		})
	}
	if err := config.Save(cfg, airlockDir); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("%s env secret %s\n", action, name)
	return nil
}

// readSecretValue resolves --value / --stdin / TTY prompt precedence.
// Returns an error for non-TTY without explicit flags.
func readSecretValue(name string, valueFlag string, valueFlagSet bool, stdinFlag bool, stdin io.Reader) (string, error) {
	if valueFlagSet && stdinFlag {
		return "", errors.New("--value and --stdin are mutually exclusive")
	}
	if valueFlagSet {
		return valueFlag, nil
	}
	if stdinFlag {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return strings.TrimRight(string(data), "\n"), nil
	}
	// TTY prompt
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("refusing to read plaintext from non-terminal without --value or --stdin")
	}
	fmt.Printf("Value for %s: ", name)
	bytePass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(bytePass), nil
}

var (
	secretEnvAddValue string
	secretEnvAddStdin bool
	secretEnvAddForce bool
)

var secretEnvAddCmd = &cobra.Command{
	Use:   "add <NAME>",
	Short: "Register an environment-variable secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		valueFlagSet := cmd.Flags().Changed("value")
		value, err := readSecretValue(
			args[0],
			secretEnvAddValue, valueFlagSet,
			secretEnvAddStdin,
			os.Stdin,
		)
		if err != nil {
			return err
		}
		return RunSecretEnvAdd(args[0], value, secretEnvAddForce, ".airlock")
	},
}

func init() {
	secretEnvAddCmd.Flags().StringVar(&secretEnvAddValue, "value", "", "value (warning: visible in process list)")
	secretEnvAddCmd.Flags().BoolVar(&secretEnvAddStdin, "stdin", false, "read value from stdin")
	secretEnvAddCmd.Flags().BoolVar(&secretEnvAddForce, "force", false, "overwrite existing entry")
	secretEnvCmd.AddCommand(secretEnvAddCmd)
}
