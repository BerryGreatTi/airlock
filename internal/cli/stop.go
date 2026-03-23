package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
	"github.com/taeikkim92/airlock/internal/orchestrator"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop running airlock containers",
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
		orchestrator.CleanupSession(ctx, docker, cfg)
		fmt.Println("Done.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
