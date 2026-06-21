// Command tag is a standalone external end-device that connects to a running
// gateway over TCP, performs an OTAA join, and sends data uplinks. It keeps its
// own SQLite session state so DevNonce and frame counters survive restarts.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/secret"
	"github.com/t0mer/cylon/internal/store"
	"github.com/t0mer/cylon/internal/tag"
	"github.com/t0mer/cylon/internal/transport"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	gatewayAddr := pflag.String("gateway", "127.0.0.1:6000", "gateway TCP address")
	storePath := pflag.String("store", "tag.db", "SQLite session store path")
	devEUI := pflag.String("dev-eui", "", "device EUI (16 hex chars, required)")
	joinEUI := pflag.String("join-eui", "", "join EUI (16 hex chars, required)")
	appKey := pflag.String("app-key", "", "AppKey (32 hex chars, required)")
	class := pflag.String("class", "A", "device class (A|B|C)")
	region := pflag.String("region", "EU868", "RF region")
	dr := pflag.Int("dr", 5, "default data rate")
	fport := pflag.Int("fport", 10, "uplink FPort")
	payload := pflag.String("payload", "counter", "payload generator type")
	interval := pflag.Duration("interval", 10*time.Second, "interval between uplinks")
	count := pflag.Int("count", 1, "number of uplinks to send (0 = forever)")
	pflag.Parse()

	if *devEUI == "" || *joinEUI == "" || *appKey == "" {
		return fmt.Errorf("--dev-eui, --join-eui and --app-key are required")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cipher := secret.NewInsecureDisabled()
	if key := os.Getenv("CYLON_DB_KEY"); key != "" {
		c, err := secret.New(key)
		if err != nil {
			return err
		}
		cipher = c
	} else {
		logger.Warn("CYLON_DB_KEY not set; tag session keys stored unencrypted")
	}

	database, err := db.Open(*storePath)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := db.Migrate(database, "up"); err != nil {
		return err
	}
	st := store.New(database, cipher)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tg, err := st.Tags().GetByDevEUI(ctx, *devEUI)
	if errors.Is(err, store.ErrNotFound) {
		tg, err = st.Tags().Create(ctx, store.NewTag{
			DevEUI: *devEUI, JoinEUI: *joinEUI, AppKey: *appKey, Class: *class,
			Region: *region, DefaultDR: *dr, FPort: *fport, PayloadType: *payload, Enabled: true,
		})
	}
	if err != nil {
		return fmt.Errorf("provisioning tag: %w", err)
	}

	dev, err := tag.NewDevice(*tg, st)
	if err != nil {
		return err
	}

	conn, err := transport.Dial(*gatewayAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := tag.NewClient(dev, conn)
	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("tag connection closed", "err", err)
		}
	}()

	if err := client.Hello(); err != nil {
		return err
	}

	sess, err := st.Sessions().Get(ctx, tg.ID)
	if err != nil {
		return err
	}
	if !sess.Joined {
		joinCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := client.Join(joinCtx)
		cancel()
		if err != nil {
			return fmt.Errorf("join: %w", err)
		}
		logger.Info("joined", "dev_eui", *devEUI)
	}

	for i := 0; *count == 0 || i < *count; i++ {
		if _, err := client.SendUplink(ctx, nil); err != nil {
			return fmt.Errorf("uplink: %w", err)
		}
		logger.Info("uplink sent", "n", i+1)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(*interval):
		}
	}
	return nil
}
