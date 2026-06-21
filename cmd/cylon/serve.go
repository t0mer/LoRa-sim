package main

import (
	"context"
	"crypto/tls"
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
	"github.com/t0mer/cylon/internal/creds"
	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/gateway"
	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/metrics"
	"github.com/t0mer/cylon/internal/sim"
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

			m := metrics.New()
			hub := api.NewHub()
			publisher := api.NewPublisher(st.Events(), hub, m, logger)

			// Run the gateway + orchestrator when a connection is configured:
			// either a direct LNS URL (mock-lns) or credentials in the volume
			// (real AWS via CUPS bootstrap or LNS-direct).
			var gw *gateway.Gateway
			var orch *sim.Orchestrator
			if gatewayEnabled(cfg, g) {
				var closeGW func()
				gw, closeGW, err = startGateway(ctx, cfg, g, logger)
				if err != nil {
					return err
				}
				defer closeGW()
				orch = sim.New(st, gw.Addr(), publisher, logger)
				defer orch.StopAll()
			} else {
				logger.Info("gateway disabled (no lns_url and no credentials)")
			}

			bindGauges(m, hub, gw, orch)

			a := api.NewAPI(st, hub, orch, gw, version.Version, g.EUI)
			srv := &http.Server{
				Addr:              cfg.Server.HTTPListen,
				Handler:           api.NewRouter(a, spaHandler()),
				ReadHeaderTimeout: 5 * time.Second,
			}
			metricsSrv := &http.Server{
				Addr:              cfg.Server.MetricsListen,
				Handler:           m.Handler(),
				ReadHeaderTimeout: 5 * time.Second,
			}

			errCh := make(chan error, 2)
			go func() {
				logger.Info("cylon serving", "http", cfg.Server.HTTPListen)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errCh <- err
				}
			}()
			go func() {
				logger.Info("metrics serving", "addr", cfg.Server.MetricsListen)
				if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
				_ = metricsSrv.Shutdown(shutdownCtx)
				return srv.Shutdown(shutdownCtx)
			}
		},
	}
}

func bindGauges(m *metrics.Metrics, hub *api.Hub, gw *gateway.Gateway, orch *sim.Orchestrator) {
	m.BindGauge("cylon_ws_clients", "Connected WebSocket clients.", func() float64 {
		return float64(hub.Clients())
	})
	m.BindGauge("cylon_active_tags", "Running simulated tags.", func() float64 {
		if orch == nil {
			return 0
		}
		return float64(len(orch.Running()))
	})
	m.BindGauge("cylon_tag_conns", "Tag TCP connections at the gateway.", func() float64 {
		if gw == nil {
			return 0
		}
		return float64(gw.ConnCount())
	})
}

// gatewayEnabled reports whether a usable connection is configured: a direct LNS
// URL, or credentials in the volume for the configured connection mode.
func gatewayEnabled(cfg *config.Config, g *store.Gateway) bool {
	if cfg.Gateway.LNSURL != "" {
		return true
	}
	c, err := creds.Load(cfg.Gateway.Connection.CredsDir)
	if err != nil {
		return false
	}
	if g.ConnectionMode == "cups" {
		return c.HasCUPS()
	}
	return c.HasTC()
}

// startGateway resolves the LNS connection, connects, and starts the tag-facing
// TCP listener. The returned close func stops the listener and LNS connection.
func startGateway(ctx context.Context, cfg *config.Config, g *store.Gateway, logger *slog.Logger) (*gateway.Gateway, func(), error) {
	url, tlsCfg, err := resolveLNS(ctx, cfg, g, logger)
	if err != nil {
		return nil, nil, err
	}

	lns, err := gateway.ConnectLNSTLS(ctx, url, protocol.Version{
		Station:  "cylon",
		Model:    "cylon-sim",
		Firmware: version.Version,
		Protocol: 2,
	}, tlsCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to LNS: %w", err)
	}

	gw := gateway.New(g.EUI, lns, logger)
	if err := gw.ListenTCP(cfg.Gateway.TCPListen); err != nil {
		lns.Close()
		return nil, nil, err
	}
	go func() {
		if err := lns.Run(gw.OnDnmsg); err != nil && ctx.Err() == nil {
			logger.Error("LNS connection closed", "err", err)
		}
	}()

	return gw, func() {
		gw.Close()
		lns.Close()
	}, nil
}

// resolveLNS determines the LNS WebSocket URL and TLS config. A configured
// lns_url is used directly (plain ws, for mock-lns). Otherwise it uses the
// credentials volume: connection_mode "cups" bootstraps CUPS first (writing back
// tc.*), then both modes connect to the LNS over mutual TLS.
func resolveLNS(ctx context.Context, cfg *config.Config, g *store.Gateway, logger *slog.Logger) (string, *tls.Config, error) {
	if cfg.Gateway.LNSURL != "" {
		return cfg.Gateway.LNSURL, nil, nil
	}

	credsDir := cfg.Gateway.Connection.CredsDir
	c, err := creds.Load(credsDir)
	if err != nil {
		return "", nil, err
	}

	if g.ConnectionMode == "cups" {
		if !c.HasCUPS() {
			return "", nil, fmt.Errorf("connection_mode is cups but no CUPS credentials in %s", credsDir)
		}
		logger.Info("CUPS bootstrap", "uri", c.CupsURI)
		if _, err := gateway.BootstrapCUPS(ctx, credsDir, protocol.CupsRequest{
			Router: g.EUI, Station: "cylon", Model: "cylon-sim", Package: version.Version,
		}); err != nil {
			return "", nil, fmt.Errorf("CUPS bootstrap: %w", err)
		}
		if c, err = creds.Load(credsDir); err != nil {
			return "", nil, err
		}
	}

	if !c.HasTC() {
		return "", nil, fmt.Errorf("no LNS (tc) credentials available in %s", credsDir)
	}
	tlsCfg, err := c.TCTLSConfig()
	if err != nil {
		return "", nil, err
	}
	logger.Info("LNS connection", "uri", c.TcURI, "mode", g.ConnectionMode)
	return c.TcURI, tlsCfg, nil
}
