package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/crypto"
	"github.com/taeikkim92/airlock/internal/orchestrator"
	"github.com/taeikkim92/airlock/internal/secrets"
)

var (
	runWorkspace string
	runEnvFile   string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Launch Claude Code in an isolated container",
	Long: `Starts a containerized Claude Code session with encrypted secrets
and a transparent decryption proxy.

All airlock commands must be run from the project root (where .airlock/ is).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		airlockDir := ".airlock"
		keysDir := filepath.Join(airlockDir, "keys")

		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config (run 'airlock init' first): %w", err)
		}

		workspace := runWorkspace
		if workspace == "" {
			workspace, _ = os.Getwd()
		}
		workspace, _ = filepath.Abs(workspace)

		homeDir, _ := os.UserHomeDir()
		claudeDir := filepath.Join(homeDir, ".claude")

		tmpDir, _ := os.MkdirTemp("", "airlock-*")
		defer os.RemoveAll(tmpDir)

		params := orchestrator.SessionParams{
			Workspace: workspace,
			ClaudeDir: claudeDir,
			Config:    cfg,
			TmpDir:    tmpDir,
		}

		if runEnvFile != "" {
			kp, err := crypto.LoadKeyPair(keysDir)
			if err != nil {
				return fmt.Errorf("load keypair: %w", err)
			}
			entries, err := secrets.ParseEnvFile(runEnvFile)
			if err != nil {
				return fmt.Errorf("parse env file: %w", err)
			}
			result, err := secrets.EncryptEntries(entries, kp.PublicKey, kp.PrivateKey)
			if err != nil {
				return fmt.Errorf("encrypt entries: %w", err)
			}
			params.EnvFilePath = filepath.Join(tmpDir, "env.enc")
			if err := secrets.WriteEnvFile(params.EnvFilePath, result.Encrypted); err != nil {
				return fmt.Errorf("write encrypted env: %w", err)
			}
			var mappingErr error
			params.MappingPath, mappingErr = secrets.SaveMapping(result.Mapping, tmpDir)
			if mappingErr != nil {
				return fmt.Errorf("save mapping: %w", mappingErr)
			}
			absEnvFile, absErr := filepath.Abs(runEnvFile)
			if absErr == nil {
				rel, relErr := filepath.Rel(workspace, absEnvFile)
				if relErr == nil && !strings.HasPrefix(rel, "..") {
					params.EnvShadowPath = "/workspace/" + filepath.ToSlash(rel)
				}
			}
		}

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		err = orchestrator.StartSession(ctx, docker, params)
		orchestrator.CleanupSession(ctx, docker, cfg, "")
		return err
	},
}

func init() {
	runCmd.Flags().StringVarP(&runWorkspace, "workspace", "w", "", "workspace directory (default: current directory)")
	runCmd.Flags().StringVarP(&runEnvFile, "env", "e", "", "env file to encrypt and mount")
	rootCmd.AddCommand(runCmd)
}
