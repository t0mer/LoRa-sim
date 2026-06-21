package tag_test

import (
	"context"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brocaar/lorawan"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/gateway"
	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/mocklns"
	"github.com/t0mer/cylon/internal/secret"
	"github.com/t0mer/cylon/internal/store"
	"github.com/t0mer/cylon/internal/tag"
	"github.com/t0mer/cylon/internal/transport"
)

const e2eKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func e2eStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database, "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cipher, _ := secret.New(e2eKey)
	return store.New(database, cipher)
}

// TestEndToEndJoinAndUplinkOverTCP exercises the full Phase 2 path: a tag dials
// the gateway over TCP, the gateway speaks Basic Station to mock-lns, the tag
// completes an OTAA join via a routed downlink, and a data uplink is relayed all
// the way to the LNS.
func TestEndToEndJoinAndUplinkOverTCP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	appKey := lorawan.AES128Key{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	devAddr := lorawan.DevAddr{1, 2, 3, 4}

	// 1. mock-lns
	lnsSrv := mocklns.New(mocklns.Config{
		Region: "EU863", NetID: lorawan.NetID{0, 0, 1}, AppKey: appKey, DevAddr: devAddr,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	httpSrv := httptest.NewServer(lnsSrv.Handler())
	defer httpSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")

	// 2. gateway -> mock-lns, plus tag-facing TCP listener
	lns, err := gateway.ConnectLNS(ctx, wsURL, protocol.Version{Station: "cylon", Protocol: 2})
	if err != nil {
		t.Fatalf("ConnectLNS: %v", err)
	}
	defer lns.Close()

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := gateway.New("0000000000000001", lns, quiet)
	if err := gw.ListenTCP("127.0.0.1:0"); err != nil {
		t.Fatalf("ListenTCP: %v", err)
	}
	defer gw.Close()
	go func() { _ = lns.Run(gw.OnDnmsg) }()

	// 3. tag device backed by SQLite
	st := e2eStore(t)
	tagRow, err := st.Tags().Create(ctx, store.NewTag{
		DevEUI: "0101010101010101", JoinEUI: "0202020202020202",
		AppKey: hex.EncodeToString(appKey[:]), Class: "A", Region: "EU868",
		DefaultDR: 5, FPort: 10, PayloadType: "counter", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create tag: %v", err)
	}
	dev, err := tag.NewDevice(*tagRow, st)
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}

	// 4. tag dials the gateway over TCP
	conn, err := transport.Dial(gw.Addr())
	if err != nil {
		t.Fatalf("Dial gateway: %v", err)
	}
	defer conn.Close()
	client := tag.NewClient(dev, conn)
	go func() { _ = client.Run(ctx) }()
	if err := client.Hello(); err != nil {
		t.Fatalf("Hello: %v", err)
	}

	// 5. OTAA join over the full path
	joinCtx, joinCancel := context.WithTimeout(ctx, 5*time.Second)
	defer joinCancel()
	if err := client.Join(joinCtx); err != nil {
		t.Fatalf("Join: %v", err)
	}

	sess, err := st.Sessions().Get(ctx, tagRow.ID)
	if err != nil || !sess.Joined {
		t.Fatalf("session not joined: %v joined=%v", err, sess.Joined)
	}
	if sess.DevAddr != "01020304" {
		t.Errorf("session DevAddr = %q, want 01020304", sess.DevAddr)
	}

	// 6. data uplink relayed to the LNS
	if _, err := client.SendUplink(ctx, nil); err != nil {
		t.Fatalf("SendUplink: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ups := lnsSrv.Uplinks(); len(ups) >= 1 {
			if ups[0].FPort != 10 {
				t.Errorf("relayed updf FPort = %d, want 10", ups[0].FPort)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("mock-lns never received the relayed data uplink")
}
