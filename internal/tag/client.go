package tag

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync/atomic"

	"github.com/t0mer/cylon/internal/transport"
)

// Default uplink radio parameters (EU868 channel 0). The tag picks freq/DR; the
// gateway validates against the channel plan. Channel hopping is a later refinement.
const defaultUplinkFreq = 868100000

// Client drives a Device over a tag<->gateway TCP connection: it sends hello,
// performs OTAA join, sends uplinks, and dispatches downlinks. Run must be
// active (in its own goroutine) for Join/downlinks to complete.
type Client struct {
	dev  *Device
	conn *transport.Conn
	freq uint32

	joined atomic.Bool
	joinCh chan []byte
}

// NewClient wraps a Device and a connected transport.Conn.
func NewClient(dev *Device, conn *transport.Conn) *Client {
	return &Client{
		dev:    dev,
		conn:   conn,
		freq:   defaultUplinkFreq,
		joinCh: make(chan []byte, 1),
	}
}

// Hello announces the device to the gateway so downlinks can be routed.
func (c *Client) Hello() error {
	return c.conn.Write(transport.Envelope{
		Type:   transport.TypeHello,
		DevEUI: c.dev.tag.DevEUI,
		Class:  c.dev.tag.Class,
		Region: c.dev.tag.Region,
	})
}

// Run reads downlinks until the connection closes or ctx is done. Before the
// device has joined, a downlink is treated as the join-accept; afterwards it is
// decoded as data.
func (c *Client) Run(ctx context.Context) error {
	for {
		env, err := c.conn.Read()
		if err != nil {
			return err
		}
		if env.Type != transport.TypeDown {
			continue
		}
		pdu, err := hex.DecodeString(env.Phy)
		if err != nil {
			continue
		}
		if !c.joined.Load() {
			select {
			case c.joinCh <- pdu:
			default:
			}
			continue
		}
		if _, err := c.dev.HandleDownlink(ctx, pdu); err != nil {
			// Stale/invalid downlinks are dropped; keep serving.
			continue
		}
	}
}

// Join performs an OTAA join: it sends the join-request uplink and waits for the
// join-accept downlink (which Run delivers), then persists the session.
func (c *Client) Join(ctx context.Context) error {
	frame, devNonce, err := c.dev.BuildJoinRequest(ctx)
	if err != nil {
		return err
	}
	if err := c.sendUp(frame); err != nil {
		return err
	}
	select {
	case pdu := <-c.joinCh:
		if err := c.dev.CompleteJoin(ctx, devNonce, pdu); err != nil {
			return err
		}
		c.joined.Store(true)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for join-accept: %w", ctx.Err())
	}
}

// SendUplink builds and transmits a data uplink. With override nil the payload
// comes from the device's generator.
func (c *Client) SendUplink(ctx context.Context, override []byte) error {
	if !c.joined.Load() {
		return fmt.Errorf("cannot send uplink before join")
	}
	frame, _, err := c.dev.BuildUplink(ctx, override)
	if err != nil {
		return err
	}
	return c.sendUp(frame)
}

func (c *Client) sendUp(frame []byte) error {
	return c.conn.Write(transport.Envelope{
		Type:   transport.TypeUp,
		DevEUI: c.dev.tag.DevEUI,
		Phy:    hex.EncodeToString(frame),
		Freq:   c.freq,
		DR:     uint8(c.dev.tag.DefaultDR),
		CodR:   "4/5",
	})
}
