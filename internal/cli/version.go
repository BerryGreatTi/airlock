package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print airlock version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("airlock %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
