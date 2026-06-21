package gateway

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/transport"
)

// Gateway bridges TCP-connected tags to the LNS: it parses tag uplinks into
// Basic Station jreq/updf messages, forwards them, and routes LNS dnmsg
// downlinks back to the addressed tag over its TCP connection.
type Gateway struct {
	eui string
	lns lnsSender
	log *slog.Logger

	srv *transport.Server

	mu           sync.RWMutex
	byDevEUI     map[string]*transport.Conn
	devAddrToEUI map[string]string

	xtime atomic.Int64 // synthetic clock for uplink xtime
}

// lnsSender is the subset of *LNS the gateway uses to reach the network server
// (an interface so the server can be unit-tested without a live WebSocket).
type lnsSender interface {
	SendJreq(protocol.Jreq) error
	SendUpdf(protocol.Updf) error
	SendDntxed(protocol.Dntxed) error
}

// New creates a Gateway with the given EUI and LNS sender.
func New(eui string, lns lnsSender, log *slog.Logger) *Gateway {
	if log == nil {
		log = slog.Default()
	}
	return &Gateway{
		eui:          eui,
		lns:          lns,
		log:          log,
		byDevEUI:     make(map[string]*transport.Conn),
		devAddrToEUI: make(map[string]string),
	}
}

// ListenTCP starts the tag-facing TCP server on addr.
func (g *Gateway) ListenTCP(addr string) error {
	srv, err := transport.Listen(addr, g.handleConn)
	if err != nil {
		return err
	}
	g.srv = srv
	g.log.Info("gateway tcp listening", "addr", srv.Addr(), "eui", g.eui)
	return nil
}

// Addr returns the tag-facing TCP listen address.
func (g *Gateway) Addr() string { return g.srv.Addr() }

// Close stops the TCP server.
func (g *Gateway) Close() error {
	if g.srv != nil {
		return g.srv.Close()
	}
	return nil
}

// handleConn reads NDJSON envelopes from a tag connection until it closes.
func (g *Gateway) handleConn(c *transport.Conn) {
	var devEUI string
	defer func() {
		if devEUI != "" {
			g.unregister(devEUI)
		}
	}()

	for {
		env, err := c.Read()
		if err != nil {
			if err != io.EOF {
				g.log.Debug("tag conn read", "err", err)
			}
			return
		}
		switch env.Type {
		case transport.TypeHello:
			devEUI = env.DevEUI
			g.register(devEUI, c)
			if env.DevAddr != "" {
				g.bindDevAddr(env.DevAddr, devEUI)
			}
			g.log.Debug("tag hello", "dev_eui", devEUI, "class", env.Class)
		case transport.TypeUp:
			if err := g.handleUplink(env); err != nil {
				g.log.Warn("uplink", "dev_eui", env.DevEUI, "err", err)
			}
		default:
			g.log.Debug("ignoring tag message", "type", env.Type)
		}
	}
}

func (g *Gateway) handleUplink(env transport.Envelope) error {
	xtime := g.xtime.Add(1)
	info := SynthUpInfo(0, xtime)

	pu, err := ParseUplink(env.Phy, env.Freq, env.DR, info)
	if err != nil {
		return err
	}
	switch msg := pu.Message.(type) {
	case protocol.Jreq:
		return g.lns.SendJreq(msg)
	case protocol.Updf:
		if pu.DevAddr != "" {
			g.bindDevAddr(pu.DevAddr, env.DevEUI)
		}
		return g.lns.SendUpdf(msg)
	default:
		return fmt.Errorf("unexpected parsed message %T", pu.Message)
	}
}

// OnDnmsg routes a downlink from the LNS to the addressed tag and confirms the
// transmission. It is the callback passed to LNS.Run.
func (g *Gateway) OnDnmsg(dn *protocol.Dnmsg) {
	conn, ok := g.lookup(dn.DevEui)
	if !ok {
		g.log.Warn("downlink for unknown tag", "dev_eui", dn.DevEui)
		return
	}
	rx := ChooseRXWindow(dn)
	if err := conn.Write(transport.Envelope{
		Type:   transport.TypeDown,
		DevEUI: dn.DevEui,
		Phy:    dn.Pdu,
		Window: rx.Window,
		Freq:   rx.Freq,
		DR:     rx.DR,
	}); err != nil {
		g.log.Warn("routing downlink", "dev_eui", dn.DevEui, "err", err)
		return
	}
	if err := g.lns.SendDntxed(protocol.Dntxed{
		DIID:   dn.DIID,
		DevEui: dn.DevEui,
		RCtx:   dn.RCtx,
		XTime:  dn.XTime,
	}); err != nil {
		g.log.Debug("dntxed", "err", err)
	}
}

func (g *Gateway) register(devEUI string, c *transport.Conn) {
	g.mu.Lock()
	g.byDevEUI[devEUI] = c
	g.mu.Unlock()
}

func (g *Gateway) unregister(devEUI string) {
	g.mu.Lock()
	delete(g.byDevEUI, devEUI)
	g.mu.Unlock()
}

func (g *Gateway) bindDevAddr(devAddr, devEUI string) {
	g.mu.Lock()
	g.devAddrToEUI[devAddr] = devEUI
	g.mu.Unlock()
}

func (g *Gateway) lookup(devEUI string) (*transport.Conn, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	c, ok := g.byDevEUI[devEUI]
	return c, ok
}
