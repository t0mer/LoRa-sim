package gateway

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0mer/cylon/internal/creds"
	"github.com/t0mer/cylon/internal/gateway/protocol"
)

func genPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "t"}, IsCA: true, BasicConstraintsValid: true}
	der, _ := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func credBundle(t *testing.T) []byte {
	cert, key := genPEM(t)
	bundle := append([]byte(nil), cert...)
	return append(bundle, key...)
}

func TestPostCUPS(t *testing.T) {
	bundle := credBundle(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/update-info" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req protocol.CupsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Router != "0102030405060708" {
			t.Errorf("router = %q", req.Router)
		}
		_, _ = w.Write(protocol.BuildCupsResponse(protocol.CupsResponse{TcURI: "wss://new.lns:443", TcCred: bundle}))
	}))
	defer srv.Close()

	resp, err := PostCUPS(context.Background(), srv.Client(), srv.URL, protocol.CupsRequest{Router: "0102030405060708"})
	if err != nil {
		t.Fatalf("PostCUPS: %v", err)
	}
	if resp.TcURI != "wss://new.lns:443" {
		t.Errorf("TcURI = %q", resp.TcURI)
	}
	if !resp.HasUpdate() {
		t.Error("HasUpdate = false")
	}
}

func TestApplyTCUpdateWritesBack(t *testing.T) {
	dir := t.TempDir()
	resp := &protocol.CupsResponse{TcURI: "wss://x.lns:443", TcCred: credBundle(t)}
	if err := applyTCUpdate(dir, resp); err != nil {
		t.Fatalf("applyTCUpdate: %v", err)
	}
	c, _ := creds.Load(dir)
	if !c.HasTC() {
		t.Fatal("tc creds not written back")
	}
	if c.TcURI != "wss://x.lns:443" {
		t.Errorf("TcURI = %q", c.TcURI)
	}
	if _, err := c.TCTLSConfig(); err != nil {
		t.Errorf("written-back creds invalid: %v", err)
	}
}

func TestApplyTCUpdateNoopOnEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := applyTCUpdate(dir, &protocol.CupsResponse{}); err != nil {
		t.Errorf("applyTCUpdate(empty) = %v, want nil", err)
	}
}
