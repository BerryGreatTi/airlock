package cli

import (
	"context"
	"encoding/json"
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

// StartResult holds the JSON-serializable result of a successful start.
type StartResult struct {
	Status    string `json:"status"`
	Container string `json:"container"`
	Proxy     string `json:"proxy"`
	Network   string `json:"network"`
}

// RunStart encapsulates the start logic so it can be tested without cobra.
// When passthroughOverride is true, passthroughHosts replaces config.yaml
// (even if empty, which clears the list).
func RunStart(ctx context.Context, runtime container.ContainerRuntime, id, workspace, envFile, airlockDir, passthroughHosts string, passthroughOverride bool, proxyPort int, containerImage, proxyImage string) (*StartResult, error) {
	keysDir := filepath.Join(airlockDir, "keys")

	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config (run 'airlock init' first): %w", err)
	}

	if passthroughOverride {
		if passthroughHosts == "" {
			cfg.PassthroughHosts = nil
		} else {
			hosts := strings.Split(passthroughHosts, ",")
			trimmed := make([]string, 0, len(hosts))
			for _, h := range hosts {
				if s := strings.TrimSpace(h); s != "" {
					trimmed = append(trimmed, s)
				}
			}
			cfg.PassthroughHosts = trimmed
		}
	}

	if proxyPort > 0 {
		cfg.ProxyPort = proxyPort
	}
	if containerImage != "" {
		cfg.ContainerImage = containerImage
	}
	if proxyImage != "" {
		cfg.ProxyImage = proxyImage
	}

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
		return nil, fmt.Errorf("determine home directory: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "airlock-"+id+"-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	if err := runtime.EnsureVolume(ctx, volumeName); err != nil {
		return nil, fmt.Errorf("ensure volume: %w", err)
	}

	params := orchestrator.SessionParams{
		ID:         id,
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
		if envFile != "" {
			scanners = append(scanners, secrets.NewEnvScanner(envFile, workspace))
		}
		volSettingsDir, extractErr := orchestrator.ExtractVolumeSettings(ctx, runtime, volumeName, tmpDir)
		if extractErr != nil {
			return nil, fmt.Errorf("extract volume settings: %w", extractErr)
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
		})
		if err != nil {
			return nil, fmt.Errorf("scan secrets: %w", err)
		}
		params.ShadowMounts = scanResult.Mounts
		if len(scanResult.Mapping) > 0 {
			mappingPath, mappingErr := secrets.SaveMapping(scanResult.Mapping, tmpDir)
			if mappingErr != nil {
				return nil, fmt.Errorf("save mapping: %w", mappingErr)
			}
			params.MappingPath = mappingPath
		}
	}

	if err := orchestrator.StartDetachedSession(ctx, runtime, params); err != nil {
		return nil, err
	}

	networkName := cfg.NetworkName + "-" + id

	return &StartResult{
		Status:    "running",
		Container: "airlock-claude-" + id,
		Proxy:     "airlock-proxy-" + id,
		Network:   networkName,
	}, nil
}

var (
	startID               string
	startWorkspace        string
	startEnvFile          string
	startPassthroughHosts string
	startProxyPort        int
	startContainerImage   string
	startProxyImage       string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start containers in detached mode for GUI interaction",
	Long: `Starts the proxy and agent containers in the background.
Returns JSON with container names for subsequent exec/stop operations.

Requires --id to identify this session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		result, err := RunStart(ctx, docker, startID, startWorkspace, startEnvFile, ".airlock", startPassthroughHosts, cmd.Flags().Changed("passthrough-hosts"), startProxyPort, startContainerImage, startProxyImage)
		if err != nil {
			return err
		}

		out, _ := json.Marshal(result)
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	startCmd.Flags().StringVar(&startID, "id", "", "session ID (required)")
	startCmd.MarkFlagRequired("id")
	startCmd.Flags().StringVarP(&startWorkspace, "workspace", "w", "", "workspace directory (default: current directory)")
	startCmd.Flags().StringVarP(&startEnvFile, "env", "e", "", "env file to encrypt and mount")
	startCmd.Flags().StringVar(&startPassthroughHosts, "passthrough-hosts", "", "comma-separated hosts to skip proxy decryption (overrides config)")
	startCmd.Flags().IntVar(&startProxyPort, "proxy-port", 0, "proxy listening port (overrides config, default 8080)")
	startCmd.Flags().StringVar(&startContainerImage, "container-image", "", "container image (overrides config)")
	startCmd.Flags().StringVar(&startProxyImage, "proxy-image", "", "proxy image (overrides config)")
	rootCmd.AddCommand(startCmd)
}
