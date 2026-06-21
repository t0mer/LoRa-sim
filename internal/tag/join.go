package tag

import (
	"errors"
	"fmt"

	"github.com/brocaar/lorawan"
)

// Provisioning holds the OTAA identity provisioned onto a device.
type Provisioning struct {
	DevEUI  lorawan.EUI64
	JoinEUI lorawan.EUI64
	AppKey  lorawan.AES128Key
}

// JoinResult is the session state derived from a successful OTAA join-accept.
type JoinResult struct {
	DevAddr     lorawan.DevAddr
	NwkSKey     lorawan.AES128Key
	AppSKey     lorawan.AES128Key
	NetID       lorawan.NetID
	JoinNonce   lorawan.JoinNonce
	RxDelay     uint8
	RX1DROffset uint8
	RX2DataRate uint8
	CFList      *lorawan.CFList
}

// BuildJoinRequest builds and signs an OTAA join-request PHYPayload for the
// given DevNonce, returning its binary encoding. DevNonce must be monotonic and
// never reused (the caller persists it).
func BuildJoinRequest(p Provisioning, devNonce uint16) ([]byte, error) {
	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{MType: lorawan.JoinRequest, Major: lorawan.LoRaWANR1},
		MACPayload: &lorawan.JoinRequestPayload{
			JoinEUI:  p.JoinEUI,
			DevEUI:   p.DevEUI,
			DevNonce: lorawan.DevNonce(devNonce),
		},
	}
	if err := phy.SetUplinkJoinMIC(p.AppKey); err != nil {
		return nil, fmt.Errorf("signing join-request: %w", err)
	}
	b, err := phy.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling join-request: %w", err)
	}
	return b, nil
}

// ParseJoinAccept decrypts and validates an OTAA join-accept against the
// provisioning and the DevNonce used for the matching join-request, then derives
// the 1.0.x session keys and returns the resulting session state.
func ParseJoinAccept(p Provisioning, devNonce uint16, data []byte) (*JoinResult, error) {
	var phy lorawan.PHYPayload
	if err := phy.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("unmarshaling join-accept: %w", err)
	}
	if err := phy.DecryptJoinAcceptPayload(p.AppKey); err != nil {
		return nil, fmt.Errorf("decrypting join-accept: %w", err)
	}

	ok, err := phy.ValidateDownlinkJoinMIC(lorawan.JoinRequestType, p.JoinEUI, lorawan.DevNonce(devNonce), p.AppKey)
	if err != nil {
		return nil, fmt.Errorf("validating join-accept MIC: %w", err)
	}
	if !ok {
		return nil, errors.New("join-accept MIC is invalid")
	}

	jap, ok := phy.MACPayload.(*lorawan.JoinAcceptPayload)
	if !ok {
		return nil, fmt.Errorf("join-accept payload has unexpected type %T", phy.MACPayload)
	}

	nwkSKey, err := DeriveNwkSKey(p.AppKey, jap.HomeNetID, jap.JoinNonce, lorawan.DevNonce(devNonce))
	if err != nil {
		return nil, fmt.Errorf("deriving NwkSKey: %w", err)
	}
	appSKey, err := DeriveAppSKey(p.AppKey, jap.HomeNetID, jap.JoinNonce, lorawan.DevNonce(devNonce))
	if err != nil {
		return nil, fmt.Errorf("deriving AppSKey: %w", err)
	}

	return &JoinResult{
		DevAddr:     jap.DevAddr,
		NwkSKey:     nwkSKey,
		AppSKey:     appSKey,
		NetID:       jap.HomeNetID,
		JoinNonce:   jap.JoinNonce,
		RxDelay:     jap.RXDelay,
		RX1DROffset: jap.DLSettings.RX1DROffset,
		RX2DataRate: jap.DLSettings.RX2DataRate,
		CFList:      jap.CFList,
	}, nil
}
