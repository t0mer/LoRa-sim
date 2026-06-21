package creds

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"hash/crc32"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// genCert returns a self-signed cert PEM and its private key PEM.
func genCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cylon-test"},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func writeCredDir(t *testing.T, prefix string, certName string) string {
	t.Helper()
	dir := t.TempDir()
	cert, key := genCert(t)
	files := map[string][]byte{
		prefix + ".uri":   []byte("wss://example.lns.amazonaws.com:443\n"),
		prefix + ".trust": cert, // self-signed acts as its own trust anchor
		prefix + certName: cert,
		prefix + ".key":   key,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadAndTLSConfig(t *testing.T) {
	dir := writeCredDir(t, "tc", ".cert")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.HasTC() {
		t.Fatal("HasTC = false, want true")
	}
	if c.HasCUPS() {
		t.Error("HasCUPS = true, want false (no cups files)")
	}
	if c.TcURI != "wss://example.lns.amazonaws.com:443" {
		t.Errorf("TcURI = %q (should be trimmed)", c.TcURI)
	}
	cfg, err := c.TCTLSConfig()
	if err != nil {
		t.Fatalf("TCTLSConfig: %v", err)
	}
	if len(cfg.Certificates) != 1 || cfg.RootCAs == nil {
		t.Errorf("tls config incomplete: %+v", cfg)
	}
}

func TestCertCrtFallback(t *testing.T) {
	dir := writeCredDir(t, "cups", ".crt") // only .crt present
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.CupsCert) == 0 {
		t.Error("cups cert not loaded from .crt fallback")
	}
	if !c.HasCUPS() {
		t.Error("HasCUPS = false despite complete .crt set")
	}
}

func TestCredCRCOrder(t *testing.T) {
	c := &Credentials{
		CupsTrust: []byte("A"), CupsCert: []byte("B"), CupsKey: []byte("C"),
	}
	want := crc32.ChecksumIEEE([]byte("ABC"))
	if got := c.CupsCredCRC(); got != want {
		t.Errorf("CupsCredCRC = %08x, want %08x (trust||cert||key)", got, want)
	}
}

func TestWriteTCRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cert, key := genCert(t)
	if err := WriteTC(dir, "wss://new.lns:443", cert, cert, key); err != nil {
		t.Fatalf("WriteTC: %v", err)
	}
	c, _ := Load(dir)
	if !c.HasTC() {
		t.Fatal("written tc creds incomplete")
	}
	if c.TcURI != "wss://new.lns:443" {
		t.Errorf("TcURI = %q", c.TcURI)
	}
	if _, err := c.TCTLSConfig(); err != nil {
		t.Errorf("written creds do not form a valid TLS config: %v", err)
	}
}

func TestWriteTCSkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	// Pre-existing key that must survive a partial write-back.
	cert, key := genCert(t)
	os.WriteFile(filepath.Join(dir, "tc.key"), key, 0o600)
	if err := WriteTC(dir, "wss://x:443", cert, cert, nil); err != nil {
		t.Fatal(err)
	}
	if got := readFile(dir, "tc.key"); string(got) != string(key) {
		t.Error("empty key in WriteTC overwrote the existing tc.key")
	}
}
