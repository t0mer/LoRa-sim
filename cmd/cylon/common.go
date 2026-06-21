package main

import (
	"github.com/spf13/cobra"

	"github.com/t0mer/cylon/internal/config"
)

// loadConfig resolves the --config flag and loads the bootstrap configuration.
func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	path, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, err
	}
	return config.Load(path)
}
