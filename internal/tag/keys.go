package tag

import (
	"crypto/aes"
	"fmt"

	"github.com/brocaar/lorawan"
)

// LoRaWAN 1.0.x session-key derivation.
//
// This brocaar/lorawan release is PHY-only and no longer exports session-key
// derivation, so we replicate its reference construction verbatim
// (backend/joinserver getSKey with optNeg=false):
//
//	SKey = aes128_encrypt(AppKey, type | JoinNonce | NetID | DevNonce | pad₁₆)
//
// where JoinNonce (3 B), NetID (3 B) and DevNonce (2 B) are little-endian as on
// the wire. type 0x01 yields NwkSKey, 0x02 yields AppSKey. JoinEUI is unused in
// the 1.0.x (non-OptNeg) path.
func deriveSKey(typ byte, appKey lorawan.AES128Key, netID lorawan.NetID, joinNonce lorawan.JoinNonce, devNonce lorawan.DevNonce) (lorawan.AES128Key, error) {
	var key lorawan.AES128Key

	netIDB, err := netID.MarshalBinary()
	if err != nil {
		return key, fmt.Errorf("marshal netID: %w", err)
	}
	joinNonceB, err := joinNonce.MarshalBinary()
	if err != nil {
		return key, fmt.Errorf("marshal joinNonce: %w", err)
	}
	devNonceB, err := devNonce.MarshalBinary()
	if err != nil {
		return key, fmt.Errorf("marshal devNonce: %w", err)
	}

	b := make([]byte, 16)
	b[0] = typ
	copy(b[1:4], joinNonceB)
	copy(b[4:7], netIDB)
	copy(b[7:9], devNonceB)

	block, err := aes.NewCipher(appKey[:])
	if err != nil {
		return key, fmt.Errorf("aes cipher: %w", err)
	}
	block.Encrypt(key[:], b)
	return key, nil
}

// DeriveNwkSKey derives the network session key for LoRaWAN 1.0.x OTAA.
func DeriveNwkSKey(appKey lorawan.AES128Key, netID lorawan.NetID, joinNonce lorawan.JoinNonce, devNonce lorawan.DevNonce) (lorawan.AES128Key, error) {
	return deriveSKey(0x01, appKey, netID, joinNonce, devNonce)
}

// DeriveAppSKey derives the application session key for LoRaWAN 1.0.x OTAA.
func DeriveAppSKey(appKey lorawan.AES128Key, netID lorawan.NetID, joinNonce lorawan.JoinNonce, devNonce lorawan.DevNonce) (lorawan.AES128Key, error) {
	return deriveSKey(0x02, appKey, netID, joinNonce, devNonce)
}
