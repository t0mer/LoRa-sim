package tag

import (
	"errors"
	"fmt"

	"github.com/brocaar/lorawan"
)

// UplinkParams describes a data uplink to build.
type UplinkParams struct {
	DevAddr   lorawan.DevAddr
	NwkSKey   lorawan.AES128Key
	AppSKey   lorawan.AES128Key
	FCnt      uint32
	FPort     uint8
	Data      []byte
	Confirmed bool
	ADR       bool
	ACK       bool
}

// BuildUplink builds, encrypts, and signs a LoRaWAN 1.0.x data uplink and
// returns its binary PHYPayload. FRMPayload is encrypted with AppSKey, except on
// FPort 0 (MAC-command payload) where NwkSKey is used.
func BuildUplink(p UplinkParams) ([]byte, error) {
	mType := lorawan.UnconfirmedDataUp
	if p.Confirmed {
		mType = lorawan.ConfirmedDataUp
	}

	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{MType: mType, Major: lorawan.LoRaWANR1},
		MACPayload: &lorawan.MACPayload{
			FHDR: lorawan.FHDR{
				DevAddr: p.DevAddr,
				FCtrl:   lorawan.FCtrl{ADR: p.ADR, ACK: p.ACK},
				FCnt:    p.FCnt,
			},
			FPort:      &p.FPort,
			FRMPayload: []lorawan.Payload{&lorawan.DataPayload{Bytes: p.Data}},
		},
	}

	frmKey := p.AppSKey
	if p.FPort == 0 {
		frmKey = p.NwkSKey
	}
	if err := phy.EncryptFRMPayload(frmKey); err != nil {
		return nil, fmt.Errorf("encrypting FRMPayload: %w", err)
	}
	// LoRaWAN 1.0: FNwkSIntKey == SNwkSIntKey == NwkSKey; confFCnt/txDR/txCh unused.
	if err := phy.SetUplinkDataMIC(lorawan.LoRaWAN1_0, 0, 0, 0, p.NwkSKey, p.NwkSKey); err != nil {
		return nil, fmt.Errorf("signing uplink: %w", err)
	}
	b, err := phy.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling uplink: %w", err)
	}
	return b, nil
}

// MACCommandRaw is a surfaced MAC command: its identifier and raw payload bytes.
type MACCommandRaw struct {
	CID     byte
	Payload []byte
}

// Downlink is the decoded content of a data downlink.
type Downlink struct {
	FPort       uint8
	FCnt        uint32
	Confirmed   bool
	ACK         bool
	FPending    bool
	Payload     []byte          // decrypted application payload (FPort > 0)
	MACCommands []MACCommandRaw // from FOpts and/or FPort 0
}

// DownlinkParams carries the session material needed to decode a downlink.
type DownlinkParams struct {
	DevAddr lorawan.DevAddr
	NwkSKey lorawan.AES128Key
	AppSKey lorawan.AES128Key
}

// ParseDownlink validates and decodes a LoRaWAN 1.0.x data downlink: it checks
// the MIC, decrypts the application payload (or MAC commands on FPort 0), and
// surfaces any MAC commands carried in FOpts.
func ParseDownlink(p DownlinkParams, data []byte) (*Downlink, error) {
	var phy lorawan.PHYPayload
	if err := phy.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("unmarshaling downlink: %w", err)
	}

	switch phy.MHDR.MType {
	case lorawan.UnconfirmedDataDown, lorawan.ConfirmedDataDown:
	default:
		return nil, fmt.Errorf("not a data downlink (mtype %v)", phy.MHDR.MType)
	}

	// LoRaWAN 1.0: only the low 16 bits are transmitted. Phase 1 sims stay well
	// below 2^16, so the transmitted value is the full counter (rollover later).
	ok, err := phy.ValidateDownlinkDataMIC(lorawan.LoRaWAN1_0, 0, p.NwkSKey)
	if err != nil {
		return nil, fmt.Errorf("validating downlink MIC: %w", err)
	}
	if !ok {
		return nil, errors.New("downlink MIC is invalid")
	}

	mac, ok := phy.MACPayload.(*lorawan.MACPayload)
	if !ok {
		return nil, fmt.Errorf("downlink payload has unexpected type %T", phy.MACPayload)
	}

	out := &Downlink{
		FCnt:      mac.FHDR.FCnt,
		Confirmed: phy.MHDR.MType == lorawan.ConfirmedDataDown,
		ACK:       mac.FHDR.FCtrl.ACK,
		FPending:  mac.FHDR.FCtrl.FPending,
	}

	// MAC commands carried in FOpts (not encrypted in 1.0).
	if len(mac.FHDR.FOpts) > 0 {
		if err := phy.DecodeFOptsToMACCommands(); err != nil {
			return nil, fmt.Errorf("decoding FOpts MAC commands: %w", err)
		}
		cmds, err := collectMACCommands(mac.FHDR.FOpts)
		if err != nil {
			return nil, err
		}
		out.MACCommands = append(out.MACCommands, cmds...)
	}

	if mac.FPort != nil {
		out.FPort = *mac.FPort
		if *mac.FPort == 0 {
			// FRMPayload carries MAC commands, encrypted with NwkSKey.
			if err := phy.DecryptFRMPayload(p.NwkSKey); err != nil {
				return nil, fmt.Errorf("decrypting MAC FRMPayload: %w", err)
			}
			if err := phy.DecodeFRMPayloadToMACCommands(); err != nil {
				return nil, fmt.Errorf("decoding FRMPayload MAC commands: %w", err)
			}
			cmds, err := collectMACCommands(mac.FRMPayload)
			if err != nil {
				return nil, err
			}
			out.MACCommands = append(out.MACCommands, cmds...)
		} else {
			if err := phy.DecryptFRMPayload(p.AppSKey); err != nil {
				return nil, fmt.Errorf("decrypting FRMPayload: %w", err)
			}
			if dp, ok := firstDataPayload(mac.FRMPayload); ok {
				out.Payload = dp
			}
		}
	}

	return out, nil
}

func collectMACCommands(payloads []lorawan.Payload) ([]MACCommandRaw, error) {
	var cmds []MACCommandRaw
	for _, pl := range payloads {
		cmd, ok := pl.(*lorawan.MACCommand)
		if !ok {
			continue
		}
		b, err := cmd.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshaling MAC command: %w", err)
		}
		if len(b) == 0 {
			continue
		}
		cmds = append(cmds, MACCommandRaw{CID: b[0], Payload: b[1:]})
	}
	return cmds, nil
}

func firstDataPayload(payloads []lorawan.Payload) ([]byte, bool) {
	for _, pl := range payloads {
		if dp, ok := pl.(*lorawan.DataPayload); ok {
			return dp.Bytes, true
		}
	}
	return nil, false
}
