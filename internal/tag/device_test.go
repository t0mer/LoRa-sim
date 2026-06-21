package tag

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/brocaar/lorawan"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/secret"
	"github.com/t0mer/cylon/internal/store"
)

const deviceTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func newDeviceStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database, "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cipher, err := secret.New(deviceTestKey)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	return store.New(database, cipher)
}

// TestFullJoinUplinkDownlinkCycle drives a device through the complete Phase 1
// PHY cycle — OTAA join, an uplink, and a downlink — with the test playing the
// network. All session state is persisted in SQLite.
func TestFullJoinUplinkDownlinkCycle(t *testing.T) {
	ctx := context.Background()
	st := newDeviceStore(t)

	const appKeyHex = "000102030405060708090a0b0c0d0e0f"
	appKey, _ := parseAES128(appKeyHex)

	tagRow, err := st.Tags().Create(ctx, store.NewTag{
		DevEUI:      "0102030405060708",
		JoinEUI:     "0807060504030201",
		AppKey:      appKeyHex,
		Class:       "A",
		Region:      "EU868",
		SubBand:     2,
		DefaultDR:   5,
		FPort:       10,
		PayloadType: "counter",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("Create tag: %v", err)
	}

	dev, err := NewDevice(*tagRow, st)
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}

	// --- JOIN ---
	req, devNonce, err := dev.BuildJoinRequest(ctx)
	if err != nil {
		t.Fatalf("BuildJoinRequest: %v", err)
	}
	if devNonce != 1 {
		t.Errorf("first DevNonce = %d, want 1", devNonce)
	}

	var jphy lorawan.PHYPayload
	if err := jphy.UnmarshalBinary(req); err != nil {
		t.Fatalf("unmarshal join-request: %v", err)
	}
	if ok, err := jphy.ValidateUplinkJoinMIC(appKey); err != nil || !ok {
		t.Fatalf("join-request MIC invalid: ok=%v err=%v", ok, err)
	}
	jr := jphy.MACPayload.(*lorawan.JoinRequestPayload)

	netID := lorawan.NetID{0, 0, 1}
	joinNonce := lorawan.JoinNonce(0x000123)
	accept := buildJoinAccept(t, appKey, jr.JoinEUI, devNonce, lorawan.JoinAcceptPayload{
		JoinNonce:  joinNonce,
		HomeNetID:  netID,
		DevAddr:    lorawan.DevAddr{1, 2, 3, 4},
		DLSettings: lorawan.DLSettings{RX2DataRate: 0, RX1DROffset: 0},
		RXDelay:    1,
	})
	if err := dev.CompleteJoin(ctx, devNonce, accept); err != nil {
		t.Fatalf("CompleteJoin: %v", err)
	}

	sess, err := st.Sessions().Get(ctx, tagRow.ID)
	if err != nil || !sess.Joined {
		t.Fatalf("session not joined after CompleteJoin: %v joined=%v", err, sess.Joined)
	}

	// Network-side session keys (independent derivation) for parsing/building.
	nwk, _ := DeriveNwkSKey(appKey, netID, joinNonce, lorawan.DevNonce(devNonce))
	app, _ := DeriveAppSKey(appKey, netID, joinNonce, lorawan.DevNonce(devNonce))

	// --- UPLINK ---
	up, fcnt, err := dev.BuildUplink(ctx, nil)
	if err != nil {
		t.Fatalf("BuildUplink: %v", err)
	}
	if fcnt != 0 {
		t.Errorf("first uplink FCnt = %d, want 0", fcnt)
	}
	var uphy lorawan.PHYPayload
	if err := uphy.UnmarshalBinary(up); err != nil {
		t.Fatalf("unmarshal uplink: %v", err)
	}
	if ok, err := uphy.ValidateUplinkDataMIC(lorawan.LoRaWAN1_0, 0, 0, 0, nwk, nwk); err != nil || !ok {
		t.Fatalf("uplink MIC invalid: ok=%v err=%v", ok, err)
	}
	if err := uphy.DecryptFRMPayload(app); err != nil {
		t.Fatalf("decrypt uplink: %v", err)
	}
	gotUp := uphy.MACPayload.(*lorawan.MACPayload).FRMPayload[0].(*lorawan.DataPayload).Bytes
	if !bytes.Equal(gotUp, []byte{0, 0, 0, 0}) {
		t.Errorf("counter payload at fcnt 0 = % x, want 00000000", gotUp)
	}

	// --- DOWNLINK ---
	dnParams := DownlinkParams{DevAddr: lorawan.DevAddr{1, 2, 3, 4}, NwkSKey: nwk, AppSKey: app}
	dlFrame := buildDownlink(t, dnParams, 10, 0, []byte("ack-me"), true, true, nil)
	out, err := dev.HandleDownlink(ctx, dlFrame)
	if err != nil {
		t.Fatalf("HandleDownlink: %v", err)
	}
	if !bytes.Equal(out.Payload, []byte("ack-me")) {
		t.Errorf("downlink payload = %q, want ack-me", out.Payload)
	}
	if !out.ACK {
		t.Errorf("downlink ACK flag not set")
	}

	// --- persisted state ---
	final, _ := st.Sessions().Get(ctx, tagRow.ID)
	if final.DevNonce != 1 {
		t.Errorf("persisted DevNonce = %d, want 1", final.DevNonce)
	}
	if final.FCntUp != 1 {
		t.Errorf("persisted FCntUp = %d, want 1 (advanced after uplink)", final.FCntUp)
	}
	if final.FCntDown != 0 {
		t.Errorf("persisted FCntDown = %d, want 0", final.FCntDown)
	}
}

func TestBuildUplinkBeforeJoinFails(t *testing.T) {
	ctx := context.Background()
	st := newDeviceStore(t)
	tagRow, _ := st.Tags().Create(ctx, store.NewTag{
		DevEUI: "0102030405060708", JoinEUI: "0807060504030201",
		AppKey: "000102030405060708090a0b0c0d0e0f", Class: "A", Region: "EU868",
		DefaultDR: 5, FPort: 10, PayloadType: "counter", Enabled: true,
	})
	dev, _ := NewDevice(*tagRow, st)
	if _, _, err := dev.BuildUplink(ctx, nil); err == nil {
		t.Errorf("BuildUplink before join = nil error, want 'not joined'")
	}
}
