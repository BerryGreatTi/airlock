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
	importFrom  string
	importAll   bool
	importItems string
	importForce bool
)

var defaultImportItems = []string{"CLAUDE.md", "rules", "settings.json", "settings.local.json"}
var optionalImportItems = []string{"plugins", "skills", "history.jsonl", "projects"}

var allowedImportItems = map[string]bool{
	"CLAUDE.md": true, "rules": true, "settings.json": true,
	"settings.local.json": true, "plugins": true, "skills": true,
	"history.jsonl": true, "projects": true,
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage airlock configuration",
}

var configImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import host Claude Code config into the airlock volume",
	Long: `Copies selected files from the host's ~/.claude directory into the
persistent airlock volume. By default imports: CLAUDE.md, rules/,
settings.json, settings.local.json.

Existing files in the volume are skipped unless --force is set.`,
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
		srcDir := importFrom
		if srcDir == "" {
			homeDir, _ := os.UserHomeDir()
			srcDir = filepath.Join(homeDir, ".claude")
		}
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			return fmt.Errorf("source directory does not exist: %s", srcDir)
		}
		var items []string
		if importAll {
			items = append(append(items, defaultImportItems...), optionalImportItems...)
		} else if importItems != "" {
			for _, item := range strings.Split(importItems, ",") {
				items = append(items, strings.TrimSpace(item))
			}
		} else {
			items = append(items, defaultImportItems...)
		}
		for _, item := range items {
			if !allowedImportItems[item] {
				return fmt.Errorf("invalid import item %q: must be one of %v", item, defaultImportItems)
			}
		}
		docker, err := container.NewDocker()
		if err != nil {
			return fmt.Errorf("docker init: %w", err)
		}
		defer docker.Close()
		ctx := context.Background()
		if err := docker.EnsureVolume(ctx, volumeName); err != nil {
			return fmt.Errorf("ensure volume: %w", err)
		}
		var cpParts []string
		for _, item := range items {
			srcPath := filepath.Join("/src", item)
			dstPath := filepath.Join("/dst", item)
			check := fmt.Sprintf("if [ -e %s ]; then ", srcPath)
			if !importForce {
				check += fmt.Sprintf("if [ -e %s ]; then echo 'SKIP %s (exists)'; else cp -a %s %s && echo 'OK %s'; fi",
					dstPath, item, srcPath, dstPath, item)
			} else {
				check += fmt.Sprintf("cp -a %s %s && echo 'OK %s'", srcPath, dstPath, item)
			}
			check += fmt.Sprintf("; else echo 'SKIP %s (not in source)'; fi", item)
			cpParts = append(cpParts, check)
		}
		script := strings.Join(cpParts, " ; ")
		// chown volume root to airlock user, then copy, then chown results
		fullScript := "chown 1001:1001 /dst ; " + script + " ; chown -R 1001:1001 /dst"
		importCfg := container.ContainerConfig{
			Image: cfg.ContainerImage,
			Name:  "airlock-importer",
			User:  "root",
			Binds: []string{
				fmt.Sprintf("%s:/src:ro", srcDir),
				fmt.Sprintf("%s:/dst", volumeName),
			},
			Cmd: []string{"sh", "-c", fullScript},
		}
		fmt.Printf("Importing from %s into volume %s...\n", srcDir, volumeName)
		if err := docker.RunAttached(ctx, importCfg); err != nil {
			if !strings.Contains(err.Error(), "exited with code") {
				return fmt.Errorf("import failed: %w", err)
			}
		}
		fmt.Println("\nSettings imported. Secrets will be encrypted on next container start.")
		return nil
	},
}

func init() {
	configImportCmd.Flags().StringVar(&importFrom, "from", "", "source directory (default: ~/.claude)")
	configImportCmd.Flags().BoolVar(&importAll, "all", false, "import all items including history and projects")
	configImportCmd.Flags().StringVar(&importItems, "items", "", "comma-separated items to import")
	configImportCmd.Flags().BoolVar(&importForce, "force", false, "overwrite existing files in volume")
	configCmd.AddCommand(configImportCmd)
	rootCmd.AddCommand(configCmd)
}
