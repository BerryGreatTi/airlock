package container

import "context"

// ContainerInfo holds status information about a single container.
type ContainerInfo struct {
	Name   string
	Status string
	Uptime string
	Error  string
}

// ContainerRuntime abstracts container operations for testability.
type ContainerRuntime interface {
	EnsureNetwork(ctx context.Context, opts NetworkOpts) (string, error)
	RunDetached(ctx context.Context, cfg ContainerConfig) (string, error)
	RunAttached(ctx context.Context, cfg ContainerConfig) error
	Stop(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	RemoveNetwork(ctx context.Context, name string) error
	ConnectNetwork(ctx context.Context, networkID, containerID string) error
	CopyFromContainer(ctx context.Context, containerName, srcPath, dstPath string) error
	WaitForFile(ctx context.Context, containerName, path string, maxRetries int) error
	ListContainers(ctx context.Context, prefix string) ([]ContainerInfo, error)
}
