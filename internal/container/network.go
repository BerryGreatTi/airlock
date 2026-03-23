package container

// NetworkOpts describes a Docker network to create.
type NetworkOpts struct {
	Name     string
	Driver   string
	Internal bool
}

// NetworkConfig returns the default network configuration for airlock.
// The network is internal (no external access) by design.
func NetworkConfig(name string) NetworkOpts {
	return NetworkOpts{
		Name:     name,
		Driver:   "bridge",
		Internal: true,
	}
}
