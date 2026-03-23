package container

import (
	"fmt"
	"strings"
)

// RunOpts holds all options needed to launch the airlock container pair.
type RunOpts struct {
	Workspace        string
	Image            string
	ProxyImage       string
	NetworkName      string
	EnvFilePath      string
	MappingPath      string
	ClaudeDir        string
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
	passthroughStr := strings.Join(opts.PassthroughHosts, ",")
	return ContainerConfig{
		Image:   opts.ProxyImage,
		Name:    "airlock-proxy",
		Network: opts.NetworkName,
		Binds:   []string{fmt.Sprintf("%s:/run/airlock/mapping.json:ro", opts.MappingPath)},
		Env: []string{
			"AIRLOCK_MAPPING_PATH=/run/airlock/mapping.json",
			fmt.Sprintf("AIRLOCK_PASSTHROUGH_HOSTS=%s", passthroughStr),
		},
		CapDrop: []string{"ALL"},
	}
}

// BuildClaudeConfig returns a ContainerConfig for the AI agent container.
func BuildClaudeConfig(opts RunOpts) ContainerConfig {
	proxyURL := fmt.Sprintf("http://airlock-proxy:%d", opts.ProxyPort)

	binds := []string{
		fmt.Sprintf("%s:/workspace", opts.Workspace),
		fmt.Sprintf("%s:/home/airlock/.claude:ro", opts.ClaudeDir),
	}
	if opts.EnvFilePath != "" {
		binds = append(binds, fmt.Sprintf("%s:/run/airlock/env.enc:ro", opts.EnvFilePath))
	}
	if opts.CACertPath != "" {
		binds = append(binds, fmt.Sprintf("%s:/usr/local/share/ca-certificates/airlock-proxy.crt:ro", opts.CACertPath))
	}

	return ContainerConfig{
		Image:      opts.Image,
		Name:       "airlock-claude",
		Network:    opts.NetworkName,
		WorkingDir: "/workspace",
		Tty:        true,
		Stdin:      true,
		Binds:      binds,
		Env: []string{
			fmt.Sprintf("HTTP_PROXY=%s", proxyURL),
			fmt.Sprintf("HTTPS_PROXY=%s", proxyURL),
			fmt.Sprintf("http_proxy=%s", proxyURL),
			fmt.Sprintf("https_proxy=%s", proxyURL),
			"NO_PROXY=localhost,127.0.0.1",
		},
		CapDrop: []string{"ALL"},
		Cmd:     []string{"claude", "--dangerouslySkipPermissions"},
	}
}
