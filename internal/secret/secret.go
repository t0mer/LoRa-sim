// Package secret provides authenticated at-rest encryption for sensitive
// database columns (AppKey, NwkSKey, AppSKey).
//
// The key comes from CYLON_DB_KEY and must be a 32-byte key encoded as 64 hex
// chars or as base64 (generate one with `openssl rand -hex 32`). When no key is
// configured the cipher operates in pass-through mode so dev setups still work —
// callers should warn loudly in that case.
//
// Each value is bound to a caller-supplied AAD (associated data), e.g. the row
// identity, so an encrypted value cannot be moved to a different row/column
// without failing authentication. Stored ciphertext carries a version prefix;
// once a key is configured every value must be prefixed (an unprefixed value is
// a hard error, never silently trusted plaintext).
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// prefix marks an encrypted, base64-encoded value.
const prefix = "v1:"

// keyLen is the required key length in bytes (AES-256).
const keyLen = 32

// Cipher encrypts and decrypts secret column values. Construct one with New.
type Cipher struct {
	aead    cipher.AEAD
	enabled bool
}

// New builds an encrypting Cipher from a non-empty key that decodes to exactly
// 32 bytes (hex or base64). An empty key is rejected — callers that intend to
// run without encryption must opt in explicitly via NewInsecureDisabled so that
// fail-open never happens silently.
func New(key string) (*Cipher, error) {
	if key == "" {
		return nil, fmt.Errorf("secret: no key configured; set CYLON_DB_KEY, or call NewInsecureDisabled for dev plaintext")
	}
	raw, err := decodeKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, fmt.Errorf("creating aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating gcm: %w", err)
	}
	return &Cipher{aead: aead, enabled: true}, nil
}

// NewInsecureDisabled returns a pass-through Cipher that does NOT encrypt. It
// exists for dev/CI without a configured key; callers MUST log a loud warning
// before using it. Decrypting an encrypted (prefixed) value with it is an error.
func NewInsecureDisabled() *Cipher {
	return &Cipher{enabled: false}
}

func decodeKey(s string) ([]byte, error) {
	if len(s) == hex.EncodedLen(keyLen) {
		if b, err := hex.DecodeString(s); err == nil {
			return b, nil
		}
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == keyLen {
			return b, nil
		}
	}
	return nil, fmt.Errorf("secret: CYLON_DB_KEY must be a %d-byte key (64 hex chars or base64)", keyLen)
}

// Enabled reports whether encryption is active.
func (c *Cipher) Enabled() bool { return c.enabled }

// Encrypt returns the protected form of plaintext, bound to aad. With encryption
// disabled the plaintext is returned unchanged.
func (c *Cipher) Encrypt(plaintext string, aad []byte) (string, error) {
	if !c.enabled {
		return plaintext, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("reading nonce: %w", err)
	}
	ct := c.aead.Seal(nonce, nonce, []byte(plaintext), aad)
	return prefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt using the same aad. With encryption disabled, values
// are returned unchanged. With encryption enabled, an unprefixed value is a hard
// error — plaintext is never silently trusted once a key is configured.
func (c *Cipher) Decrypt(stored string, aad []byte) (string, error) {
	if !strings.HasPrefix(stored, prefix) {
		if c.enabled {
			return "", fmt.Errorf("secret: value is not encrypted but a key is configured")
		}
		return stored, nil
	}
	if !c.enabled {
		return "", fmt.Errorf("secret: value is encrypted but no key is configured (set CYLON_DB_KEY)")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := c.aead.Open(nil, nonce, ct, aad)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}
	return string(pt), nil
}
