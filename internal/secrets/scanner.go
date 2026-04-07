package secrets

import "fmt"

// Scanner finds and encrypts secrets in a specific config format.
type Scanner interface {
	Name() string
	Scan(opts ScanOpts) (*ScanResult, error)
}

// ScanOpts holds parameters shared by all scanners.
type ScanOpts struct {
	Workspace            string
	HomeDir              string
	PublicKey            string
	PrivateKey           string
	TmpDir               string
	VolumeSettingsDir    string // when set, read global settings from this dir instead of HomeDir
	ContainerWorkDir     string // container-side workspace path (e.g., /workspace/my-app)
}

// ShadowMount describes a file-level Docker bind mount that shadows
// a plaintext file with its encrypted counterpart.
type ShadowMount struct {
	HostPath      string // processed file in tmpDir
	ContainerPath string // container path to shadow
}

// EnvVar is an environment variable to inject into the agent
// container. Value is always an ENC[age:...] ciphertext; the proxy
// substitutes it back to plaintext on the wire when the agent makes
// outbound HTTP calls to non-passthrough hosts.
type EnvVar struct {
	Name  string
	Value string
}

// ScanResult holds the outputs from a scanner: shadow mounts, proxy
// mapping, and environment variables to inject into the agent container.
type ScanResult struct {
	Mounts  []ShadowMount
	Mapping map[string]string // ENC[age:...] -> plaintext
	Env     []EnvVar
}

// ScanAll runs all scanners and merges their results.
func ScanAll(scanners []Scanner, opts ScanOpts) (*ScanResult, error) {
	merged := &ScanResult{Mapping: make(map[string]string)}
	for _, s := range scanners {
		result, err := s.Scan(opts)
		if err != nil {
			return nil, fmt.Errorf("scanner %s: %w", s.Name(), err)
		}
		if result == nil {
			continue
		}
		merged.Mounts = append(merged.Mounts, result.Mounts...)
		for k, v := range result.Mapping {
			merged.Mapping[k] = v
		}
		merged.Env = append(merged.Env, result.Env...)
	}
	return merged, nil
}
