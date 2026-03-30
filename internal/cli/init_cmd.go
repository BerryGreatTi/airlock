package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/crypto"
)

// RunInit creates the .airlock directory structure, generates an age key pair,
// and writes a default config file. It returns an error if the directory
// already exists.
func RunInit(airlockDir string) error {
	keysDir := filepath.Join(airlockDir, "keys")

	if _, err := os.Stat(airlockDir); err == nil {
		return fmt.Errorf(".airlock/ already exists; remove it first to reinitialize")
	}

	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("create .airlock/keys: %w", err)
	}

	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generate keypair: %w", err)
	}

	if err := crypto.SaveKeyPair(kp, keysDir); err != nil {
		return fmt.Errorf("save keypair: %w", err)
	}

	cfg := config.Default()
	if err := config.Save(cfg, airlockDir); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println("Initialized .airlock/")
	fmt.Printf("  Public key: %s\n", kp.PublicKey)
	fmt.Println("  Config:     .airlock/config.yaml")

	// Create persistent volume for Claude Code state
	docker, dockerErr := container.NewDocker()
	if dockerErr == nil {
		defer docker.Close()
		ctx := context.Background()
		if err := docker.EnsureVolume(ctx, cfg.VolumeName); err != nil {
			fmt.Printf("Warning: could not create volume %s: %v\n", cfg.VolumeName, err)
		} else {
			fmt.Printf("  Volume:    %s\n", cfg.VolumeName)
		}
	}

	fmt.Println()
	fmt.Println("Add .airlock/keys/ to .gitignore")
	fmt.Println("Run 'airlock config import' to import your host Claude Code settings.")

	return nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize airlock in the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunInit(".airlock")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
