package main

import (
	"github.com/spf13/cobra"

	"github.com/t0mer/cylon/internal/db"
)

func newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "migrate [up|down|status]",
		Short:     "Run database migrations",
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"up", "down", "status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			database, err := db.Open(cfg.Store.Path)
			if err != nil {
				return err
			}
			defer database.Close()

			command := "up"
			if len(args) == 1 {
				command = args[0]
			}
			return db.Migrate(database, command)
		},
	}
}
