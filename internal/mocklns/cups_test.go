package mocklns_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/t0mer/cylon/internal/creds"
	"github.com/t0mer/cylon/internal/gateway"
	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/mocklns"
)

type ca struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newCA(t *testing.T) *ca {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "cylon-test-ca"},
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	cert, _ := x509.ParseCertificate(der)
	return &ca{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

func (c *ca) issue(t *testing.T, cn string, server bool, ips []net.IP) (certPEM, keyPEM []byte) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: cn},
		IPAddresses: ips,
		NotBefore:   time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
	}
	if server {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, c.cert, &key.PublicKey, c.key)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// TestCUPSBootstrapOverMutualTLS proves the full offline CUPS bootstrap: the
// gateway authenticates to the mock CUPS endpoint with mutual TLS, receives the
// tc credentials, and writes them back so a subsequent LNS connection is ready.
func TestCUPSBootstrapOverMutualTLS(t *testing.T) {
	ctx := context.Background()
	root := newCA(t)
	serverCert, serverKey := root.issue(t, "cups", true, []net.IP{net.ParseIP("127.0.0.1")})
	clientCert, clientKey := root.issue(t, "gateway", false, nil)

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(root.pem)
	srvTLSCert, _ := tls.X509KeyPair(serverCert, serverKey)

	// tc credential bundle the CUPS server hands back: trust(CA) || cert || key.
	tcBundle := append(append(append([]byte(nil), root.pem...), clientCert...), clientKey...)

	srv := httptest.NewUnstartedServer(mocklns.CUPSHandler(mocklns.CUPSConfig{
		TcURI: "wss://lns.example.amazonaws.com:443", TcCred: tcBundle,
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{srvTLSCert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	srv.StartTLS()
	defer srv.Close()

	// Credentials volume: cups.* present, tc.* absent (filled by bootstrap).
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("cups.uri", []byte(srv.URL))
	write("cups.trust", root.pem)
	write("cups.cert", clientCert)
	write("cups.key", clientKey)

	resp, err := gateway.BootstrapCUPS(ctx, dir, protocol.CupsRequest{
		Router: "0102030405060708", Station: "cylon", Model: "cylon-sim",
	})
	if err != nil {
		t.Fatalf("BootstrapCUPS: %v", err)
	}
	if resp.TcURI != "wss://lns.example.amazonaws.com:443" {
		t.Errorf("response TcURI = %q", resp.TcURI)
	}

	// The tc.* credentials must now be written and usable.
	c, _ := creds.Load(dir)
	if !c.HasTC() {
		t.Fatal("tc credentials were not written back")
	}
	if c.TcURI != "wss://lns.example.amazonaws.com:443" {
		t.Errorf("written TcURI = %q", c.TcURI)
	}
	if _, err := c.TCTLSConfig(); err != nil {
		t.Errorf("written-back tc creds do not form a valid TLS config: %v", err)
	}
}
