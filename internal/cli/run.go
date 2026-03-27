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

		kp, kpErr := crypto.LoadKeyPair(keysDir)
		if kpErr == nil {
			scanners := []secrets.Scanner{
				secrets.NewClaudeScanner(),
			}
			if runEnvFile != "" {
				scanners = append(scanners, secrets.NewEnvScanner(runEnvFile, workspace))
			}
			scanResult, err := secrets.ScanAll(scanners, secrets.ScanOpts{
				Workspace:  workspace,
				HomeDir:    homeDir,
				PublicKey:  kp.PublicKey,
				PrivateKey: kp.PrivateKey,
				TmpDir:     tmpDir,
			})
			if err != nil {
				return fmt.Errorf("scan secrets: %w", err)
			}
			params.ShadowMounts = scanResult.Mounts
			if len(scanResult.Mapping) > 0 {
				mappingPath, mappingErr := secrets.SaveMapping(scanResult.Mapping, tmpDir)
				if mappingErr != nil {
					return fmt.Errorf("save mapping: %w", mappingErr)
				}
				params.MappingPath = mappingPath
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
