package gateway

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateEUIRandom(t *testing.T) {
	a, err := GenerateEUI("")
	if err != nil {
		t.Fatalf("GenerateEUI: %v", err)
	}
	if len(a) != 16 {
		t.Fatalf("len(%q) = %d, want 16", a, len(a))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Errorf("EUI %q is not valid hex: %v", a, err)
	}
	b, _ := GenerateEUI("")
	if a == b {
		t.Errorf("two random EUIs are identical (%q); randomness broken", a)
	}
}

func TestGenerateEUIWithOUIPrefixInsertsFFFE(t *testing.T) {
	eui, err := GenerateEUI("aabbcc")
	if err != nil {
		t.Fatalf("GenerateEUI: %v", err)
	}
	if len(eui) != 16 {
		t.Fatalf("len = %d, want 16", len(eui))
	}
	if !strings.HasPrefix(eui, "aabbccfffe") {
		t.Errorf("EUI %q does not start with aabbccfffe (Station FFFE insertion)", eui)
	}
}

func TestGenerateEUIFullPrefixReturnedAsIs(t *testing.T) {
	eui, err := GenerateEUI("0102030405060708")
	if err != nil {
		t.Fatalf("GenerateEUI: %v", err)
	}
	if eui != "0102030405060708" {
		t.Errorf("EUI = %q, want 0102030405060708", eui)
	}
}

func TestGenerateEUIRejectsBadPrefix(t *testing.T) {
	for _, p := range []string{"xyz", "abc", "0102030405060708aa"} {
		if _, err := GenerateEUI(p); err == nil {
			t.Errorf("GenerateEUI(%q) = nil error, want error", p)
		}
	}
}

func TestNormalizeEUI(t *testing.T) {
	got, err := NormalizeEUI("  AABBCCDDEEFF0011 ")
	if err != nil {
		t.Fatalf("NormalizeEUI: %v", err)
	}
	if got != "aabbccddeeff0011" {
		t.Errorf("NormalizeEUI = %q, want aabbccddeeff0011", got)
	}
	if _, err := NormalizeEUI("nothex"); err == nil {
		t.Errorf("NormalizeEUI(nothex) = nil, want error")
	}
	if _, err := NormalizeEUI("aabb"); err == nil {
		t.Errorf("NormalizeEUI(short) = nil, want error")
	}
}
