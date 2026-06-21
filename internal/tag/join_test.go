package tag

import (
	"testing"

	"github.com/brocaar/lorawan"
)

func testProvisioning() Provisioning {
	return Provisioning{
		DevEUI:  lorawan.EUI64{1, 1, 1, 1, 1, 1, 1, 1},
		JoinEUI: lorawan.EUI64{2, 2, 2, 2, 2, 2, 2, 2},
		AppKey:  lorawan.AES128Key{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
	}
}

// buildJoinAccept mimics the network side: sign then encrypt the join-accept.
func buildJoinAccept(t *testing.T, appKey lorawan.AES128Key, joinEUI lorawan.EUI64, devNonce uint16, jap lorawan.JoinAcceptPayload) []byte {
	t.Helper()
	phy := lorawan.PHYPayload{
		MHDR:       lorawan.MHDR{MType: lorawan.JoinAccept, Major: lorawan.LoRaWANR1},
		MACPayload: &jap,
	}
	if err := phy.SetDownlinkJoinMIC(lorawan.JoinRequestType, joinEUI, lorawan.DevNonce(devNonce), appKey); err != nil {
		t.Fatalf("SetDownlinkJoinMIC: %v", err)
	}
	if err := phy.EncryptJoinAcceptPayload(appKey); err != nil {
		t.Fatalf("EncryptJoinAcceptPayload: %v", err)
	}
	b, err := phy.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	return b
}

func TestBuildJoinRequestRoundTrip(t *testing.T) {
	p := testProvisioning()
	const devNonce = 0x0042

	b, err := BuildJoinRequest(p, devNonce)
	if err != nil {
		t.Fatalf("BuildJoinRequest: %v", err)
	}

	var phy lorawan.PHYPayload
	if err := phy.UnmarshalBinary(b); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	ok, err := phy.ValidateUplinkJoinMIC(p.AppKey)
	if err != nil || !ok {
		t.Fatalf("ValidateUplinkJoinMIC = (%v, %v), want (true, nil)", ok, err)
	}
	jr, ok := phy.MACPayload.(*lorawan.JoinRequestPayload)
	if !ok {
		t.Fatalf("MACPayload type = %T", phy.MACPayload)
	}
	if jr.DevNonce != lorawan.DevNonce(devNonce) {
		t.Errorf("DevNonce = %d, want %d", jr.DevNonce, devNonce)
	}
	if jr.DevEUI != p.DevEUI || jr.JoinEUI != p.JoinEUI {
		t.Errorf("EUIs mismatch in join-request")
	}
}

func TestParseJoinAcceptDerivesSession(t *testing.T) {
	p := testProvisioning()
	const devNonce = 0x0042

	accept := lorawan.JoinAcceptPayload{
		JoinNonce:  lorawan.JoinNonce(0x010203),
		HomeNetID:  lorawan.NetID{1, 2, 3},
		DevAddr:    lorawan.DevAddr{4, 3, 2, 1},
		DLSettings: lorawan.DLSettings{RX2DataRate: 3, RX1DROffset: 1},
		RXDelay:    5,
	}
	frame := buildJoinAccept(t, p.AppKey, p.JoinEUI, devNonce, accept)

	res, err := ParseJoinAccept(p, devNonce, frame)
	if err != nil {
		t.Fatalf("ParseJoinAccept: %v", err)
	}
	if res.DevAddr != accept.DevAddr {
		t.Errorf("DevAddr = %v, want %v", res.DevAddr, accept.DevAddr)
	}
	if res.NetID != accept.HomeNetID {
		t.Errorf("NetID = %v, want %v", res.NetID, accept.HomeNetID)
	}
	if res.RxDelay != 5 || res.RX2DataRate != 3 || res.RX1DROffset != 1 {
		t.Errorf("DL settings mismatch: %+v", res)
	}

	// Keys must match independent derivation.
	wantNwk, _ := DeriveNwkSKey(p.AppKey, accept.HomeNetID, accept.JoinNonce, lorawan.DevNonce(devNonce))
	wantApp, _ := DeriveAppSKey(p.AppKey, accept.HomeNetID, accept.JoinNonce, lorawan.DevNonce(devNonce))
	if res.NwkSKey != wantNwk || res.AppSKey != wantApp {
		t.Errorf("derived session keys mismatch")
	}
}

func TestParseJoinAcceptRejectsWrongKey(t *testing.T) {
	p := testProvisioning()
	const devNonce = 7
	accept := lorawan.JoinAcceptPayload{
		JoinNonce: lorawan.JoinNonce(1),
		HomeNetID: lorawan.NetID{1, 2, 3},
		DevAddr:   lorawan.DevAddr{1, 2, 3, 4},
	}
	frame := buildJoinAccept(t, p.AppKey, p.JoinEUI, devNonce, accept)

	wrong := p
	wrong.AppKey = lorawan.AES128Key{0xff}
	if _, err := ParseJoinAccept(wrong, devNonce, frame); err == nil {
		t.Errorf("ParseJoinAccept with wrong AppKey = nil error, want MIC failure")
	}
}
