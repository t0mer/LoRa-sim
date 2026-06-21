package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/t0mer/cylon/internal/config"
	"github.com/t0mer/cylon/internal/secret"
)

// loadConfig resolves the --config flag and loads the bootstrap configuration.
func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	path, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, err
	}
	return config.Load(path)
}

// newCipher builds the at-rest secret cipher from CYLON_DB_KEY. When unset it
// returns a pass-through cipher and logs a loud warning (dev only). logger may
// be nil for commands that do not touch sensitive columns.
func newCipher(logger *slog.Logger) (*secret.Cipher, error) {
	key := os.Getenv("CYLON_DB_KEY")
	if key == "" {
		if logger != nil {
			logger.Warn("CYLON_DB_KEY is not set; secrets (AppKey, session keys) are stored UNENCRYPTED — dev use only")
		}
		return secret.NewInsecureDisabled(), nil
	}
	return secret.New(key)
}
