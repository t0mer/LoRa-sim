package tag

import (
	"encoding/hex"
	"testing"

	"github.com/brocaar/lorawan"
)

func TestDeriveSessionKeysKnownAnswer(t *testing.T) {
	appKey := lorawan.AES128Key{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	netID := lorawan.NetID{1, 2, 3}
	joinNonce := lorawan.JoinNonce(0x010203)
	devNonce := lorawan.DevNonce(0x0405)

	nwk, err := DeriveNwkSKey(appKey, netID, joinNonce, devNonce)
	if err != nil {
		t.Fatalf("DeriveNwkSKey: %v", err)
	}
	app, err := DeriveAppSKey(appKey, netID, joinNonce, devNonce)
	if err != nil {
		t.Fatalf("DeriveAppSKey: %v", err)
	}

	// Frozen vectors (regression guard) computed from the brocaar reference
	// getSKey construction replicated in keys.go.
	const wantNwk = "edcf82cc9cba661664437179452f64b0"
	const wantApp = "7a6f20d8b5846ec745335082ce69f9b7"
	if got := hex.EncodeToString(nwk[:]); got != wantNwk {
		t.Errorf("NwkSKey = %s, want %s", got, wantNwk)
	}
	if got := hex.EncodeToString(app[:]); got != wantApp {
		t.Errorf("AppSKey = %s, want %s", got, wantApp)
	}
}

func TestDeriveKeysDeterministicAndDistinct(t *testing.T) {
	appKey := lorawan.AES128Key{0xaa}
	netID := lorawan.NetID{9, 9, 9}
	jn := lorawan.JoinNonce(42)
	dn := lorawan.DevNonce(7)

	nwk1, _ := DeriveNwkSKey(appKey, netID, jn, dn)
	nwk2, _ := DeriveNwkSKey(appKey, netID, jn, dn)
	if nwk1 != nwk2 {
		t.Errorf("NwkSKey not deterministic for identical inputs")
	}
	app, _ := DeriveAppSKey(appKey, netID, jn, dn)
	if nwk1 == app {
		t.Errorf("NwkSKey and AppSKey are identical; type byte not applied")
	}
}

func TestDeriveKeysVaryWithDevNonce(t *testing.T) {
	appKey := lorawan.AES128Key{0xbb}
	netID := lorawan.NetID{1, 1, 1}
	jn := lorawan.JoinNonce(1)

	a, _ := DeriveNwkSKey(appKey, netID, jn, lorawan.DevNonce(1))
	b, _ := DeriveNwkSKey(appKey, netID, jn, lorawan.DevNonce(2))
	if a == b {
		t.Errorf("NwkSKey did not change with DevNonce")
	}
}
