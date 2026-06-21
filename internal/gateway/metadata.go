package gateway

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/brocaar/lorawan"

	"github.com/t0mer/cylon/internal/gateway/protocol"
)

// Default synthesized radio quality for uplinks (no real RF).
const (
	defaultRSSI = -42.0
	defaultSNR  = 9.0
)

// SynthUpInfo builds synthesized radio metadata for an uplink received at the
// given synthetic xtime on receive context rctx.
func SynthUpInfo(rctx, xtime int64) protocol.UpInfo {
	return protocol.UpInfo{
		RCtx:  rctx,
		XTime: xtime,
		RSSI:  defaultRSSI,
		SNR:   defaultSNR,
	}
}

// ParsedUplink is the result of decoding a tag's raw PHYPayload: the Basic
// Station message to forward (a *protocol.Jreq or *protocol.Updf) plus the
// routing keys the gateway's registry needs.
type ParsedUplink struct {
	Message any    // *protocol.Jreq or *protocol.Updf
	IsJoin  bool   // true for a join-request
	DevEUI  string // set for join-requests
	DevAddr string // set for data uplinks (hex)
}

// ParseUplink decodes a tag's PHYPayload (hex) and produces the matching Basic
// Station uplink message, carrying parsed fields plus the synthesized metadata.
// Per the bridge invariant, FRMPayload is forwarded still-encrypted.
func ParseUplink(phyHex string, freq uint32, dr uint8, info protocol.UpInfo) (*ParsedUplink, error) {
	raw, err := hex.DecodeString(phyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding phy hex: %w", err)
	}
	var phy lorawan.PHYPayload
	if err := phy.UnmarshalBinary(raw); err != nil {
		return nil, fmt.Errorf("unmarshaling phy: %w", err)
	}

	mhdr, err := phy.MHDR.MarshalBinary()
	if err != nil || len(mhdr) != 1 {
		return nil, fmt.Errorf("marshaling MHDR: %w", err)
	}
	mic := int32(binary.LittleEndian.Uint32(phy.MIC[:]))

	switch phy.MHDR.MType {
	case lorawan.JoinRequest:
		jr, ok := phy.MACPayload.(*lorawan.JoinRequestPayload)
		if !ok {
			return nil, fmt.Errorf("join-request payload type %T", phy.MACPayload)
		}
		devEUI := hex.EncodeToString(jr.DevEUI[:])
		msg := protocol.Jreq{
			MHdr:     mhdr[0],
			JoinEui:  hex.EncodeToString(jr.JoinEUI[:]),
			DevEui:   devEUI,
			DevNonce: uint16(jr.DevNonce),
			MIC:      mic,
			DR:       dr,
			Freq:     freq,
			UpInfo:   info,
		}
		return &ParsedUplink{Message: msg, IsJoin: true, DevEUI: devEUI}, nil

	case lorawan.UnconfirmedDataUp, lorawan.ConfirmedDataUp:
		mac, ok := phy.MACPayload.(*lorawan.MACPayload)
		if !ok {
			return nil, fmt.Errorf("data payload type %T", phy.MACPayload)
		}
		fopts := marshalPayloads(mac.FHDR.FOpts)
		frm := marshalPayloads(mac.FRMPayload)
		fport := -1
		if mac.FPort != nil {
			fport = int(*mac.FPort)
		}
		devAddr := hex.EncodeToString(mac.FHDR.DevAddr[:])
		msg := protocol.Updf{
			MHdr:       mhdr[0],
			DevAddr:    int32(binary.BigEndian.Uint32(mac.FHDR.DevAddr[:])),
			FCtrl:      uplinkFCtrl(mac.FHDR.FCtrl, len(fopts)),
			FCnt:       mac.FHDR.FCnt,
			FOpts:      hex.EncodeToString(fopts),
			FPort:      fport,
			FRMPayload: hex.EncodeToString(frm),
			MIC:        mic,
			DR:         dr,
			Freq:       freq,
			UpInfo:     info,
		}
		return &ParsedUplink{Message: msg, DevAddr: devAddr}, nil

	default:
		return nil, fmt.Errorf("unsupported uplink mtype %v", phy.MHDR.MType)
	}
}

func uplinkFCtrl(f lorawan.FCtrl, foptsLen int) uint8 {
	var b uint8
	if f.ADR {
		b |= 0x80
	}
	if f.ADRACKReq {
		b |= 0x40
	}
	if f.ACK {
		b |= 0x20
	}
	if f.ClassB {
		b |= 0x10
	}
	b |= uint8(foptsLen & 0x0f)
	return b
}

func marshalPayloads(payloads []lorawan.Payload) []byte {
	var out []byte
	for _, pl := range payloads {
		b, err := pl.MarshalBinary()
		if err != nil {
			continue
		}
		out = append(out, b...)
	}
	return out
}
