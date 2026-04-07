package cli

import "github.com/spf13/cobra"

var secretEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment-variable secrets",
	Long: `Register, list, show, and remove individual environment variable
secrets. Stored encrypted in .airlock/config.yaml; injected into the
agent container as NAME=ENC[age:...]; substituted at the proxy
boundary on outbound HTTP calls.`,
}

func init() {
	secretCmd.AddCommand(secretEnvCmd)
}
