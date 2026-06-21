package tag

import (
	"encoding/hex"
	"fmt"

	"github.com/brocaar/lorawan"
)

func parseEUI64(s string) (lorawan.EUI64, error) {
	var e lorawan.EUI64
	b, err := hex.DecodeString(s)
	if err != nil {
		return e, fmt.Errorf("parse EUI64 %q: %w", s, err)
	}
	if len(b) != len(e) {
		return e, fmt.Errorf("EUI64 %q must be %d bytes", s, len(e))
	}
	copy(e[:], b)
	return e, nil
}

func parseAES128(s string) (lorawan.AES128Key, error) {
	var k lorawan.AES128Key
	b, err := hex.DecodeString(s)
	if err != nil {
		return k, fmt.Errorf("parse AES128 key: %w", err)
	}
	if len(b) != len(k) {
		return k, fmt.Errorf("AES128 key must be %d bytes", len(k))
	}
	copy(k[:], b)
	return k, nil
}

func parseDevAddr(s string) (lorawan.DevAddr, error) {
	var a lorawan.DevAddr
	b, err := hex.DecodeString(s)
	if err != nil {
		return a, fmt.Errorf("parse DevAddr %q: %w", s, err)
	}
	if len(b) != len(a) {
		return a, fmt.Errorf("DevAddr %q must be %d bytes", s, len(a))
	}
	copy(a[:], b)
	return a, nil
}

func hexBytes(b []byte) string { return hex.EncodeToString(b) }
