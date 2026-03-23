package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "airlock",
	Short: "Run Claude Code in an isolated container with encrypted secrets",
	Long: `Airlock creates a containerized environment for Claude Code where
secrets are always encrypted. A transparent proxy at the network boundary
decrypts secrets only when they leave the container.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
