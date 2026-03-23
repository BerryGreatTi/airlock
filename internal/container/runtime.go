package container

import "context"

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
}
