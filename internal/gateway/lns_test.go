package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/t0mer/cylon/internal/gateway/protocol"
)

// fakeLNS is an httptest server that performs the LNS side of the handshake:
// read version -> send router_config -> read jreq -> send dnmsg.
func fakeLNS(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()

		// Expect the version message first.
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		if msg, _ := protocol.Decode(data); msg != nil {
			if _, ok := msg.(*protocol.Version); !ok {
				t.Errorf("first message = %T, want *Version", msg)
			}
		}

		rc := protocol.RouterConfig{
			MsgType: protocol.TypeRouterConfig, Region: "EU863",
			NetID: []uint32{1}, HwSpec: "sx1301/1",
			FreqRange: []uint32{863000000, 870000000},
		}
		b, _ := protocol.Encode(rc)
		_ = c.Write(ctx, websocket.MessageText, b)

		// Echo a downlink in response to a join-request.
		_, data, err = c.Read(ctx)
		if err != nil {
			return
		}
		msg, _ := protocol.Decode(data)
		jr, ok := msg.(*protocol.Jreq)
		if !ok {
			t.Errorf("uplink = %T, want *Jreq", msg)
			return
		}
		dn := protocol.Dnmsg{MsgType: protocol.TypeDnmsg, DevEui: jr.DevEui, Pdu: "aabbcc", DIID: 7, RX1Freq: 868100000}
		b, _ = protocol.Encode(dn)
		_ = c.Write(ctx, websocket.MessageText, b)

		<-ctx.Done()
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func TestLNSHandshakeAndDownlink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lns, err := ConnectLNS(ctx, fakeLNS(t), protocol.Version{Station: "cylon", Model: "sim", Protocol: 2})
	if err != nil {
		t.Fatalf("ConnectLNS: %v", err)
	}
	defer lns.Close()

	rc := lns.RouterConfig()
	if rc == nil || rc.Region != "EU863" {
		t.Fatalf("router_config = %+v, want region EU863", rc)
	}

	got := make(chan *protocol.Dnmsg, 1)
	go func() { _ = lns.Run(func(d *protocol.Dnmsg) { got <- d }) }()

	if err := lns.SendJreq(protocol.Jreq{DevEui: "0102030405060708", DevNonce: 1}); err != nil {
		t.Fatalf("SendJreq: %v", err)
	}

	select {
	case dn := <-got:
		if dn.Pdu != "aabbcc" || dn.DIID != 7 {
			t.Errorf("dnmsg = %+v", dn)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive dnmsg within 3s")
	}
}

func TestConnectLNSDialFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := ConnectLNS(ctx, "ws://127.0.0.1:1", protocol.Version{}); err == nil {
		t.Errorf("ConnectLNS to dead endpoint = nil error, want failure")
	}
}
