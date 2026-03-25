package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/container"
)

const (
	claudeContainerPrefix = "airlock-claude-"
	proxyContainerPrefix  = "airlock-proxy-"
)

// WorkspaceStatus holds the status of a single workspace's containers.
type WorkspaceStatus struct {
	ID        string `json:"id"`
	Container string `json:"container"`
	Proxy     string `json:"proxy"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime,omitempty"`
	Error     string `json:"error,omitempty"`
}

// StatusOutput is the top-level JSON output of the status command.
type StatusOutput struct {
	Workspaces []WorkspaceStatus `json:"workspaces"`
}

// extractIDFromContainerName extracts the workspace ID from a container name
// given a prefix. Returns empty string if the name equals the prefix base
// (legacy name without an ID).
func extractIDFromContainerName(name, prefix string) string {
	if len(name) > len(prefix) {
		return name[len(prefix):]
	}
	return ""
}

// RunStatus queries container state and returns structured status output.
// If filterID is non-empty, only that workspace is included.
func RunStatus(ctx context.Context, runtime container.ContainerRuntime, filterID string) (*StatusOutput, error) {
	claudeContainers, err := runtime.ListContainers(ctx, claudeContainerPrefix)
	if err != nil {
		return nil, fmt.Errorf("list claude containers: %w", err)
	}

	proxyMap := make(map[string]container.ContainerInfo)
	proxyContainers, err := runtime.ListContainers(ctx, proxyContainerPrefix)
	if err != nil {
		return nil, fmt.Errorf("list proxy containers: %w", err)
	}
	for _, p := range proxyContainers {
		id := extractIDFromContainerName(p.Name, proxyContainerPrefix)
		if id != "" {
			proxyMap[id] = p
		}
	}

	output := &StatusOutput{
		Workspaces: []WorkspaceStatus{},
	}

	if filterID != "" && len(claudeContainers) == 0 {
		output.Workspaces = append(output.Workspaces, WorkspaceStatus{
			ID:        filterID,
			Container: claudeContainerPrefix + filterID,
			Proxy:     proxyContainerPrefix + filterID,
			Status:    "not_found",
		})
		return output, nil
	}

	for _, c := range claudeContainers {
		id := extractIDFromContainerName(c.Name, claudeContainerPrefix)
		if id == "" {
			continue
		}
		if filterID != "" && id != filterID {
			continue
		}

		ws := WorkspaceStatus{
			ID:        id,
			Container: c.Name,
			Proxy:     proxyContainerPrefix + id,
			Status:    c.Status,
		}
		if c.Status == "running" {
			ws.Uptime = c.Uptime
		}
		if c.Status == "exited" {
			ws.Error = c.Error
		}

		if proxy, ok := proxyMap[id]; ok {
			if proxy.Status != "running" && c.Status == "running" {
				ws.Status = "degraded"
			}
		}

		output.Workspaces = append(output.Workspaces, ws)
	}

	if filterID != "" && len(output.Workspaces) == 0 {
		output.Workspaces = append(output.Workspaces, WorkspaceStatus{
			ID:        filterID,
			Container: claudeContainerPrefix + filterID,
			Proxy:     proxyContainerPrefix + filterID,
			Status:    "not_found",
		})
	}

	return output, nil
}

var statusID string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of running airlock workspaces",
	Long: `Queries running container state and outputs JSON describing each workspace.
Optionally filter by --id to show a single workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		result, err := RunStatus(ctx, docker, statusID)
		if err != nil {
			return err
		}

		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal status: %w", err)
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusID, "id", "", "filter by workspace ID")
	rootCmd.AddCommand(statusCmd)
}
