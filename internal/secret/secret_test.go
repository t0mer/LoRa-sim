package secret

import (
	"strings"
	"testing"
)

// testKey is a valid 32-byte key in hex (deterministic for tests only).
const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

var aad = []byte("dev_eui=0102030405060708|app_key")

func TestRoundTripEnabled(t *testing.T) {
	c, err := New(testKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !c.Enabled() {
		t.Fatal("Enabled() = false, want true")
	}

	const plain = "00112233445566778899aabbccddeeff"
	enc, err := c.Encrypt(plain, aad)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plain {
		t.Errorf("ciphertext equals plaintext; not encrypted")
	}
	if dec, err := c.Decrypt(enc, aad); err != nil || dec != plain {
		t.Errorf("Decrypt = (%q, %v), want (%q, nil)", dec, err, plain)
	}
}

func TestBase64KeyAccepted(t *testing.T) {
	// 32 zero bytes, base64-encoded.
	if _, err := New("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Errorf("New(base64 32-byte key): %v", err)
	}
}

func TestShortKeyRejected(t *testing.T) {
	if _, err := New("tooshort"); err == nil {
		t.Errorf("New(short) = nil error, want error")
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	// Pass-through must be opt-in, never a silent fail-open from New.
	if _, err := New(""); err == nil {
		t.Errorf("New(\"\") = nil error, want error (use NewInsecureDisabled)")
	}
}

func TestEncryptIsNondeterministic(t *testing.T) {
	c, _ := New(testKey)
	a, _ := c.Encrypt("same", aad)
	b, _ := c.Encrypt("same", aad)
	if a == b {
		t.Errorf("two encryptions of the same plaintext are identical; nonce reuse")
	}
	for _, v := range []string{a, b} {
		if got, err := c.Decrypt(v, aad); err != nil || got != "same" {
			t.Errorf("Decrypt(%q) = (%q, %v)", v, got, err)
		}
	}
}

func TestWrongAADFails(t *testing.T) {
	c, _ := New(testKey)
	enc, _ := c.Encrypt("secret", aad)
	if _, err := c.Decrypt(enc, []byte("different-row")); err == nil {
		t.Errorf("Decrypt with wrong AAD = nil error, want authentication failure")
	}
}

func TestDisabledIsPassthrough(t *testing.T) {
	c := NewInsecureDisabled()
	if c.Enabled() {
		t.Fatal("Enabled() = true for empty key, want false")
	}
	enc, err := c.Encrypt("plain", aad)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc != "plain" {
		t.Errorf("disabled Encrypt = %q, want passthrough 'plain'", enc)
	}
	if dec, _ := c.Decrypt("plain", aad); dec != "plain" {
		t.Errorf("disabled Decrypt = %q, want 'plain'", dec)
	}
}

func TestUnprefixedRejectedWhenEnabled(t *testing.T) {
	// Once a key is configured, an unencrypted (unprefixed) value must be a hard
	// error, never silently trusted plaintext.
	c, _ := New(testKey)
	if _, err := c.Decrypt("legacy-plaintext", aad); err == nil {
		t.Errorf("Decrypt(unprefixed) when enabled = nil error, want error")
	}
}

func TestDecryptEncryptedWhenDisabledErrors(t *testing.T) {
	c, _ := New(testKey)
	enc, _ := c.Encrypt("secret", aad)
	disabled := NewInsecureDisabled()
	if _, err := disabled.Decrypt(enc, aad); err == nil {
		t.Errorf("disabled Decrypt of ciphertext = nil error, want error")
	}
}

func TestPrefixedFormat(t *testing.T) {
	c, _ := New(testKey)
	enc, _ := c.Encrypt("x", aad)
	if !strings.HasPrefix(enc, "v1:") {
		t.Errorf("ciphertext %q missing v1: prefix", enc)
	}
}
