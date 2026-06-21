package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDecodeRouterConfig(t *testing.T) {
	raw := []byte(`{
		"msgtype":"router_config",
		"NetID":[1],
		"JoinEui":[[0,18446744073709551615]],
		"region":"EU863",
		"hwspec":"sx1301/1",
		"freq_range":[863000000,870000000],
		"DRs":[[12,125,0],[11,125,0]],
		"sx1301_conf":[{"radio_0":{"enable":true,"freq":867500000}}],
		"nocca":true,"nodc":true,"nodwell":true,"max_eirp":16
	}`)
	msg, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	rc, ok := msg.(*RouterConfig)
	if !ok {
		t.Fatalf("Decode returned %T, want *RouterConfig", msg)
	}
	if rc.Region != "EU863" {
		t.Errorf("Region = %q, want EU863", rc.Region)
	}
	if len(rc.NetID) != 1 || rc.NetID[0] != 1 {
		t.Errorf("NetID = %v", rc.NetID)
	}
	if len(rc.JoinEui) != 1 || rc.JoinEui[0][1] != 18446744073709551615 {
		t.Errorf("JoinEui range = %v", rc.JoinEui)
	}
	if !rc.NoCCA || !rc.NoDC || !rc.NoDwell {
		t.Errorf("flags not parsed: %+v", rc)
	}
}

func TestDecodeDnmsg(t *testing.T) {
	raw := []byte(`{"msgtype":"dnmsg","DevEui":"0102030405060708","dC":0,"diid":42,
		"pdu":"aabbcc","RxDelay":1,"RX1DR":5,"RX1Freq":868100000,
		"RX2DR":0,"RX2Freq":869525000,"priority":0,"xtime":12345,"rctx":1}`)
	msg, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	dn, ok := msg.(*Dnmsg)
	if !ok {
		t.Fatalf("Decode returned %T, want *Dnmsg", msg)
	}
	if dn.Pdu != "aabbcc" || dn.DIID != 42 || dn.RX1Freq != 868100000 {
		t.Errorf("dnmsg fields: %+v", dn)
	}
}

func TestJreqRoundTrip(t *testing.T) {
	in := Jreq{
		MsgType: TypeJreq, MHdr: 0,
		JoinEui: "0202020202020202", DevEui: "0101010101010101",
		DevNonce: 7, MIC: -123456, DR: 5, Freq: 868100000,
		UpInfo: UpInfo{RCtx: 1, XTime: 999, RSSI: -42, SNR: 9.5},
	}
	b, err := Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	msg, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got := msg.(*Jreq)
	if *got != in {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", *got, in)
	}
}

func TestUpdfSignedFields(t *testing.T) {
	// DevAddr and MIC must serialize as signed JSON numbers.
	in := Updf{MsgType: TypeUpdf, DevAddr: -1, MIC: -2147483648, FPort: -1, FCnt: 3}
	b, _ := Encode(in)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["DevAddr"].(float64) != -1 {
		t.Errorf("DevAddr not signed: %v", m["DevAddr"])
	}
	if m["FPort"].(float64) != -1 {
		t.Errorf("FPort absent marker not -1: %v", m["FPort"])
	}
}

func TestDecodeUnknownType(t *testing.T) {
	raw := []byte(`{"msgtype":"runcmd","x":1}`)
	msg, err := Decode(raw)
	if !errors.Is(err, ErrUnknownType) {
		t.Errorf("err = %v, want ErrUnknownType", err)
	}
	if env, ok := msg.(*Envelope); !ok || env.MsgType != "runcmd" {
		t.Errorf("unknown decode = %v", msg)
	}
}
