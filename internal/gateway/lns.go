package gateway

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coder/websocket"

	"github.com/t0mer/cylon/internal/gateway/protocol"
)

// LNS is the gateway's Basic Station client to a LoRaWAN Network Server (real
// AWS in LNS-direct mode, or the offline mock-lns). It performs the version ->
// router_config handshake on connect, then sends uplinks and receives downlinks.
type LNS struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	wmu sync.Mutex

	cmu    sync.Mutex
	config *protocol.RouterConfig
}

// ConnectLNS dials the LNS WebSocket endpoint, sends the version message, and
// waits for the router_config that establishes the channel plan.
func ConnectLNS(parent context.Context, url string, v protocol.Version) (*LNS, error) {
	ctx, cancel := context.WithCancel(parent)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("dialing LNS %s: %w", url, err)
	}
	// Basic Station messages (router_config with sx1301_conf) can exceed the
	// default 32 KiB read limit.
	conn.SetReadLimit(1 << 20)

	l := &LNS{conn: conn, ctx: ctx, cancel: cancel}

	v.MsgType = protocol.TypeVersion
	if err := l.send(v); err != nil {
		l.Close()
		return nil, fmt.Errorf("sending version: %w", err)
	}
	rc, err := l.awaitRouterConfig()
	if err != nil {
		l.Close()
		return nil, err
	}
	l.setConfig(rc)
	return l, nil
}

func (l *LNS) awaitRouterConfig() (*protocol.RouterConfig, error) {
	for {
		_, data, err := l.conn.Read(l.ctx)
		if err != nil {
			return nil, fmt.Errorf("reading router_config: %w", err)
		}
		msg, err := protocol.Decode(data)
		if err != nil {
			continue // ignore unknown/unsupported messages during handshake
		}
		if rc, ok := msg.(*protocol.RouterConfig); ok {
			return rc, nil
		}
	}
}

// RouterConfig returns the most recently received channel plan.
func (l *LNS) RouterConfig() *protocol.RouterConfig {
	l.cmu.Lock()
	defer l.cmu.Unlock()
	return l.config
}

func (l *LNS) setConfig(rc *protocol.RouterConfig) {
	l.cmu.Lock()
	l.config = rc
	l.cmu.Unlock()
}

// Run reads from the LNS until the context is cancelled or the connection drops,
// invoking onDnmsg for each downlink. router_config updates are applied
// in-place. It returns the terminating error (nil on clean shutdown).
func (l *LNS) Run(onDnmsg func(*protocol.Dnmsg)) error {
	for {
		_, data, err := l.conn.Read(l.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("reading from LNS: %w", err)
		}
		msg, err := protocol.Decode(data)
		if err != nil {
			continue // tolerate unknown message types (runcmd, timesync, …)
		}
		switch m := msg.(type) {
		case *protocol.Dnmsg:
			if onDnmsg != nil {
				onDnmsg(m)
			}
		case *protocol.RouterConfig:
			l.setConfig(m)
		}
	}
}

// SendJreq forwards a join-request uplink to the LNS.
func (l *LNS) SendJreq(j protocol.Jreq) error {
	j.MsgType = protocol.TypeJreq
	return l.send(j)
}

// SendUpdf forwards a data uplink to the LNS.
func (l *LNS) SendUpdf(u protocol.Updf) error {
	u.MsgType = protocol.TypeUpdf
	return l.send(u)
}

// SendDntxed confirms a downlink transmission to the LNS.
func (l *LNS) SendDntxed(d protocol.Dntxed) error {
	d.MsgType = protocol.TypeDntxed
	return l.send(d)
}

func (l *LNS) send(msg any) error {
	b, err := protocol.Encode(msg)
	if err != nil {
		return err
	}
	l.wmu.Lock()
	defer l.wmu.Unlock()
	return l.conn.Write(l.ctx, websocket.MessageText, b)
}

// Close cancels the client context and closes the WebSocket.
func (l *LNS) Close() error {
	l.cancel()
	return l.conn.Close(websocket.StatusNormalClosure, "bye")
}
