package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/t0mer/cylon/internal/api"
	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/store"
	"github.com/t0mer/cylon/internal/version"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the cylon web app (HTTP server + database)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			logger := setupLogger(cfg.Server.LogLevel)

			database, err := db.Open(cfg.Store.Path)
			if err != nil {
				return err
			}
			defer database.Close()
			if err := db.Migrate(database, "up"); err != nil {
				return err
			}

			st := store.New(database)
			g, err := st.Gateway().EnsureEUI(cmd.Context(), cfg.Gateway.EUI, cfg.Gateway.EUIPrefix)
			if err != nil {
				return err
			}
			logger.Info("gateway identity", "eui", g.EUI, "region", g.Region)

			srv := &http.Server{
				Addr:              cfg.Server.HTTPListen,
				Handler:           api.NewRouter(version.Version, g.EUI),
				ReadHeaderTimeout: 5 * time.Second,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() {
				logger.Info("cylon serving", "http", cfg.Server.HTTPListen)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errCh <- err
				}
			}()

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
				logger.Info("shutting down")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return srv.Shutdown(shutdownCtx)
			}
		},
	}
}
