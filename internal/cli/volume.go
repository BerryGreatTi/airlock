package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
)

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Manage the persistent Claude Code state volume",
}

var volumeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show volume status",
	RunE: func(cmd *cobra.Command, args []string) error {
		airlockDir := ".airlock"
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config (run 'airlock init' first): %w", err)
		}
		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}
		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()
		ctx := context.Background()
		err = docker.EnsureVolume(ctx, volumeName)
		if err != nil {
			fmt.Printf("Volume: %s (not available: %v)\n", volumeName, err)
			return nil
		}
		fmt.Printf("Volume: %s (ready)\n", volumeName)
		return nil
	},
}

var volumeResetConfirm bool

var volumeResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete and recreate the volume (destroys all state including OAuth tokens)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !volumeResetConfirm {
			return fmt.Errorf("this will delete all Claude Code state (OAuth tokens, history, memory).\nRe-run with --confirm to proceed")
		}
		airlockDir := ".airlock"
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}
		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()
		ctx := context.Background()

		// Remove helper containers that may hold references to the volume
		for _, name := range []string{"airlock-importer", "airlock-exporter"} {
			docker.Remove(ctx, name)
		}

		fmt.Printf("Removing volume %s...\n", volumeName)
		if err := docker.RemoveVolume(ctx, volumeName); err != nil {
			fmt.Printf("Warning: remove failed (volume may not exist): %v\n", err)
		}
		if err := docker.EnsureVolume(ctx, volumeName); err != nil {
			return fmt.Errorf("recreate volume: %w", err)
		}
		fmt.Printf("Volume %s has been reset.\n", volumeName)
		return nil
	},
}

func init() {
	volumeResetCmd.Flags().BoolVar(&volumeResetConfirm, "confirm", false, "confirm destructive reset")
	volumeCmd.AddCommand(volumeStatusCmd)
	volumeCmd.AddCommand(volumeResetCmd)
	rootCmd.AddCommand(volumeCmd)
}
