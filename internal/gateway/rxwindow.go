package gateway

import (
	"github.com/t0mer/cylon/internal/gateway/protocol"
	"github.com/t0mer/cylon/internal/transport"
)

// RXChoice is the receive window the gateway selects for a downlink: which
// window to use plus the radio parameters supplied by the LNS.
type RXChoice struct {
	Window string
	Freq   uint32
	DR     uint8
}

// ChooseRXWindow selects the receive window for a downlink:
//   - dC==2 (Class C): the continuously-open RX2 window.
//   - dC==1 (Class B): a ping slot; radio params come from the dnmsg (carried in
//     the RX2 fields) and the slot time is GPS-aligned.
//   - otherwise (Class A): prefer RX1, fall back to RX2.
//
// RX timing is owned by the gateway's synthetic clock; the radio parameters come
// straight from the LNS dnmsg.
func ChooseRXWindow(dn *protocol.Dnmsg) RXChoice {
	switch dn.DC {
	case 2:
		return RXChoice{Window: transport.WindowClassC, Freq: dn.RX2Freq, DR: dn.RX2DR}
	case 1:
		return RXChoice{Window: transport.WindowClassB, Freq: dn.RX2Freq, DR: dn.RX2DR}
	}
	if dn.RX1Freq != 0 {
		return RXChoice{Window: transport.WindowRX1, Freq: dn.RX1Freq, DR: dn.RX1DR}
	}
	return RXChoice{Window: transport.WindowRX2, Freq: dn.RX2Freq, DR: dn.RX2DR}
}
