package cli

import "github.com/spf13/cobra"

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secret files",
	Long:  `Register, list, encrypt, decrypt, and remove secret files for a workspace.`,
}

func init() {
	rootCmd.AddCommand(secretCmd)
}
