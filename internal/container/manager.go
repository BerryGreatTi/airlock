package container

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/taeikkim92/airlock/internal/secrets"
)

// RunOpts holds all options needed to launch the airlock container pair.
type RunOpts struct {
	ID               string
	Workspace        string
	Image            string
	ProxyImage       string
	NetworkName      string
	ShadowMounts     []secrets.ShadowMount
	MappingPath      string
	VolumeName       string
	ClaudeDir        string // Deprecated: use VolumeName for writable volume mount.
	CACertPath       string
	ProxyPort        int
	PassthroughHosts []string
}

// Validate checks that required fields are present.
func (o RunOpts) Validate() error {
	if o.Workspace == "" {
		return fmt.Errorf("workspace path is required")
	}
	if o.Image == "" {
		return fmt.Errorf("container image is required")
	}
	return nil
}

// ContainerConfig describes a container to be launched.
type ContainerConfig struct {
	Image      string
	Name       string
	Binds      []string
	Mounts     []mount.Mount
	Env        []string
	Network    string
	CapDrop    []string
	WorkingDir string
	Tty        bool
	Stdin      bool
	Cmd        []string
}

// BuildProxyConfig returns a ContainerConfig for the mitmproxy sidecar.
func BuildProxyConfig(opts RunOpts) ContainerConfig {
	name := "airlock-proxy"
	if opts.ID != "" {
		name = "airlock-proxy-" + opts.ID
	}
	passthroughStr := strings.Join(opts.PassthroughHosts, ",")

	var binds []string
	if opts.MappingPath != "" {
		binds = append(binds, fmt.Sprintf("%s:/run/airlock/mapping.json:ro", opts.MappingPath))
	}

	return ContainerConfig{
		Image:   opts.ProxyImage,
		Name:    name,
		Network: opts.NetworkName,
		Binds:   binds,
		Env: []string{
			"AIRLOCK_MAPPING_PATH=/run/airlock/mapping.json",
			fmt.Sprintf("AIRLOCK_PASSTHROUGH_HOSTS=%s", passthroughStr),
		},
		CapDrop: []string{"ALL"},
	}
}

// BuildClaudeDetachedConfig returns a ContainerConfig for the AI agent container
// in detached (background) mode. It uses the keep-alive entrypoint instead of
// running claude interactively, and disables TTY/Stdin since no terminal is attached.
func BuildClaudeDetachedConfig(opts RunOpts) ContainerConfig {
	cfg := BuildClaudeConfig(opts)
	cfg.Cmd = []string{"/usr/local/bin/entrypoint-keepalive.sh"}
	cfg.Tty = false
	cfg.Stdin = false
	return cfg
}

// BuildClaudeConfig returns a ContainerConfig for the AI agent container.
func BuildClaudeConfig(opts RunOpts) ContainerConfig {
	claudeName := "airlock-claude"
	proxyName := "airlock-proxy"
	if opts.ID != "" {
		claudeName = "airlock-claude-" + opts.ID
		proxyName = "airlock-proxy-" + opts.ID
	}
	proxyURL := fmt.Sprintf("http://%s:%d", proxyName, opts.ProxyPort)

	binds := []string{
		fmt.Sprintf("%s:/workspace", opts.Workspace),
	}

	var mounts []mount.Mount
	if opts.VolumeName != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: opts.VolumeName,
			Target: "/home/airlock/.claude",
		})
	} else if opts.ClaudeDir != "" {
		binds = append(binds, fmt.Sprintf("%s:/home/airlock/.claude:ro", opts.ClaudeDir))
	}

	if opts.CACertPath != "" {
		binds = append(binds, fmt.Sprintf("%s:/usr/local/share/ca-certificates/airlock-proxy.crt:ro", opts.CACertPath))
	}
	for _, m := range opts.ShadowMounts {
		binds = append(binds, fmt.Sprintf("%s:%s:ro", m.HostPath, m.ContainerPath))
	}

	return ContainerConfig{
		Image:      opts.Image,
		Name:       claudeName,
		Network:    opts.NetworkName,
		WorkingDir: "/workspace",
		Tty:        true,
		Stdin:      true,
		Binds:      binds,
		Mounts:     mounts,
		Env: []string{
			fmt.Sprintf("HTTP_PROXY=%s", proxyURL),
			fmt.Sprintf("HTTPS_PROXY=%s", proxyURL),
			fmt.Sprintf("http_proxy=%s", proxyURL),
			fmt.Sprintf("https_proxy=%s", proxyURL),
			"NO_PROXY=localhost,127.0.0.1",
			"LANG=C.UTF-8",
		},
		CapDrop: []string{"ALL"},
		Cmd:     []string{"claude", "--dangerouslySkipPermissions"},
	}
}
