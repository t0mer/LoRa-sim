package main

import (
	"github.com/spf13/cobra"
)

// newRootCmd builds the cobra command tree for the cylon binary.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "cylon",
		Short:         "Cylon — a LoRaWAN simulator for AWS IoT Core for LoRaWAN",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Persistent config-file flag shared by all subcommands.
	root.PersistentFlags().StringP("config", "c", "", "path to config file (YAML)")

	root.AddCommand(
		newVersionCmd(),
		newGenConfigCmd(),
		newMigrateCmd(),
		newGatewayEUICmd(),
	)

	return root
}
