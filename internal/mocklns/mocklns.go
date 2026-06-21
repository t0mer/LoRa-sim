// Package mocklns emulates AWS IoT Core for LoRaWAN's LNS endpoint for offline
// development and CI. It speaks the Basic Station LNS protocol: it answers the
// version handshake with a router_config, replies to a join-request with a
// signed+encrypted join-accept dnmsg, and accepts data uplinks.
//
// It is intentionally a single-device, single-key emulator (the AppKey is
// configured) — enough to exercise the full join/uplink/downlink path without
// AWS.
package mocklns

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/brocaar/lorawan"
	"github.com/coder/websocket"

	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/tag"
)

// Config configures the emulated network server.
type Config struct {
	Region    string
	NetID     lorawan.NetID
	AppKey    lorawan.AES128Key
	DevAddr   lorawan.DevAddr
	JoinNonce uint32
	RX2Freq   uint32
	RX2DR     uint8
	RxDelay   int
	Logger    *slog.Logger
}

func (c *Config) withDefaults() {
	if c.Region == "" {
		c.Region = "EU863"
	}
	if c.RX2Freq == 0 {
		c.RX2Freq = 869525000
	}
	if c.RxDelay == 0 {
		c.RxDelay = 1
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

// joinInfo remembers what the mock issued for a device so it can later derive
// that device's session keys (e.g. to encrypt an unsolicited Class C downlink).
type joinInfo struct {
	joinNonce uint32
	devNonce  uint16
	fcntDown  uint32
}

// Server is the mock LNS. Use Handler with an http.Server (or httptest).
type Server struct {
	cfg       Config
	joinNonce atomic.Uint32
	diid      atomic.Int64

	mu       sync.Mutex
	updfs    []protocol.Updf // received data uplinks (for assertions)
	conn     *websocket.Conn // most recent gateway connection
	wmu      sync.Mutex      // serializes writes to conn
	sessions map[string]*joinInfo
}

// New builds a mock LNS from cfg.
func New(cfg Config) *Server {
	cfg.withDefaults()
	s := &Server{cfg: cfg, sessions: make(map[string]*joinInfo)}
	s.joinNonce.Store(cfg.JoinNonce)
	return s
}

// Uplinks returns a copy of the data uplinks received so far.
func (s *Server) Uplinks() []protocol.Updf {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocol.Updf(nil), s.updfs...)
}

// Handler returns the WebSocket handler implementing the LNS side.
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		if err := s.serve(r.Context(), c); err != nil &&
			!errors.Is(err, context.Canceled) {
			s.cfg.Logger.Debug("mock-lns session ended", "err", err)
		}
	}
}

func (s *Server) serve(ctx context.Context, c *websocket.Conn) error {
	c.SetReadLimit(1 << 20)

	s.mu.Lock()
	s.conn = c
	s.mu.Unlock()

	// Expect version, then push router_config.
	if _, _, err := c.Read(ctx); err != nil {
		return err
	}
	rc, err := protocol.Encode(s.routerConfig())
	if err != nil {
		return err
	}
	if err := s.write(ctx, c, rc); err != nil {
		return err
	}

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return err
		}
		msg, err := protocol.Decode(data)
		if err != nil {
			continue
		}
		switch m := msg.(type) {
		case *protocol.Jreq:
			if err := s.handleJreq(ctx, c, m); err != nil {
				return err
			}
		case *protocol.Updf:
			s.mu.Lock()
			s.updfs = append(s.updfs, *m)
			s.mu.Unlock()
		}
	}
}

func (s *Server) routerConfig() protocol.RouterConfig {
	return protocol.RouterConfig{
		MsgType:   protocol.TypeRouterConfig,
		Region:    s.cfg.Region,
		NetID:     []uint32{netIDToUint32(s.cfg.NetID)},
		JoinEui:   [][]uint64{{0, ^uint64(0)}},
		HwSpec:    "sx1301/1",
		FreqRange: []uint32{863000000, 870000000},
		DRs:       [][]int{{12, 125, 0}, {11, 125, 0}, {10, 125, 0}, {9, 125, 0}, {8, 125, 0}, {7, 125, 0}},
		NoCCA:     true, NoDC: true, NoDwell: true, MaxEIRP: 16,
	}
}

func (s *Server) handleJreq(ctx context.Context, c *websocket.Conn, jr *protocol.Jreq) error {
	joinNonce := s.joinNonce.Add(1)
	accept, err := s.buildJoinAccept(jr, joinNonce)
	if err != nil {
		return fmt.Errorf("building join-accept: %w", err)
	}
	s.mu.Lock()
	s.sessions[jr.DevEui] = &joinInfo{joinNonce: joinNonce, devNonce: jr.DevNonce}
	s.mu.Unlock()
	dn := protocol.Dnmsg{
		MsgType: protocol.TypeDnmsg,
		DevEui:  jr.DevEui,
		DC:      0,
		DIID:    s.diid.Add(1),
		Pdu:     hex.EncodeToString(accept),
		RxDelay: s.cfg.RxDelay,
		RX1DR:   jr.DR,
		RX1Freq: jr.Freq,
		RX2DR:   s.cfg.RX2DR,
		RX2Freq: s.cfg.RX2Freq,
		XTime:   jr.UpInfo.XTime,
		RCtx:    jr.UpInfo.RCtx,
	}
	b, err := protocol.Encode(dn)
	if err != nil {
		return err
	}
	return s.write(ctx, c, b)
}

