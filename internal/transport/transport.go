// Package transport implements the internal tag<->gateway link: newline-
// delimited JSON (NDJSON), one object per line, over TCP. It is Cylon-internal
// and distinct from the Basic Station protocol the gateway speaks to the LNS.
//
// The gateway (not the tag) synthesizes radio metadata (rssi/snr/xtime/rctx);
// routing is by DevEUI.
package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Message types for the Type field.
const (
	TypeHello = "hello"
	TypeUp    = "up"
	TypeDown  = "down"
)

// Downlink window markers.
const (
	WindowRX1    = "rx1"
	WindowRX2    = "rx2"
	WindowClassC = "classc"
)

// Envelope is a single NDJSON line. Field presence depends on Type:
//   - hello: DevEUI, DevAddr (optional), Class, Region
//   - up:    DevEUI, Phy, Freq, DR, CodR, At
//   - down:  DevEUI, Phy, Window, Freq, DR, At
type Envelope struct {
	Type    string `json:"type"`
	DevEUI  string `json:"dev_eui"`
	DevAddr string `json:"dev_addr,omitempty"`
	Class   string `json:"class,omitempty"`
	Region  string `json:"region,omitempty"`
	Phy     string `json:"phy,omitempty"`
	Freq    uint32 `json:"freq,omitempty"`
	DR      uint8  `json:"dr,omitempty"`
	CodR    string `json:"codr,omitempty"`
	Window  string `json:"window,omitempty"`
	At      int64  `json:"at,omitempty"`
}

// Conn is a framed NDJSON connection. It is safe for one reader goroutine and
// concurrent writers (writes are serialized).
type Conn struct {
	rwc io.ReadWriteCloser
	r   *bufio.Reader
	wmu sync.Mutex
}

// NewConn wraps an io.ReadWriteCloser (e.g. a net.Conn) as a framed connection.
func NewConn(rwc io.ReadWriteCloser) *Conn {
	return &Conn{rwc: rwc, r: bufio.NewReader(rwc)}
}

// Write serializes and sends one envelope followed by a newline.
func (c *Conn) Write(env Envelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", env.Type, err)
	}
	b = append(b, '\n')
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := c.rwc.Write(b); err != nil {
		return fmt.Errorf("writing %s: %w", env.Type, err)
	}
	return nil
}

// Read reads and decodes the next envelope. It returns io.EOF at end of stream.
func (c *Conn) Read() (Envelope, error) {
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		if len(line) == 0 {
			return Envelope{}, err
		}
		// A final line without a trailing newline is still valid.
	}
	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return Envelope{}, fmt.Errorf("decoding envelope: %w", err)
	}
	return env, nil
}

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.rwc.Close() }
