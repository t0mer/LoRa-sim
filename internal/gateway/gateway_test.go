package gateway

import (
	"encoding/hex"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/brocaar/lorawan"

	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/tag"
	"github.com/t0mer/cylon/internal/transport"
)

type fakeSender struct {
	mu     sync.Mutex
	jreq   chan protocol.Jreq
	updf   chan protocol.Updf
	dntxed []protocol.Dntxed
}

func newFakeSender() *fakeSender {
	return &fakeSender{jreq: make(chan protocol.Jreq, 4), updf: make(chan protocol.Updf, 4)}
}

func (f *fakeSender) SendJreq(j protocol.Jreq) error { f.jreq <- j; return nil }
func (f *fakeSender) SendUpdf(u protocol.Updf) error { f.updf <- u; return nil }
func (f *fakeSender) SendDntxed(d protocol.Dntxed) error {
	f.mu.Lock()
	f.dntxed = append(f.dntxed, d)
	f.mu.Unlock()
	return nil
}

func quietGateway(t *testing.T, s lnsSender) *Gateway {
	t.Helper()
	g := New("0000000000000001", s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := g.ListenTCP("127.0.0.1:0"); err != nil {
		t.Fatalf("ListenTCP: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

func TestGatewayRelaysJoinAndRoutesDownlink(t *testing.T) {
	sender := newFakeSender()
	gw := quietGateway(t, sender)

	cli, err := transport.Dial(gw.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

	const devEUIHex = "0102030405060708"
	prov := tag.Provisioning{
		DevEUI:  lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
		JoinEUI: lorawan.EUI64{8, 7, 6, 5, 4, 3, 2, 1},
		AppKey:  lorawan.AES128Key{9},
	}
	frame, _ := tag.BuildJoinRequest(prov, 1)

	if err := cli.Write(transport.Envelope{Type: transport.TypeHello, DevEUI: devEUIHex, Class: "A"}); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if err := cli.Write(transport.Envelope{Type: transport.TypeUp, DevEUI: devEUIHex, Phy: hex.EncodeToString(frame), Freq: 868100000, DR: 5}); err != nil {
		t.Fatalf("up: %v", err)
	}

	select {
	case jr := <-sender.jreq:
		if jr.DevEui != devEUIHex {
			t.Errorf("forwarded jreq DevEui = %q, want %q", jr.DevEui, devEUIHex)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("gateway did not forward jreq")
	}

	// Route a downlink back to the tag (hello already registered the conn).
	gw.OnDnmsg(&protocol.Dnmsg{DevEui: devEUIHex, Pdu: "aabbcc", DIID: 11, RX1Freq: 868100000, RX1DR: 5})

	down, err := cli.Read()
	if err != nil {
		t.Fatalf("read downlink: %v", err)
	}
	if down.Type != transport.TypeDown || down.Phy != "aabbcc" || down.Window != transport.WindowRX1 {
		t.Errorf("downlink envelope = %+v", down)
	}
	if down.Freq != 868100000 || down.DR != 5 {
		t.Errorf("downlink radio = freq %d dr %d", down.Freq, down.DR)
	}

	sender.mu.Lock()
	n := len(sender.dntxed)
	sender.mu.Unlock()
	if n != 1 {
		t.Errorf("dntxed count = %d, want 1", n)
	}
}

func TestGatewayForwardsDataUplink(t *testing.T) {
	sender := newFakeSender()
	gw := quietGateway(t, sender)
	cli, _ := transport.Dial(gw.Addr())
	defer cli.Close()

	const devEUIHex = "1111111111111111"
	frame, _ := tag.BuildUplink(tag.UplinkParams{
		DevAddr: lorawan.DevAddr{4, 3, 2, 1},
		NwkSKey: lorawan.AES128Key{1},
		AppSKey: lorawan.AES128Key{2},
		FCnt:    0,
		FPort:   10,
		Data:    []byte("hi"),
	})
	cli.Write(transport.Envelope{Type: transport.TypeHello, DevEUI: devEUIHex, Class: "A"})
	cli.Write(transport.Envelope{Type: transport.TypeUp, DevEUI: devEUIHex, Phy: hex.EncodeToString(frame), Freq: 868300000, DR: 4})

	select {
	case up := <-sender.updf:
		if up.FPort != 10 || up.DevAddr == 0 {
			t.Errorf("forwarded updf = %+v", up)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("gateway did not forward updf")
	}
}
