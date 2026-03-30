package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/taeikkim92/airlock/internal/config"
	"github.com/taeikkim92/airlock/internal/container"
)

var (
	exportTo    string
	exportItems string
)

var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export airlock volume config to a host directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		airlockDir := ".airlock"
		cfg, err := config.Load(airlockDir)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = "airlock-claude-home"
		}
		dstDir := exportTo
		if dstDir == "" {
			homeDir, _ := os.UserHomeDir()
			dstDir = filepath.Join(homeDir, "airlock-claude-export")
		}
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("create export directory: %w", err)
		}
		items := defaultImportItems
		if exportItems != "" {
			items = strings.Split(exportItems, ",")
			for i, item := range items {
				items[i] = strings.TrimSpace(item)
			}
		}
		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()
		ctx := context.Background()
		var cpParts []string
		for _, item := range items {
			srcPath := filepath.Join("/src", item)
			dstPath := filepath.Join("/dst", item)
			cpParts = append(cpParts, fmt.Sprintf("if [ -e %s ]; then cp -a %s %s && echo 'OK %s'; else echo 'SKIP %s'; fi", srcPath, srcPath, dstPath, item, item))
		}
		script := strings.Join(cpParts, " ; ")
		exportCfg := container.ContainerConfig{
			Image: cfg.ContainerImage,
			Name:  "airlock-exporter",
			Binds: []string{
				fmt.Sprintf("%s:/src:ro", volumeName),
				fmt.Sprintf("%s:/dst", dstDir),
			},
			Cmd: []string{"sh", "-c", script},
		}
		fmt.Printf("Exporting from volume %s to %s...\n", volumeName, dstDir)
		if err := docker.RunAttached(ctx, exportCfg); err != nil {
			if !strings.Contains(err.Error(), "exited with code") {
				return fmt.Errorf("export failed: %w", err)
			}
		}
		fmt.Printf("\nExported to %s\n", dstDir)
		return nil
	},
}

func init() {
	configExportCmd.Flags().StringVar(&exportTo, "to", "", "destination directory (default: ~/airlock-claude-export/)")
	configExportCmd.Flags().StringVar(&exportItems, "items", "", "comma-separated items to export")
	configCmd.AddCommand(configExportCmd)
}
