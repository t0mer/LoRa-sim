package gateway

import (
	"encoding/hex"
	"testing"

	"github.com/brocaar/lorawan"

	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/tag"
)

func TestParseUplinkJoinRequest(t *testing.T) {
	prov := tag.Provisioning{
		DevEUI:  lorawan.EUI64{1, 1, 1, 1, 1, 1, 1, 1},
		JoinEUI: lorawan.EUI64{2, 2, 2, 2, 2, 2, 2, 2},
		AppKey:  lorawan.AES128Key{9},
	}
	frame, err := tag.BuildJoinRequest(prov, 0x0042)
	if err != nil {
		t.Fatalf("BuildJoinRequest: %v", err)
	}

	info := SynthUpInfo(1, 1000)
	pu, err := ParseUplink(hex.EncodeToString(frame), 868100000, 5, info)
	if err != nil {
		t.Fatalf("ParseUplink: %v", err)
	}
	if !pu.IsJoin {
		t.Fatal("IsJoin = false, want true")
	}
	if pu.DevEUI != "0101010101010101" {
		t.Errorf("DevEUI = %q", pu.DevEUI)
	}
	jr, ok := pu.Message.(protocol.Jreq)
	if !ok {
		t.Fatalf("Message = %T, want protocol.Jreq", pu.Message)
	}
	if jr.DevNonce != 0x0042 {
		t.Errorf("DevNonce = %d, want 0x42", jr.DevNonce)
	}
	if jr.JoinEui != "0202020202020202" || jr.DevEui != "0101010101010101" {
		t.Errorf("EUIs: %+v", jr)
	}
	if jr.Freq != 868100000 || jr.DR != 5 {
		t.Errorf("radio: freq=%d dr=%d", jr.Freq, jr.DR)
	}
	if jr.UpInfo.RSSI != defaultRSSI {
		t.Errorf("UpInfo not synthesized: %+v", jr.UpInfo)
	}
}

func TestParseUplinkDataFrame(t *testing.T) {
	plain := []byte("payload")
	frame, err := tag.BuildUplink(tag.UplinkParams{
		DevAddr: lorawan.DevAddr{4, 3, 2, 1},
		NwkSKey: lorawan.AES128Key{1},
		AppSKey: lorawan.AES128Key{2},
		FCnt:    9,
		FPort:   10,
		Data:    plain,
	})
	if err != nil {
		t.Fatalf("BuildUplink: %v", err)
	}

	pu, err := ParseUplink(hex.EncodeToString(frame), 868300000, 4, SynthUpInfo(2, 2000))
	if err != nil {
		t.Fatalf("ParseUplink: %v", err)
	}
	if pu.IsJoin {
		t.Fatal("IsJoin = true, want false for data uplink")
	}
	if pu.DevAddr != "04030201" {
		t.Errorf("DevAddr = %q, want 04030201", pu.DevAddr)
	}
	up, ok := pu.Message.(protocol.Updf)
	if !ok {
		t.Fatalf("Message = %T, want protocol.Updf", pu.Message)
	}
	if up.FPort != 10 || up.FCnt != 9 {
		t.Errorf("FPort=%d FCnt=%d", up.FPort, up.FCnt)
	}
	// FRMPayload must be forwarded still-encrypted (not the plaintext).
	if up.FRMPayload == "" {
		t.Error("FRMPayload empty")
	}
	if got, _ := hex.DecodeString(up.FRMPayload); string(got) == string(plain) {
		t.Error("FRMPayload forwarded in plaintext; bridge must keep it encrypted")
	}
}

func TestParseChannelPlan(t *testing.T) {
	rc := &protocol.RouterConfig{
		Region:    "EU863",
		NetID:     []uint32{1, 2},
		JoinEui:   [][]uint64{{100, 200}},
		FreqRange: []uint32{863000000, 870000000},
		DRs:       [][]int{{12, 125, 0}},
	}
	cp := ParseChannelPlan(rc)
	if cp.Region != "EU863" || len(cp.NetIDs) != 2 {
		t.Errorf("plan = %+v", cp)
	}
	if cp.FreqRange != [2]uint32{863000000, 870000000} {
		t.Errorf("freq range = %v", cp.FreqRange)
	}
	if !cp.AllowsJoinEUI(150) {
		t.Error("AllowsJoinEUI(150) = false, want true (in [100,200])")
	}
	if cp.AllowsJoinEUI(50) {
		t.Error("AllowsJoinEUI(50) = true, want false")
	}
}
