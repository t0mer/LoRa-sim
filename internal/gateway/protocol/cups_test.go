package protocol

import (
	"bytes"
	"testing"
)

func TestCupsResponseRoundTrip(t *testing.T) {
	in := CupsResponse{
		TcURI:  "wss://lns.example.com:443",
		TcCred: []byte("trust+cert+key blob"),
		Sig:    []byte{1, 2, 3, 4},
	}
	encoded := BuildCupsResponse(in)
	got, err := ParseCupsResponse(encoded)
	if err != nil {
		t.Fatalf("ParseCupsResponse: %v", err)
	}
	if got.TcURI != in.TcURI {
		t.Errorf("TcURI = %q, want %q", got.TcURI, in.TcURI)
	}
	if !bytes.Equal(got.TcCred, in.TcCred) {
		t.Errorf("TcCred = %q", got.TcCred)
	}
	if !bytes.Equal(got.Sig, in.Sig) {
		t.Errorf("Sig = %v", got.Sig)
	}
	if !got.HasUpdate() {
		t.Error("HasUpdate = false, want true")
	}
}

func TestCupsNoUpdateIs14ZeroBytes(t *testing.T) {
	encoded := BuildCupsResponse(CupsResponse{})
	if len(encoded) != 14 {
		t.Errorf("empty response = %d bytes, want 14 (1+1+2+2+4+4)", len(encoded))
	}
	for i, b := range encoded {
		if b != 0 {
			t.Errorf("byte %d = %d, want 0", i, b)
		}
	}
	got, err := ParseCupsResponse(encoded)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.HasUpdate() {
		t.Error("HasUpdate = true for all-zero response, want false")
	}
}

func TestParseCupsResponseTruncated(t *testing.T) {
	// A length prefix claiming more data than present must error, not panic.
	bad := []byte{0xff} // cupsUri len 255 but no data
	if _, err := ParseCupsResponse(bad); err == nil {
		t.Error("ParseCupsResponse(truncated) = nil error, want error")
	}
}

func TestSplitCredBundle(t *testing.T) {
	trust := []byte("-----BEGIN CERTIFICATE-----\nQUFB\n-----END CERTIFICATE-----\n")
	cert := []byte("-----BEGIN CERTIFICATE-----\nQkJC\n-----END CERTIFICATE-----\n")
	key := []byte("-----BEGIN PRIVATE KEY-----\nQ0ND\n-----END PRIVATE KEY-----\n")
	bundle := bytes.Join([][]byte{trust, cert, key}, nil)

	gotTrust, gotCert, gotKey, err := SplitCredBundle(bundle)
	if err != nil {
		t.Fatalf("SplitCredBundle: %v", err)
	}
	if !bytes.Contains(gotTrust, []byte("QUFB")) {
		t.Errorf("trust = %q, want the first cert", gotTrust)
	}
	if !bytes.Contains(gotCert, []byte("QkJC")) {
		t.Errorf("cert = %q, want the second cert", gotCert)
	}
	if !bytes.Contains(gotKey, []byte("Q0ND")) {
		t.Errorf("key = %q", gotKey)
	}
}

func TestSplitCredBundleSingleCert(t *testing.T) {
	cert := []byte("-----BEGIN CERTIFICATE-----\nQUFB\n-----END CERTIFICATE-----\n")
	key := []byte("-----BEGIN PRIVATE KEY-----\nQ0ND\n-----END PRIVATE KEY-----\n")
	trust, gotCert, _, err := SplitCredBundle(append(append([]byte(nil), cert...), key...))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(trust, gotCert) {
		t.Error("single cert should serve as both trust and cert")
	}
}

func TestSplitCredBundleMissingKey(t *testing.T) {
	cert := []byte("-----BEGIN CERTIFICATE-----\nQUFB\n-----END CERTIFICATE-----\n")
	if _, _, _, err := SplitCredBundle(cert); err == nil {
		t.Error("SplitCredBundle without key = nil error, want error")
	}
}
