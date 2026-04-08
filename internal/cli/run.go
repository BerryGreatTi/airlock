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
	runWorkspace         string
	runEnvFile           string
	runPassthroughHosts  string
	runProxyPort         int
	runContainerImage    string
	runProxyImage        string
	runEnabledMCPServers string
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

		if cmd.Flags().Changed("passthrough-hosts") {
			cfg.PassthroughHosts = parseCSVList(runPassthroughHosts)
		}
		if cmd.Flags().Changed("enabled-mcps") {
			cfg.EnabledMCPServers = parseCSVList(runEnabledMCPServers)
		}

		if cmd.Flags().Changed("proxy-port") && runProxyPort > 0 {
			cfg.ProxyPort = runProxyPort
		}
		if runContainerImage != "" {
			cfg.ContainerImage = runContainerImage
		}
		if runProxyImage != "" {
			cfg.ProxyImage = runProxyImage
		}

		workspace := runWorkspace
		if workspace == "" {
			workspace, _ = os.Getwd()
		}
		workspace, _ = filepath.Abs(workspace)

		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determine home directory: %w", err)
		}

		tmpDir, err := os.MkdirTemp("", "airlock-*")
		if err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		if err := docker.EnsureVolume(ctx, volumeName); err != nil {
			return fmt.Errorf("ensure volume: %w", err)
		}

		params := orchestrator.SessionParams{
			Workspace:  workspace,
			VolumeName: volumeName,
			Config:     cfg,
			TmpDir:     tmpDir,
		}

		kp, kpErr := crypto.LoadKeyPair(keysDir)
		if kpErr == nil {
			scanners := []secrets.Scanner{
				secrets.NewClaudeScanner(),
			}
			var fileScanner *secrets.FileScanner
			if len(cfg.SecretFiles) > 0 {
				fileScanner = secrets.NewFileScanner(cfg.SecretFiles, workspace)
				scanners = append(scanners, fileScanner)
			}
			if len(cfg.EnvSecrets) > 0 {
				scanners = append(scanners, secrets.NewEnvSecretScanner(cfg.EnvSecrets))
			}
			if runEnvFile != "" && (fileScanner == nil || !fileScanner.ContainsPath(runEnvFile)) {
				scanners = append(scanners, secrets.NewEnvScanner(runEnvFile, workspace))
			}
			volSettingsDir, extractErr := orchestrator.ExtractVolumeSettings(ctx, docker, volumeName, tmpDir)
			if extractErr != nil {
				return fmt.Errorf("extract volume settings: %w", extractErr)
			}
			wsName := filepath.Base(workspace)
			scanResult, err := secrets.ScanAll(scanners, secrets.ScanOpts{
				Workspace:         workspace,
				HomeDir:           homeDir,
				PublicKey:         kp.PublicKey,
				PrivateKey:        kp.PrivateKey,
				TmpDir:            tmpDir,
				VolumeSettingsDir: volSettingsDir,
				ContainerWorkDir:  fmt.Sprintf("/workspace/%s", wsName),
				EnabledMCPServers: cfg.EnabledMCPServers,
			})
			if err != nil {
				return fmt.Errorf("scan secrets: %w", err)
			}
			params.ShadowMounts = scanResult.Mounts
			params.EnvSecrets = scanResult.Env
			if len(scanResult.Mapping) > 0 {
				mappingPath, mappingErr := secrets.SaveMapping(scanResult.Mapping, tmpDir)
				if mappingErr != nil {
					return fmt.Errorf("save mapping: %w", mappingErr)
				}
				params.MappingPath = mappingPath
			}
		}

		err = orchestrator.StartSession(ctx, docker, params)
		orchestrator.CleanupSession(ctx, docker, cfg, "")
		return err
	},
}

func init() {
	runCmd.Flags().StringVarP(&runWorkspace, "workspace", "w", "", "workspace directory (default: current directory)")
	runCmd.Flags().StringVarP(&runEnvFile, "env", "e", "", "env file to encrypt and mount")
	runCmd.Flags().StringVar(&runPassthroughHosts, "passthrough-hosts", "", "comma-separated hosts to skip proxy decryption (overrides config)")
	runCmd.Flags().IntVar(&runProxyPort, "proxy-port", 0, "proxy listening port (overrides config, default 8080)")
	runCmd.Flags().StringVar(&runContainerImage, "container-image", "", "container image (overrides config)")
	runCmd.Flags().StringVar(&runProxyImage, "proxy-image", "", "proxy image (overrides config)")
	runCmd.Flags().StringVar(&runEnabledMCPServers, "enabled-mcps", "", "comma-separated MCP server allow-list (overrides config). Empty value with this flag = disable all MCPs.")
	rootCmd.AddCommand(runCmd)
}