// write serializes writes to the gateway connection (one writer at a time).
func (s *Server) write(ctx context.Context, c *websocket.Conn, b []byte) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return c.Write(ctx, websocket.MessageText, b)
}

// PushClassC sends an unsolicited Class C (dC=2) data downlink to a joined
// device over the RX2 window. It derives the device's session keys from the
// join it issued and encrypts the payload, exercising the always-on RX path.
func (s *Server) PushClassC(ctx context.Context, devEUI string, fport uint8, data []byte) error {
	s.mu.Lock()
	conn := s.conn
	ji := s.sessions[devEUI]
	if ji != nil {
		ji.fcntDown++
	}
	s.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no gateway connected")
	}
	if ji == nil {
		return fmt.Errorf("device %s has not joined", devEUI)
	}

	nwk, err := tag.DeriveNwkSKey(s.cfg.AppKey, s.cfg.NetID, lorawan.JoinNonce(ji.joinNonce), lorawan.DevNonce(ji.devNonce))
	if err != nil {
		return err
	}
	app, err := tag.DeriveAppSKey(s.cfg.AppKey, s.cfg.NetID, lorawan.JoinNonce(ji.joinNonce), lorawan.DevNonce(ji.devNonce))
	if err != nil {
		return err
	}
	pdu, err := buildDataDownlink(nwk, app, s.cfg.DevAddr, ji.fcntDown-1, fport, data)
	if err != nil {
		return err
	}
	dn := protocol.Dnmsg{
		MsgType: protocol.TypeDnmsg, DevEui: devEUI, DC: 2, DIID: s.diid.Add(1),
		Pdu: hex.EncodeToString(pdu), RX2DR: s.cfg.RX2DR, RX2Freq: s.cfg.RX2Freq,
	}
	b, err := protocol.Encode(dn)
	if err != nil {
		return err
	}
	return s.write(ctx, conn, b)
}

func buildDataDownlink(nwk, app lorawan.AES128Key, devAddr lorawan.DevAddr, fcnt uint32, fport uint8, data []byte) ([]byte, error) {
	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{MType: lorawan.UnconfirmedDataDown, Major: lorawan.LoRaWANR1},
		MACPayload: &lorawan.MACPayload{
			FHDR:       lorawan.FHDR{DevAddr: devAddr, FCnt: fcnt},
			FPort:      &fport,
			FRMPayload: []lorawan.Payload{&lorawan.DataPayload{Bytes: data}},
		},
	}
	if err := phy.EncryptFRMPayload(app); err != nil {
		return nil, err
	}
	if err := phy.SetDownlinkDataMIC(lorawan.LoRaWAN1_0, 0, nwk); err != nil {
		return nil, err
	}
	return phy.MarshalBinary()
}

func (s *Server) buildJoinAccept(jr *protocol.Jreq, joinNonce uint32) ([]byte, error) {
	joinEUI, err := parseEUI64(jr.JoinEui)
	if err != nil {
		return nil, err
	}
	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{MType: lorawan.JoinAccept, Major: lorawan.LoRaWANR1},
		MACPayload: &lorawan.JoinAcceptPayload{
			JoinNonce:  lorawan.JoinNonce(joinNonce),
			HomeNetID:  s.cfg.NetID,
			DevAddr:    s.cfg.DevAddr,
			DLSettings: lorawan.DLSettings{RX2DataRate: s.cfg.RX2DR, RX1DROffset: 0},
			RXDelay:    uint8(s.cfg.RxDelay),
		},
	}
	if err := phy.SetDownlinkJoinMIC(lorawan.JoinRequestType, joinEUI, lorawan.DevNonce(jr.DevNonce), s.cfg.AppKey); err != nil {
		return nil, err
	}
	if err := phy.EncryptJoinAcceptPayload(s.cfg.AppKey); err != nil {
		return nil, err
	}
	return phy.MarshalBinary()
}

func parseEUI64(s string) (lorawan.EUI64, error) {
	var e lorawan.EUI64
	b, err := hex.DecodeString(s)
	if err != nil {
		return e, fmt.Errorf("parse EUI64 %q: %w", s, err)
	}
	if len(b) != len(e) {
		return e, fmt.Errorf("EUI64 %q must be 8 bytes", s)
	}
	copy(e[:], b)
	return e, nil
}

func netIDToUint32(n lorawan.NetID) uint32 {
	return uint32(n[0])<<16 | uint32(n[1])<<8 | uint32(n[2])
}
