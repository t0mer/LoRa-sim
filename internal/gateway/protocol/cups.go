package protocol

import (
	"encoding/binary"
	"encoding/pem"
	"fmt"
)

// CupsRequest is the JSON body POSTed to <cups.uri>/update-info.
type CupsRequest struct {
	Router      string   `json:"router"` // gateway EUI
	CupsURI     string   `json:"cupsUri"`
	TcURI       string   `json:"tcUri"`
	CupsCredCrc uint32   `json:"cupsCredCrc"`
	TcCredCrc   uint32   `json:"tcCredCrc"`
	Station     string   `json:"station"`
	Model       string   `json:"model"`
	Package     string   `json:"package"`
	Keys        []uint32 `json:"keys"`
}

// CupsResponse is the decoded CUPS update-info response. The wire format is a
// sequence of little-endian length-prefixed fields:
//
//	cupsUri   : uint8  len + bytes
//	tcUri     : uint8  len + bytes
//	cupsCred  : uint16 len + bytes
//	tcCred    : uint16 len + bytes
//	sig       : uint32 len + bytes
//	updData   : uint32 len + bytes
//
// A response of 14 zero bytes (all lengths zero) means "no update".
type CupsResponse struct {
	CupsURI  string
	TcURI    string
	CupsCred []byte
	TcCred   []byte
	Sig      []byte
	UpdData  []byte
}

// HasUpdate reports whether the response carries any change.
func (r *CupsResponse) HasUpdate() bool {
	return r.CupsURI != "" || r.TcURI != "" || len(r.CupsCred) > 0 ||
		len(r.TcCred) > 0 || len(r.Sig) > 0 || len(r.UpdData) > 0
}

// ParseCupsResponse decodes the binary CUPS update-info response.
func ParseCupsResponse(b []byte) (*CupsResponse, error) {
	c := &cursor{b: b}
	r := &CupsResponse{}
	var err error
	if r.CupsURI, err = c.lenString(1); err != nil {
		return nil, fmt.Errorf("cupsUri: %w", err)
	}
	if r.TcURI, err = c.lenString(1); err != nil {
		return nil, fmt.Errorf("tcUri: %w", err)
	}
	if r.CupsCred, err = c.lenBytes(2); err != nil {
		return nil, fmt.Errorf("cupsCred: %w", err)
	}
	if r.TcCred, err = c.lenBytes(2); err != nil {
		return nil, fmt.Errorf("tcCred: %w", err)
	}
	if r.Sig, err = c.lenBytes(4); err != nil {
		return nil, fmt.Errorf("sig: %w", err)
	}
	if r.UpdData, err = c.lenBytes(4); err != nil {
		return nil, fmt.Errorf("updData: %w", err)
	}
	return r, nil
}

// BuildCupsResponse encodes a CUPS response (used by tests and mock-lns).
func BuildCupsResponse(r CupsResponse) []byte {
	var out []byte
	out = appendLen(out, 1, []byte(r.CupsURI))
	out = appendLen(out, 1, []byte(r.TcURI))
	out = appendLen(out, 2, r.CupsCred)
	out = appendLen(out, 2, r.TcCred)
	out = appendLen(out, 4, r.Sig)
	out = appendLen(out, 4, r.UpdData)
	return out
}

func appendLen(dst []byte, lenBytes int, data []byte) []byte {
	var hdr [4]byte
	switch lenBytes {
	case 1:
		hdr[0] = byte(len(data))
	case 2:
		binary.LittleEndian.PutUint16(hdr[:2], uint16(len(data)))
	case 4:
		binary.LittleEndian.PutUint32(hdr[:4], uint32(len(data)))
	}
	dst = append(dst, hdr[:lenBytes]...)
	return append(dst, data...)
}

type cursor struct {
	b   []byte
	pos int
}

func (c *cursor) readN(n int) ([]byte, error) {
	if c.pos+n > len(c.b) {
		return nil, fmt.Errorf("unexpected end of data (need %d at %d, have %d)", n, c.pos, len(c.b))
	}
	out := c.b[c.pos : c.pos+n]
	c.pos += n
	return out, nil
}

func (c *cursor) length(lenBytes int) (int, error) {
	h, err := c.readN(lenBytes)
	if err != nil {
		return 0, err
	}
	switch lenBytes {
	case 1:
		return int(h[0]), nil
	case 2:
		return int(binary.LittleEndian.Uint16(h)), nil
	default:
		return int(binary.LittleEndian.Uint32(h)), nil
	}
}

func (c *cursor) lenBytes(lenBytes int) ([]byte, error) {
	n, err := c.length(lenBytes)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	data, err := c.readN(n)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), data...), nil
}

func (c *cursor) lenString(lenBytes int) (string, error) {
	b, err := c.lenBytes(lenBytes)
	return string(b), err
}

// SplitCredBundle splits a PEM credential bundle (trust CA, client cert, client
// key, concatenated) into its three parts. With multiple CERTIFICATE blocks the
// first is the trust anchor and the remainder is the client certificate chain;
// with a single one it serves as both.
func SplitCredBundle(bundle []byte) (trust, cert, key []byte, err error) {
	var certBlocks [][]byte
	rest := bundle
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		encoded := pem.EncodeToMemory(block)
		switch block.Type {
		case "CERTIFICATE":
			certBlocks = append(certBlocks, encoded)
		default:
			if key != nil {
				return nil, nil, nil, fmt.Errorf("multiple private keys in bundle")
			}
			key = encoded
		}
	}
	if len(certBlocks) == 0 || key == nil {
		return nil, nil, nil, fmt.Errorf("bundle missing certificate or key")
	}
	if len(certBlocks) == 1 {
		return certBlocks[0], certBlocks[0], key, nil
	}
	trust = certBlocks[0]
	for _, c := range certBlocks[1:] {
		cert = append(cert, c...)
	}
	return trust, cert, key, nil
}
