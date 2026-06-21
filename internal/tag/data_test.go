package tag

import (
	"bytes"
	"testing"

	"github.com/brocaar/lorawan"
)

func testSession() (UplinkParams, DownlinkParams) {
	devAddr := lorawan.DevAddr{4, 3, 2, 1}
	nwk := lorawan.AES128Key{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	app := lorawan.AES128Key{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	return UplinkParams{DevAddr: devAddr, NwkSKey: nwk, AppSKey: app},
		DownlinkParams{DevAddr: devAddr, NwkSKey: nwk, AppSKey: app}
}

// buildDownlink mimics the network: encrypt FRMPayload then sign.
func buildDownlink(t *testing.T, d DownlinkParams, fport uint8, fcnt uint32, data []byte, confirmed, ack bool, fopts []lorawan.Payload) []byte {
	t.Helper()
	mType := lorawan.UnconfirmedDataDown
	if confirmed {
		mType = lorawan.ConfirmedDataDown
	}
	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{MType: mType, Major: lorawan.LoRaWANR1},
		MACPayload: &lorawan.MACPayload{
			FHDR: lorawan.FHDR{
				DevAddr: d.DevAddr,
				FCtrl:   lorawan.FCtrl{ACK: ack},
				FCnt:    fcnt,
				FOpts:   fopts,
			},
			FPort:      &fport,
			FRMPayload: []lorawan.Payload{&lorawan.DataPayload{Bytes: data}},
		},
	}
	key := d.AppSKey
	if fport == 0 {
		key = d.NwkSKey
	}
	if len(data) > 0 {
		if err := phy.EncryptFRMPayload(key); err != nil {
			t.Fatalf("EncryptFRMPayload: %v", err)
		}
	}
	if err := phy.SetDownlinkDataMIC(lorawan.LoRaWAN1_0, 0, d.NwkSKey); err != nil {
		t.Fatalf("SetDownlinkDataMIC: %v", err)
	}
	b, err := phy.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	return b
}

func TestBuildUplinkRoundTrip(t *testing.T) {
	up, _ := testSession()
	up.FCnt = 1
	up.FPort = 10
	up.Data = []byte("hello")
	up.Confirmed = true

	frame, err := BuildUplink(up)
	if err != nil {
		t.Fatalf("BuildUplink: %v", err)
	}

	var phy lorawan.PHYPayload
	if err := phy.UnmarshalBinary(frame); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if phy.MHDR.MType != lorawan.ConfirmedDataUp {
		t.Errorf("MType = %v, want ConfirmedDataUp", phy.MHDR.MType)
	}
	ok, err := phy.ValidateUplinkDataMIC(lorawan.LoRaWAN1_0, 0, 0, 0, up.NwkSKey, up.NwkSKey)
	if err != nil || !ok {
		t.Fatalf("ValidateUplinkDataMIC = (%v, %v), want (true, nil)", ok, err)
	}
	if err := phy.DecryptFRMPayload(up.AppSKey); err != nil {
		t.Fatalf("DecryptFRMPayload: %v", err)
	}
	mac := phy.MACPayload.(*lorawan.MACPayload)
	got := mac.FRMPayload[0].(*lorawan.DataPayload).Bytes
	if !bytes.Equal(got, []byte("hello")) {
		t.Errorf("decrypted payload = %q, want hello", got)
	}
}

func TestParseDownlinkApplicationPayload(t *testing.T) {
	_, dn := testSession()
	frame := buildDownlink(t, dn, 20, 0, []byte("world"), true, true, nil)

	got, err := ParseDownlink(dn, frame)
	if err != nil {
		t.Fatalf("ParseDownlink: %v", err)
	}
	if got.FPort != 20 {
		t.Errorf("FPort = %d, want 20", got.FPort)
	}
	if !got.Confirmed || !got.ACK {
		t.Errorf("flags: confirmed=%v ack=%v, want both true", got.Confirmed, got.ACK)
	}
	if !bytes.Equal(got.Payload, []byte("world")) {
		t.Errorf("payload = %q, want world", got.Payload)
	}
}

func TestParseDownlinkMACCommandInFOpts(t *testing.T) {
	_, dn := testSession()
	fopts := []lorawan.Payload{&lorawan.MACCommand{CID: lorawan.DevStatusReq}}
	frame := buildDownlink(t, dn, 1, 0, []byte("x"), false, false, fopts)

	got, err := ParseDownlink(dn, frame)
	if err != nil {
		t.Fatalf("ParseDownlink: %v", err)
	}
	if len(got.MACCommands) != 1 {
		t.Fatalf("MACCommands = %d, want 1", len(got.MACCommands))
	}
	if got.MACCommands[0].CID != byte(lorawan.DevStatusReq) {
		t.Errorf("MAC CID = 0x%02x, want 0x06 (DevStatusReq)", got.MACCommands[0].CID)
	}
}

func TestParseDownlinkRejectsWrongKey(t *testing.T) {
	_, dn := testSession()
	frame := buildDownlink(t, dn, 10, 0, []byte("data"), false, false, nil)

	bad := dn
	bad.NwkSKey = lorawan.AES128Key{0xff}
	if _, err := ParseDownlink(bad, frame); err == nil {
		t.Errorf("ParseDownlink with wrong NwkSKey = nil error, want MIC failure")
	}
}
