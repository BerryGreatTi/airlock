package container

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Docker implements ContainerRuntime using the Docker Engine SDK.
type Docker struct {
	client *client.Client
}

var _ ContainerRuntime = (*Docker)(nil)

// NewDocker creates a new Docker runtime using environment configuration.
func NewDocker() (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &Docker{client: cli}, nil
}

// Close releases the Docker client resources.
func (d *Docker) Close() error {
	return d.client.Close()
}

// EnsureNetwork creates a Docker network if it does not already exist.
// Returns the network ID.
func (d *Docker) EnsureNetwork(ctx context.Context, opts NetworkOpts) (string, error) {
	networks, err := d.client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list networks: %w", err)
	}
	for _, n := range networks {
		if n.Name == opts.Name {
			return n.ID, nil
		}
	}
	resp, err := d.client.NetworkCreate(ctx, opts.Name, network.CreateOptions{
		Driver:   opts.Driver,
		Internal: opts.Internal,
	})
	if err != nil {
		return "", fmt.Errorf("create network: %w", err)
	}
	return resp.ID, nil
}

// RunDetached creates and starts a container in the background.
// Returns the container ID.
func (d *Docker) RunDetached(ctx context.Context, cfg ContainerConfig) (string, error) {
	hostConfig := &dockercontainer.HostConfig{
		Binds:   cfg.Binds,
		CapDrop: cfg.CapDrop,
	}
	containerConfig := &dockercontainer.Config{
		Image:      cfg.Image,
		Env:        cfg.Env,
		WorkingDir: cfg.WorkingDir,
		Tty:        cfg.Tty,
		OpenStdin:  cfg.Stdin,
		Cmd:        cfg.Cmd,
	}
	networkConfig := &network.NetworkingConfig{}
	if cfg.Network != "" {
		networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			cfg.Network: {},
		}
	}

	// Remove any pre-existing container with the same name.
	d.client.ContainerRemove(ctx, cfg.Name, dockercontainer.RemoveOptions{Force: true})

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, cfg.Name)
	if err != nil {
		return "", fmt.Errorf("create container %s: %w", cfg.Name, err)
	}
	if err := d.client.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		return "", fmt.Errorf("start container %s: %w", cfg.Name, err)
	}
	return resp.ID, nil
}

// RunAttached creates and starts a container with stdin/stdout/stderr attached.
// Blocks until the container exits.
func (d *Docker) RunAttached(ctx context.Context, cfg ContainerConfig) error {
	hostConfig := &dockercontainer.HostConfig{
		Binds:   cfg.Binds,
		CapDrop: cfg.CapDrop,
	}
	containerConfig := &dockercontainer.Config{
		Image:        cfg.Image,
		Env:          cfg.Env,
		WorkingDir:   cfg.WorkingDir,
		Tty:          cfg.Tty,
		OpenStdin:    cfg.Stdin,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cfg.Cmd,
	}
	networkConfig := &network.NetworkingConfig{}
	if cfg.Network != "" {
		networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			cfg.Network: {},
		}
	}

	// Remove any pre-existing container with the same name.
	d.client.ContainerRemove(ctx, cfg.Name, dockercontainer.RemoveOptions{Force: true})

	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, cfg.Name)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	attachResp, err := d.client.ContainerAttach(ctx, resp.ID, dockercontainer.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("attach container: %w", err)
	}
	defer attachResp.Close()

	if err := d.client.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	go func() {
		_, _ = io.Copy(attachResp.Conn, os.Stdin)
	}()

	if cfg.Tty {
		_, _ = io.Copy(os.Stdout, attachResp.Reader)
	} else {
		_, _ = stdcopy.StdCopy(os.Stdout, os.Stderr, attachResp.Reader)
	}

	statusCh, errCh := d.client.ContainerWait(ctx, resp.ID, dockercontainer.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("wait container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with code %d", status.StatusCode)
		}
	}
	return nil
}

// Stop sends a stop signal to a running container and waits for it to exit.
func (d *Docker) Stop(ctx context.Context, name string) error {
	timeout := 10
	return d.client.ContainerStop(ctx, name, dockercontainer.StopOptions{Timeout: &timeout})
}

// Remove force-removes a container.
func (d *Docker) Remove(ctx context.Context, name string) error {
	return d.client.ContainerRemove(ctx, name, dockercontainer.RemoveOptions{Force: true})
}

