package mocklns_test

import (
	"bytes"
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

func TestClassCUnsolicitedDownlink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	appKey := lorawan.AES128Key{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	devAddr := lorawan.DevAddr{1, 2, 3, 4}

	lnsSrv := mocklns.New(mocklns.Config{
		Region: "EU863", NetID: lorawan.NetID{0, 0, 1}, AppKey: appKey, DevAddr: devAddr, Logger: quiet,
	})
	httpSrv := httptest.NewServer(lnsSrv.Handler())
	defer httpSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")

	lns, err := gateway.ConnectLNS(ctx, wsURL, protocol.Version{Station: "cylon", Protocol: 2})
	if err != nil {
		t.Fatalf("ConnectLNS: %v", err)
	}
	defer lns.Close()
	gw := gateway.New("0000000000000001", lns, quiet)
	if err := gw.ListenTCP("127.0.0.1:0"); err != nil {
		t.Fatalf("ListenTCP: %v", err)
	}
	defer gw.Close()
	go func() { _ = lns.Run(gw.OnDnmsg) }()

	database, _ := db.Open(filepath.Join(t.TempDir(), "c.db"))
	defer database.Close()
	db.Migrate(database, "up")
	cipher, _ := secret.New("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	st := store.New(database, cipher)
	tagRow, _ := st.Tags().Create(ctx, store.NewTag{
		DevEUI: "0101010101010101", JoinEUI: "0202020202020202",
		AppKey: hex.EncodeToString(appKey[:]), Class: "C", Region: "EU868",
		DefaultDR: 5, FPort: 10, PayloadType: "counter", Enabled: true,
	})
	dev, _ := tag.NewDevice(*tagRow, st)

	conn, _ := transport.Dial(gw.Addr())
	defer conn.Close()
	client := tag.NewClient(dev, conn)

	gotDown := make(chan *tag.Downlink, 1)
	client.DownlinkHook = func(dl *tag.Downlink) { gotDown <- dl }
	go func() { _ = client.Run(ctx) }()
	client.Hello()

	joinCtx, jc := context.WithTimeout(ctx, 5*time.Second)
	defer jc()
	if err := client.Join(joinCtx); err != nil {
		t.Fatalf("Join: %v", err)
	}

	// Unsolicited Class C downlink — no preceding uplink.
	want := []byte("classc!")
	// PushClassC needs the gateway's hello registration to have landed.
	time.Sleep(50 * time.Millisecond)
	if err := lnsSrv.PushClassC(ctx, "0101010101010101", 10, want); err != nil {
		t.Fatalf("PushClassC: %v", err)
	}

	select {
	case dl := <-gotDown:
		if !bytes.Equal(dl.Payload, want) {
			t.Errorf("Class C payload = %q, want %q", dl.Payload, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("tag did not receive the unsolicited Class C downlink")
	}
}
