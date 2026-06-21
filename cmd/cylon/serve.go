package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/t0mer/cylon/internal/api"
	"github.com/t0mer/cylon/internal/config"
	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/gateway"
	"github.com/t0mer/cylon/internal/gateway/protocol"
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

			cipher, err := newCipher(logger)
			if err != nil {
				return err
			}
			st := store.New(database, cipher)
			g, err := st.Gateway().EnsureEUI(cmd.Context(), cfg.Gateway.EUI, cfg.Gateway.EUIPrefix)
			if err != nil {
				return err
			}
			logger.Info("gateway identity", "eui", g.EUI, "region", g.Region)

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// When an LNS URL is configured, run the gateway: connect to the LNS
			// (mock-lns or LNS-direct) and accept tag TCP connections. CUPS and
			// the credential volume arrive in Phase 4.
			if cfg.Gateway.LNSURL != "" {
				closeGW, err := startGateway(ctx, cfg, g.EUI, logger)
				if err != nil {
					return err
				}
				defer closeGW()
			}

			srv := &http.Server{
				Addr:              cfg.Server.HTTPListen,
				Handler:           api.NewRouter(version.Version, g.EUI),
				ReadHeaderTimeout: 5 * time.Second,
			}

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

// startGateway connects the gateway to the LNS and starts its tag-facing TCP
// listener. The returned close func stops the listener and the LNS connection.
func startGateway(ctx context.Context, cfg *config.Config, eui string, logger *slog.Logger) (func(), error) {
	lns, err := gateway.ConnectLNS(ctx, cfg.Gateway.LNSURL, protocol.Version{
		Station:  "cylon",
		Model:    "cylon-sim",
		Firmware: version.Version,
		Protocol: 2,
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to LNS: %w", err)
	}

	gw := gateway.New(eui, lns, logger)
	if err := gw.ListenTCP(cfg.Gateway.TCPListen); err != nil {
		lns.Close()
		return nil, err
	}
	go func() {
		if err := lns.Run(gw.OnDnmsg); err != nil && ctx.Err() == nil {
			logger.Error("LNS connection closed", "err", err)
		}
	}()

	return func() {
		gw.Close()
		lns.Close()
	}, nil
}
