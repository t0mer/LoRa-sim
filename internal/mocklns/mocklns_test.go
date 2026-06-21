package mocklns_test

import (
	"context"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brocaar/lorawan"

	"github.com/t0mer/cylon/internal/gateway"
	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/mocklns"
	"github.com/t0mer/cylon/internal/tag"
)

func TestMockLNSJoinAcceptParsable(t *testing.T) {
	appKey := lorawan.AES128Key{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	devAddr := lorawan.DevAddr{1, 2, 3, 4}

	lnsSrv := mocklns.New(mocklns.Config{
		Region:  "EU863",
		NetID:   lorawan.NetID{0, 0, 1},
		AppKey:  appKey,
		DevAddr: devAddr,
	})
	httpSrv := httptest.NewServer(lnsSrv.Handler())
	defer httpSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lns, err := gateway.ConnectLNS(ctx, wsURL, protocol.Version{Station: "cylon", Protocol: 2})
	if err != nil {
		t.Fatalf("ConnectLNS: %v", err)
	}
	defer lns.Close()
	if rc := lns.RouterConfig(); rc == nil || rc.Region != "EU863" {
		t.Fatalf("router_config = %+v", rc)
	}

	prov := tag.Provisioning{
		DevEUI:  lorawan.EUI64{1, 1, 1, 1, 1, 1, 1, 1},
		JoinEUI: lorawan.EUI64{2, 2, 2, 2, 2, 2, 2, 2},
		AppKey:  appKey,
	}
	const devNonce = 1
	frame, _ := tag.BuildJoinRequest(prov, devNonce)
	pu, err := gateway.ParseUplink(hex.EncodeToString(frame), 868100000, 5, gateway.SynthUpInfo(0, 1))
	if err != nil {
		t.Fatalf("ParseUplink: %v", err)
	}

	got := make(chan *protocol.Dnmsg, 1)
	go func() { _ = lns.Run(func(d *protocol.Dnmsg) { got <- d }) }()

	if err := lns.SendJreq(pu.Message.(protocol.Jreq)); err != nil {
		t.Fatalf("SendJreq: %v", err)
	}

	var dn *protocol.Dnmsg
	select {
	case dn = <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("no join-accept dnmsg")
	}

	acceptBytes, err := hex.DecodeString(dn.Pdu)
	if err != nil {
		t.Fatalf("decode pdu: %v", err)
	}
	res, err := tag.ParseJoinAccept(prov, devNonce, acceptBytes)
	if err != nil {
		t.Fatalf("tag could not parse mock join-accept: %v", err)
	}
	if res.DevAddr != devAddr {
		t.Errorf("DevAddr = %v, want %v", res.DevAddr, devAddr)
	}
	if dn.RX1Freq != 868100000 {
		t.Errorf("RX1Freq = %d, want uplink freq 868100000", dn.RX1Freq)
	}
}
