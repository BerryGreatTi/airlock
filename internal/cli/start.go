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

// StartOptions bundles per-flag overrides applied on top of config.yaml.
// An *Override bool field is used for flags whose presence (vs. value) is
// significant — e.g., an empty PassthroughHosts string with the override flag
// clears the config list, while no flag at all preserves it.
type StartOptions struct {
	ID                       string
	Workspace                string
	EnvFile                  string
	PassthroughHosts         string
	PassthroughOverride      bool
	ProxyPort                int
	ContainerImage           string
	ProxyImage               string
	EnabledMCPServers        string
	MCPOverride              bool
	NetworkAllowlist         string
	NetworkAllowlistOverride bool
}

// RunStart encapsulates the start logic so it can be tested without cobra.
// When PassthroughOverride is true, PassthroughHosts replaces config.yaml
// (even if empty, which clears the list). Same semantic for MCPOverride and
// EnabledMCPServers.
func RunStart(ctx context.Context, runtime container.ContainerRuntime, airlockDir string, opts StartOptions) (*StartResult, error) {
	keysDir := filepath.Join(airlockDir, "keys")

	cfg, err := config.Load(airlockDir)
	if err != nil {
		return nil, fmt.Errorf("load config (run 'airlock init' first): %w", err)
	}

	if opts.PassthroughOverride {
		cfg.PassthroughHosts = parseCSVList(opts.PassthroughHosts)
	}
	if opts.MCPOverride {
		cfg.EnabledMCPServers = parseCSVList(opts.EnabledMCPServers)
	}
	if opts.NetworkAllowlistOverride {
		cfg.NetworkAllowlist = parseCSVList(opts.NetworkAllowlist)
	}

	if opts.ProxyPort > 0 {
		cfg.ProxyPort = opts.ProxyPort
	}
	if opts.ContainerImage != "" {
		cfg.ContainerImage = opts.ContainerImage
	}
	if opts.ProxyImage != "" {
		cfg.ProxyImage = opts.ProxyImage
	}

	id := opts.ID
	workspace := opts.Workspace
	envFile := opts.EnvFile

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
		var fileScanner *secrets.FileScanner
		if len(cfg.SecretFiles) > 0 {
			fileScanner = secrets.NewFileScanner(cfg.SecretFiles, workspace)
			scanners = append(scanners, fileScanner)
		}
		if len(cfg.EnvSecrets) > 0 {
			scanners = append(scanners, secrets.NewEnvSecretScanner(cfg.EnvSecrets))
		}
		if envFile != "" && (fileScanner == nil || !fileScanner.ContainsPath(envFile)) {
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
			EnabledMCPServers: cfg.EnabledMCPServers,
		})
		if err != nil {
			return nil, fmt.Errorf("scan secrets: %w", err)
		}
		params.ShadowMounts = scanResult.Mounts
		params.EnvSecrets = scanResult.Env
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
	startID                string
	startWorkspace         string
	startEnvFile           string
	startPassthroughHosts  string
	startProxyPort         int
	startContainerImage    string
	startProxyImage        string
	startEnabledMCPServers string
	startNetworkAllowlist  string
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

		result, err := RunStart(ctx, docker, ".airlock", StartOptions{
			ID:                       startID,
			Workspace:                startWorkspace,
			EnvFile:                  startEnvFile,
			PassthroughHosts:         startPassthroughHosts,
			PassthroughOverride:      cmd.Flags().Changed("passthrough-hosts"),
			ProxyPort:                startProxyPort,
			ContainerImage:           startContainerImage,
			ProxyImage:               startProxyImage,
			EnabledMCPServers:        startEnabledMCPServers,
			MCPOverride:              cmd.Flags().Changed("enabled-mcps"),
			NetworkAllowlist:         startNetworkAllowlist,
			NetworkAllowlistOverride: cmd.Flags().Changed("network-allowlist"),
		})
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
	startCmd.Flags().StringVar(&startEnabledMCPServers, "enabled-mcps", "", "comma-separated MCP server allow-list (overrides config). Empty value with this flag = disable all MCPs.")
	startCmd.Flags().StringVar(&startNetworkAllowlist, "network-allowlist", "", "comma-separated host allow-list for outbound HTTP/HTTPS (supports *.example.com). Empty value with this flag = allow all (back-compat). Omitting the flag keeps the config value.")
	rootCmd.AddCommand(startCmd)
}
