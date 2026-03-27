package cli

import (
	"context"
	"encoding/json"
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

// StartResult holds the JSON-serializable result of a successful start.
type StartResult struct {
	Status    string `json:"status"`
	Container string `json:"container"`
	Proxy     string `json:"proxy"`
	Network   string `json:"network"`
}

// RunStart encapsulates the start logic so it can be tested without cobra.
func RunStart(ctx context.Context, runtime container.ContainerRuntime, id, workspace, envFile, airlockDir string) (*StartResult, error) {
	keysDir := filepath.Join(airlockDir, "keys")

	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config (run 'airlock init' first): %w", err)
	}

	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	workspace, _ = filepath.Abs(workspace)

	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	tmpDir, err := os.MkdirTemp("", "airlock-"+id+"-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	params := orchestrator.SessionParams{
		ID:        id,
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
		if envFile != "" {
			scanners = append(scanners, secrets.NewEnvScanner(envFile, workspace))
		}
		scanResult, err := secrets.ScanAll(scanners, secrets.ScanOpts{
			Workspace:  workspace,
			HomeDir:    homeDir,
			PublicKey:  kp.PublicKey,
			PrivateKey: kp.PrivateKey,
			TmpDir:     tmpDir,
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
	startID        string
	startWorkspace string
	startEnvFile   string
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

		result, err := RunStart(ctx, docker, startID, startWorkspace, startEnvFile, ".airlock")
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
	rootCmd.AddCommand(startCmd)
}
