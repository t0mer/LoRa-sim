// Package euid provides EUI-64 helpers (generation and normalization) used for
// the gateway identity. It is a low-level leaf package so persistence and the
// gateway simulator can both depend on it without creating an import cycle.
package euid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const euiLen = 8 // bytes (16 hex chars)

// NormalizeEUI trims, lower-cases, and validates a 16-hex-char EUI-64 string.
func NormalizeEUI(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != euiLen*2 {
		return "", fmt.Errorf("eui %q must be %d hex chars", s, euiLen*2)
	}
	if _, err := hex.DecodeString(s); err != nil {
		return "", fmt.Errorf("eui %q is not valid hex: %w", s, err)
	}
	return s, nil
}

// GenerateEUI returns a random EUI-64 as a 16-hex-char lowercase string.
//
// An optional hex prefix seeds the leading bytes:
//   - A 3-byte (OUI) prefix is expanded Basics-Station-style by inserting FFFE
//     before the random tail (prefix + FFFE + 3 random bytes).
//   - Any other prefix length (up to 8 bytes) is used as the leading bytes with
//     the remainder filled randomly; an 8-byte prefix is returned verbatim.
func GenerateEUI(prefix string) (string, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))

	var head []byte
	if prefix != "" {
		if len(prefix)%2 != 0 || len(prefix) > euiLen*2 {
			return "", fmt.Errorf("eui prefix %q must be even-length hex up to %d chars", prefix, euiLen*2)
		}
		b, err := hex.DecodeString(prefix)
		if err != nil {
			return "", fmt.Errorf("eui prefix %q is not valid hex: %w", prefix, err)
		}
		head = b
	}

	// Station-style FFFE insertion for a 3-byte OUI prefix.
	if len(head) == 3 {
		head = append(head, 0xFF, 0xFE)
	}

	eui := make([]byte, euiLen)
	n := copy(eui, head)
	if n < euiLen {
		if _, err := rand.Read(eui[n:]); err != nil {
			return "", fmt.Errorf("reading random bytes for eui: %w", err)
		}
	}
	return hex.EncodeToString(eui), nil
}
