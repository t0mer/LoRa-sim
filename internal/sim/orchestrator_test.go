package sim_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brocaar/lorawan"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/gateway"
	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/mocklns"
	"github.com/t0mer/cylon/internal/secret"
	"github.com/t0mer/cylon/internal/sim"
	"github.com/t0mer/cylon/internal/store"
)

type recEmitter struct {
	mu     sync.Mutex
	events []store.Event
}

func (e *recEmitter) Publish(_ context.Context, ev store.Event) {
	e.mu.Lock()
	e.events = append(e.events, ev)
	e.mu.Unlock()
}

func (e *recEmitter) count(kind string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, ev := range e.events {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestOrchestratorJoinAllAndBurst(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	appKey := lorawan.AES128Key{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	// mock-lns
	lnsSrv := mocklns.New(mocklns.Config{
		Region: "EU863", NetID: lorawan.NetID{0, 0, 1}, AppKey: appKey,
		DevAddr: lorawan.DevAddr{1, 2, 3, 4}, Logger: quiet(),
	})
	httpSrv := httptest.NewServer(lnsSrv.Handler())
	defer httpSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")

	// gateway
	lns, err := gateway.ConnectLNS(ctx, wsURL, protocol.Version{Station: "cylon", Protocol: 2})
	if err != nil {
		t.Fatalf("ConnectLNS: %v", err)
	}
	defer lns.Close()
	gw := gateway.New("0000000000000001", lns, quiet())
	if err := gw.ListenTCP("127.0.0.1:0"); err != nil {
		t.Fatalf("ListenTCP: %v", err)
	}
	defer gw.Close()
	go func() { _ = lns.Run(gw.OnDnmsg) }()

	// store with N tags sharing the AppKey
	database, _ := db.Open(filepath.Join(t.TempDir(), "c.db"))
	defer database.Close()
	if err := db.Migrate(database, "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cipher, _ := secret.New("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	st := store.New(database, cipher)

	const N = 50
	for i := 0; i < N; i++ {
		_, err := st.Tags().Create(ctx, store.NewTag{
			DevEUI: fmt.Sprintf("01010101010101%02x", i), JoinEUI: "0202020202020202",
			AppKey: hex.EncodeToString(appKey[:]), Class: "A", Region: "EU868",
			DefaultDR: 5, FPort: 10, PayloadType: "counter", Enabled: true,
		})
		if err != nil {
			t.Fatalf("create tag %d: %v", i, err)
		}
	}

	emit := &recEmitter{}
	orch := sim.New(st, gw.Addr(), emit, quiet())
	defer orch.StopAll()

	// JoinAll: every tag should join in parallel.
	started, errs := orch.JoinAll(ctx)
	if errs != 0 || started != N {
		t.Fatalf("JoinAll started=%d errs=%d, want %d/0", started, errs, N)
	}
	if got := len(orch.Running()); got != N {
		t.Errorf("running tags = %d, want %d", got, N)
	}
	if joins := emit.count("join"); joins != N {
		t.Errorf("join events = %d, want %d", joins, N)
	}

	// Burst: 100 uplinks across the fleet, 16 in flight.
	if err := orch.Burst(ctx, 100, 16); err != nil {
		t.Fatalf("Burst: %v", err)
	}
	if data := emit.count("data"); data != 100 {
		t.Errorf("data uplink events = %d, want 100", data)
	}

	// The LNS should have received the burst uplinks.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(lnsSrv.Uplinks()) >= 100 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("mock-lns received %d uplinks, want >= 100", len(lnsSrv.Uplinks()))
}
