package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var secretEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment-variable secrets",
	Long: `Register, list, show, and remove individual environment variable
secrets. Stored encrypted in .airlock/config.yaml; injected into the
agent container as NAME=ENC[age:...]; substituted at the proxy
boundary on outbound HTTP calls.`,
}

// printWithTrailingNewline writes out to stdout, appending a newline
// if the last byte is not already one. Shared by the list and show
// cobra wrappers.
func printWithTrailingNewline(out []byte) {
	fmt.Print(string(out))
	if len(out) > 0 && out[len(out)-1] != '\n' {
		fmt.Println()
	}
}

func init() {
	secretCmd.AddCommand(secretEnvCmd)
}
