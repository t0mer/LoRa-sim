package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/store"
)

func newGatewayEUICmd() *cobra.Command {
	var set string
	cmd := &cobra.Command{
		Use:   "gateway-eui",
		Short: "Show or override the gateway EUI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			database, err := db.Open(cfg.Store.Path)
			if err != nil {
				return err
			}
			defer database.Close()
			if err := db.Migrate(database, "up"); err != nil {
				return err
			}

			cipher, err := newCipher(nil)
			if err != nil {
				return err
			}
			repo := store.New(database, cipher).Gateway()
			ctx := context.Background()

			var g *store.Gateway
			if set != "" {
				g, err = repo.SetEUI(ctx, set)
			} else {
				g, err = repo.EnsureEUI(ctx, cfg.Gateway.EUI, cfg.Gateway.EUIPrefix)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), g.EUI)
			return nil
		},
	}
	cmd.Flags().StringVar(&set, "set", "", "set the gateway EUI to this value (16 hex chars)")
	return cmd
}
