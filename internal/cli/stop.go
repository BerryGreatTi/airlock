package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/orchestrator"
)

// stopAll enumerates running airlock-claude-* containers, extracts their IDs,
// and calls CleanupSession for each. Falls back to legacy fixed names if no
// ID-based containers are found.
func stopAll(ctx context.Context, runtime container.ContainerRuntime, cfg config.Config) error {
	containers, err := runtime.ListContainers(ctx, claudeContainerPrefix)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	var ids []string
	for _, c := range containers {
		id := extractIDFromContainerName(c.Name, claudeContainerPrefix)
		if id != "" {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		// Fallback: stop legacy fixed-name containers
		orchestrator.CleanupSession(ctx, runtime, cfg, "")
		return nil
	}

	for _, id := range ids {
		orchestrator.CleanupSession(ctx, runtime, cfg, id)
	}
	return nil
}

var stopID string

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop running airlock containers",
	Long: `Stop airlock containers. Use --id to stop a specific workspace,
or omit --id to stop all running workspaces.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg, err := config.Load(".airlock")
		if err != nil {
			cfg = config.Default()
		}
		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()

		if stopID != "" {
			orchestrator.CleanupSession(ctx, docker, cfg, stopID)
		} else {
			if err := stopAll(ctx, docker, cfg); err != nil {
				return err
			}
		}

		fmt.Println("Done.")
		return nil
	},
}

func init() {
	stopCmd.Flags().StringVar(&stopID, "id", "", "workspace ID to stop (default: stop all)")
	rootCmd.AddCommand(stopCmd)
}
