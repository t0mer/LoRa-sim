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

// ChooseRXWindow selects the receive window for a downlink. Class C downlinks
// (dC==2) use the continuously-open RX2 window; Class A prefers RX1 and falls
// back to RX2. RX timing is owned by the gateway's synthetic clock; the radio
// parameters come straight from the LNS dnmsg.
func ChooseRXWindow(dn *protocol.Dnmsg) RXChoice {
	if dn.DC == 2 {
		return RXChoice{Window: transport.WindowClassC, Freq: dn.RX2Freq, DR: dn.RX2DR}
	}
	if dn.RX1Freq != 0 {
		return RXChoice{Window: transport.WindowRX1, Freq: dn.RX1Freq, DR: dn.RX1DR}
	}
	return RXChoice{Window: transport.WindowRX2, Freq: dn.RX2Freq, DR: dn.RX2DR}
}
