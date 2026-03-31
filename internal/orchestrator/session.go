package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/secrets"
)

// SessionParams holds everything needed to start an airlock session.
type SessionParams struct {
	ID           string
	Workspace    string
	VolumeName   string // Docker named volume for .claude state
	ClaudeDir    string // Deprecated: bind mount fallback
	Config       config.Config
	TmpDir       string
	ShadowMounts []secrets.ShadowMount
	MappingPath  string
}

// ExtractVolumeSettings reads settings.json and settings.local.json from the
// named Docker volume into a temporary directory and returns that directory's
// path. Files that do not exist in the volume are silently skipped.
func ExtractVolumeSettings(ctx context.Context, runtime container.ContainerRuntime, volumeName, tmpDir string) (string, error) {
	dir := filepath.Join(tmpDir, "vol-settings")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create vol-settings dir: %w", err)
	}
	for _, name := range []string{"settings.json", "settings.local.json"} {
		dst := filepath.Join(dir, name)
		err := runtime.ReadFromVolume(ctx, volumeName, name, dst)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read %s from volume: %w", name, err)
		}
	}
	return dir, nil
}

const (
	mitmproxyCAPath  = "/root/.mitmproxy/mitmproxy-ca-cert.pem"
	maxCAWaitRetries = 30
)

// copyWithRetry retries CopyFromContainer on transient failures.
func copyWithRetry(ctx context.Context, runtime container.ContainerRuntime, containerName, srcPath, dstPath string, maxAttempts int) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		lastErr = runtime.CopyFromContainer(ctx, containerName, srcPath, dstPath)
		if lastErr == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return lastErr
}

// StartSession creates the network, starts the proxy sidecar, extracts the
// proxy CA certificate, and runs the Claude agent container in attached mode.
func StartSession(ctx context.Context, runtime container.ContainerRuntime, params SessionParams) error {
	cfg := params.Config

	if params.VolumeName != "" {
		if err := runtime.EnsureVolume(ctx, params.VolumeName); err != nil {
			return fmt.Errorf("ensure volume: %w", err)
		}
	}

	fmt.Println("Creating airlock network...")
	netOpts := container.NetworkConfig(cfg.NetworkName)
	_, err := runtime.EnsureNetwork(ctx, netOpts)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	opts := container.RunOpts{
		ID:               params.ID,
		Workspace:        params.Workspace,
		Image:            cfg.ContainerImage,
		ProxyImage:       cfg.ProxyImage,
		NetworkName:      cfg.NetworkName,
		ShadowMounts:     params.ShadowMounts,
		MappingPath:      params.MappingPath,
		VolumeName:       params.VolumeName,
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

	proxyContainerName := "airlock-proxy"
	if params.ID != "" {
		proxyContainerName = "airlock-proxy-" + params.ID
	}

	fmt.Println("Waiting for proxy CA certificate...")
	if err := runtime.WaitForFile(ctx, proxyContainerName, mitmproxyCAPath, maxCAWaitRetries); err != nil {
		return fmt.Errorf("proxy CA cert not ready: %w", err)
	}

	caCertDst := filepath.Join(params.TmpDir, "mitmproxy-ca-cert.pem")
	if err := copyWithRetry(ctx, runtime, proxyContainerName, mitmproxyCAPath, caCertDst, 3); err != nil {
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

// StartDetachedSession creates the network, starts the proxy sidecar, extracts
// the proxy CA certificate, and runs the Claude agent container in detached
// (background) mode. It returns immediately after both containers are running.
func StartDetachedSession(ctx context.Context, runtime container.ContainerRuntime, params SessionParams) error {
	cfg := params.Config

	if params.VolumeName != "" {
		if err := runtime.EnsureVolume(ctx, params.VolumeName); err != nil {
			return fmt.Errorf("ensure volume: %w", err)
		}
	}

	networkName := cfg.NetworkName
	if params.ID != "" {
		networkName = networkName + "-" + params.ID
	}

	netOpts := container.NetworkConfig(networkName)
	_, err := runtime.EnsureNetwork(ctx, netOpts)
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	opts := container.RunOpts{
		ID:               params.ID,
		Workspace:        params.Workspace,
		Image:            cfg.ContainerImage,
		ProxyImage:       cfg.ProxyImage,
		NetworkName:      networkName,
		ShadowMounts:     params.ShadowMounts,
		MappingPath:      params.MappingPath,
		VolumeName:       params.VolumeName,
		ClaudeDir:        params.ClaudeDir,
		ProxyPort:        cfg.ProxyPort,
		PassthroughHosts: cfg.PassthroughHosts,
	}

	proxyCfg := container.BuildProxyConfig(opts)
	proxyID, err := runtime.RunDetached(ctx, proxyCfg)
	if err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}
	runtime.ConnectNetwork(ctx, "bridge", proxyID)

	proxyContainerName := "airlock-proxy"
	if params.ID != "" {
		proxyContainerName = "airlock-proxy-" + params.ID
	}

	if err := runtime.WaitForFile(ctx, proxyContainerName, mitmproxyCAPath, maxCAWaitRetries); err != nil {
		return fmt.Errorf("proxy CA cert not ready: %w", err)
	}

	caCertDst := filepath.Join(params.TmpDir, "mitmproxy-ca-cert.pem")
	if err := copyWithRetry(ctx, runtime, proxyContainerName, mitmproxyCAPath, caCertDst, 3); err != nil {
		return fmt.Errorf("extract proxy CA cert: %w", err)
	}
	opts.CACertPath = caCertDst

	claudeCfg := container.BuildClaudeDetachedConfig(opts)
	if _, err := runtime.RunDetached(ctx, claudeCfg); err != nil {
		return fmt.Errorf("start agent: %w", err)
	}

	return nil
}

// CleanupSession removes the containers and network created by StartSession.
func CleanupSession(ctx context.Context, runtime container.ContainerRuntime, cfg config.Config, id string) {
	claudeName := "airlock-claude"
	proxyName := "airlock-proxy"
	networkName := cfg.NetworkName
	if id != "" {
		claudeName = "airlock-claude-" + id
		proxyName = "airlock-proxy-" + id
		networkName = networkName + "-" + id
	}
	fmt.Println("\n--- Session ended. Cleaning up...")
	runtime.Remove(ctx, claudeName)
	runtime.Remove(ctx, proxyName)
	runtime.RemoveNetwork(ctx, networkName)
}
