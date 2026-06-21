// Package creds loads the LoRa Basics Station credential files from a mapped
// volume and builds the mutual-TLS configuration used to reach AWS IoT Core for
// LoRaWAN (CUPS and LNS endpoints).
//
// Layout in the credentials directory (the cert file may be .cert or .crt):
//
//	cups.uri  cups.trust  cups.cert|cups.crt  cups.key
//	tc.uri    tc.trust    tc.cert|tc.crt      tc.key
//
// cupsCredCrc / tcCredCrc are the CRC-32 (IEEE) over the concatenation of the
// trust, cert, and key for each endpoint, as reported to CUPS.
package creds

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
)

// Credentials holds the loaded Basic Station credential material.
type Credentials struct {
	CupsURI string
	TcURI   string

	CupsTrust []byte
	CupsCert  []byte
	CupsKey   []byte

	TcTrust []byte
	TcCert  []byte
	TcKey   []byte
}

// Load reads whatever credential files are present in dir. Missing files are
// left empty; callers check HasCUPS / HasTC for the mode they need.
func Load(dir string) (*Credentials, error) {
	c := &Credentials{}
	c.CupsURI = readURI(dir, "cups.uri")
	c.TcURI = readURI(dir, "tc.uri")

	c.CupsTrust = readFile(dir, "cups.trust")
	c.CupsCert = readCert(dir, "cups")
	c.CupsKey = readFile(dir, "cups.key")

	c.TcTrust = readFile(dir, "tc.trust")
	c.TcCert = readCert(dir, "tc")
	c.TcKey = readFile(dir, "tc.key")
	return c, nil
}

// HasCUPS reports whether the CUPS credential set is complete.
func (c *Credentials) HasCUPS() bool {
	return c.CupsURI != "" && len(c.CupsTrust) > 0 && len(c.CupsCert) > 0 && len(c.CupsKey) > 0
}

// HasTC reports whether the LNS (tc) credential set is complete.
func (c *Credentials) HasTC() bool {
	return c.TcURI != "" && len(c.TcTrust) > 0 && len(c.TcCert) > 0 && len(c.TcKey) > 0
}

// CupsCredCRC returns the CRC-32 over cups.trust || cups.cert || cups.key.
func (c *Credentials) CupsCredCRC() uint32 {
	return credCRC(c.CupsTrust, c.CupsCert, c.CupsKey)
}

// TcCredCRC returns the CRC-32 over tc.trust || tc.cert || tc.key.
func (c *Credentials) TcCredCRC() uint32 {
	return credCRC(c.TcTrust, c.TcCert, c.TcKey)
}

func credCRC(trust, cert, key []byte) uint32 {
	h := crc32.NewIEEE()
	h.Write(trust)
	h.Write(cert)
	h.Write(key)
	return h.Sum32()
}

// TCTLSConfig builds the mutual-TLS config for the LNS connection: the tc client
// certificate plus the tc trust anchor as the root CA pool.
func (c *Credentials) TCTLSConfig() (*tls.Config, error) {
	return tlsConfig(c.TcCert, c.TcKey, c.TcTrust)
}

// CUPSTLSConfig builds the mutual-TLS config for the CUPS connection.
func (c *Credentials) CUPSTLSConfig() (*tls.Config, error) {
	return tlsConfig(c.CupsCert, c.CupsKey, c.CupsTrust)
}

func tlsConfig(certPEM, keyPEM, trustPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("loading client keypair: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(trustPEM) {
		return nil, fmt.Errorf("no valid certificates in trust store")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// WriteTC writes back the tc.* credential material returned by CUPS into dir,
// so a subsequent LNS connection uses the freshly provisioned credentials.
// Empty fields are skipped (CUPS sends only what changed).
func WriteTC(dir, uri string, trust, cert, key []byte) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating creds dir: %w", err)
	}
	writes := []struct {
		name string
		data []byte
	}{
		{"tc.uri", []byte(uri)},
		{"tc.trust", trust},
		{"tc.cert", cert},
		{"tc.key", key},
	}
	for _, wr := range writes {
		if len(wr.data) == 0 {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, wr.name), wr.data, 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", wr.name, err)
		}
	}
	return nil
}

func readFile(dir, name string) []byte {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil
	}
	return b
}

// readCert reads <prefix>.cert, falling back to <prefix>.crt.
func readCert(dir, prefix string) []byte {
	if b := readFile(dir, prefix+".cert"); b != nil {
		return b
	}
	return readFile(dir, prefix+".crt")
}

func readURI(dir, name string) string {
	return strings.TrimSpace(string(readFile(dir, name)))
}
