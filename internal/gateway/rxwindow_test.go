package gateway

import (
	"testing"

	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/transport"
)

func TestChooseRXWindow(t *testing.T) {
	cases := []struct {
		name   string
		dn     protocol.Dnmsg
		window string
		freq   uint32
	}{
		{"classA rx1", protocol.Dnmsg{DC: 0, RX1Freq: 868100000, RX1DR: 5, RX2Freq: 869525000}, transport.WindowRX1, 868100000},
		{"classA rx2 fallback", protocol.Dnmsg{DC: 0, RX1Freq: 0, RX2Freq: 869525000}, transport.WindowRX2, 869525000},
		{"classB ping", protocol.Dnmsg{DC: 1, RX2Freq: 869525000, RX2DR: 3}, transport.WindowClassB, 869525000},
		{"classC", protocol.Dnmsg{DC: 2, RX2Freq: 869525000, RX2DR: 0}, transport.WindowClassC, 869525000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ChooseRXWindow(&c.dn)
			if got.Window != c.window || got.Freq != c.freq {
				t.Errorf("ChooseRXWindow = %+v, want window %s freq %d", got, c.window, c.freq)
			}
		})
	}
}