// RemoveNetwork removes a Docker network by name.
func (d *Docker) RemoveNetwork(ctx context.Context, name string) error {
	return d.client.NetworkRemove(ctx, name)
}

// ConnectNetwork connects a container to a network.
func (d *Docker) ConnectNetwork(ctx context.Context, networkID, containerID string) error {
	return d.client.NetworkConnect(ctx, networkID, containerID, nil)
}

// CopyFromContainer copies a single file from a container to the local filesystem.
// The Docker API returns a tar stream; this extracts the first regular file.
func (d *Docker) CopyFromContainer(ctx context.Context, containerName, srcPath, dstPath string) error {
	reader, _, err := d.client.CopyFromContainer(ctx, containerName, srcPath)
	if err != nil {
		return fmt.Errorf("copy from container: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if header.Typeflag == tar.TypeReg {
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return fmt.Errorf("create parent dir: %w", err)
			}
			outFile, err := os.Create(dstPath)
			if err != nil {
				return fmt.Errorf("create destination file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("write file: %w", err)
			}
			outFile.Close()
			return nil
		}
	}
	return fmt.Errorf("file not found in tar stream: %s", srcPath)
}

// WaitForFile polls a container until a file exists or maxRetries is exhausted.
func (d *Docker) WaitForFile(ctx context.Context, containerName, path string, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		execCfg := dockercontainer.ExecOptions{
			Cmd:          []string{"test", "-f", path},
			AttachStdout: false,
			AttachStderr: false,
		}
		execResp, err := d.client.ContainerExecCreate(ctx, containerName, execCfg)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if err := d.client.ContainerExecStart(ctx, execResp.ID, dockercontainer.ExecStartOptions{}); err != nil {
			time.Sleep(time.Second)
			continue
		}
		inspect, err := d.client.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if inspect.ExitCode == 0 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("file %s not found in container %s after %d retries", path, containerName, maxRetries)
}

// EnsureVolume creates a Docker volume with the given name if it does not already exist.
func (d *Docker) EnsureVolume(ctx context.Context, name string) error {
	_, err := d.client.VolumeInspect(ctx, name)
	if err == nil {
		return nil
	}
	_, err = d.client.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}
	return nil
}

// RemoveVolume force-removes a Docker volume by name.
func (d *Docker) RemoveVolume(ctx context.Context, name string) error {
	return d.client.VolumeRemove(ctx, name, true)
}

// ReadFromVolume mounts the named volume into a temporary container and copies
// filePath out to dstPath on the host. Returns os.ErrNotExist if the file is
// not present in the volume.
func (d *Docker) ReadFromVolume(ctx context.Context, volumeName, filePath, dstPath string) error {
	tmpName := "airlock-vol-reader"
	containerConfig := &dockercontainer.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sleep", "30"},
		User:  "1001:1001",
	}
	hostConfig := &dockercontainer.HostConfig{
		Binds:      []string{fmt.Sprintf("%s:/vol:ro", volumeName)},
		AutoRemove: true,
	}
	d.client.ContainerRemove(ctx, tmpName, dockercontainer.RemoveOptions{Force: true})
	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, tmpName)
	if err != nil {
		return fmt.Errorf("create reader container: %w", err)
	}
	defer func() {
		d.client.ContainerStop(ctx, resp.ID, dockercontainer.StopOptions{})
	}()
	if err := d.client.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		return fmt.Errorf("start reader container: %w", err)
	}
	srcPath := filepath.Join("/vol", filePath)
	err = d.CopyFromContainer(ctx, tmpName, srcPath, dstPath)
	if err != nil {
		if strings.Contains(err.Error(), "No such") || strings.Contains(err.Error(), "not found") {
			return os.ErrNotExist
		}
		return err
	}
	return nil
}

// ListContainers returns info about containers whose name starts with the given prefix.
func (d *Docker) ListContainers(ctx context.Context, prefix string) ([]ContainerInfo, error) {
	containers, err := d.client.ContainerList(ctx, dockercontainer.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	var result []ContainerInfo
	for _, c := range containers {
		for _, name := range c.Names {
			clean := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(clean, prefix) {
				info := ContainerInfo{Name: clean, Status: c.State}
				if c.State == "running" {
					info.Uptime = c.Status
				}
				if c.State == "exited" {
					info.Error = c.Status
				}
				result = append(result, info)
			}
		}
	}
	return result, nil
}
