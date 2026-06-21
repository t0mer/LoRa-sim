package main

import (
	_ "embed"

	"github.com/spf13/cobra"
)

//go:embed example_config.yaml
var exampleConfig string

func newGenConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gen-config",
		Short: "Print an example configuration file to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Print(exampleConfig)
			return nil
		},
	}
}
