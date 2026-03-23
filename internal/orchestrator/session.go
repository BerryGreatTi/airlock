package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
)

// SessionParams holds everything needed to start an airlock session.
type SessionParams struct {
	Workspace   string
	ClaudeDir   string
	Config      config.Config
	TmpDir      string
	EnvFilePath string
	MappingPath string
}

const (
	mitmproxyCAPath  = "/root/.mitmproxy/mitmproxy-ca-cert.pem"
	maxCAWaitRetries = 15
)

// StartSession creates the network, starts the proxy sidecar, extracts the
// proxy CA certificate, and runs the Claude agent container in attached mode.
func StartSession(ctx context.Context, runtime container.ContainerRuntime, params SessionParams) error {
	cfg := params.Config

	fmt.Println("Creating airlock network...")
	netOpts := container.NetworkConfig(cfg.NetworkName)
	_, err := runtime.EnsureNetwork(ctx, netOpts)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	opts := container.RunOpts{
		Workspace:        params.Workspace,
		Image:            cfg.ContainerImage,
		ProxyImage:       cfg.ProxyImage,
		NetworkName:      cfg.NetworkName,
		EnvFilePath:      params.EnvFilePath,
		MappingPath:      params.MappingPath,
		ClaudeDir:        params.ClaudeDir,
		ProxyPort:        cfg.ProxyPort,
		PassthroughHosts: cfg.PassthroughHosts,
	}

	fmt.Println("Starting decryption proxy...")
	proxyCfg := container.BuildProxyConfig(opts)
	proxyID, err := runtime.RunDetached(ctx, proxyCfg)
	if err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}
	runtime.ConnectNetwork(ctx, "bridge", proxyID)

	fmt.Println("Waiting for proxy CA certificate...")
	if err := runtime.WaitForFile(ctx, "airlock-proxy", mitmproxyCAPath, maxCAWaitRetries); err != nil {
		return fmt.Errorf("proxy CA cert not ready: %w", err)
	}

	caCertDst := filepath.Join(params.TmpDir, "mitmproxy-ca-cert.pem")
	if err := runtime.CopyFromContainer(ctx, "airlock-proxy", mitmproxyCAPath, caCertDst); err != nil {
		return fmt.Errorf("extract proxy CA cert: %w", err)
	}
	opts.CACertPath = caCertDst

	fmt.Println("Starting Claude Code...")
	fmt.Printf("Workspace: %s\n", params.Workspace)
	fmt.Println("---")

	claudeCfg := container.BuildClaudeConfig(opts)
	if err := runtime.RunAttached(ctx, claudeCfg); err != nil {
		return fmt.Errorf("run claude: %w", err)
	}

	return nil
}

// CleanupSession removes the containers and network created by StartSession.
func CleanupSession(ctx context.Context, runtime container.ContainerRuntime, cfg config.Config) {
	fmt.Println("\n--- Session ended. Cleaning up...")
	runtime.Remove(ctx, "airlock-claude")
	runtime.Remove(ctx, "airlock-proxy")
	runtime.RemoveNetwork(ctx, cfg.NetworkName)
}
