package gateway

import "github.com/t0mer/cylon/internal/gateway/protocol"

// ChannelPlan is the gateway's view of the LNS router_config: the region, the
// NetID and JoinEUI filters, and the data-rate table. Phase 2 uses it for
// (optional, default-off) uplink filtering and surfacing the plan to the UI.
type ChannelPlan struct {
	Region        string
	NetIDs        []uint32
	JoinEUIRanges [][2]uint64
	FreqRange     [2]uint32
	DRs           [][]int
}

// ParseChannelPlan extracts a ChannelPlan from a router_config message.
func ParseChannelPlan(rc *protocol.RouterConfig) ChannelPlan {
	cp := ChannelPlan{
		Region: rc.Region,
		NetIDs: append([]uint32(nil), rc.NetID...),
		DRs:    rc.DRs,
	}
	for _, r := range rc.JoinEui {
		if len(r) == 2 {
			cp.JoinEUIRanges = append(cp.JoinEUIRanges, [2]uint64{r[0], r[1]})
		}
	}
	if len(rc.FreqRange) == 2 {
		cp.FreqRange = [2]uint32{rc.FreqRange[0], rc.FreqRange[1]}
	}
	return cp
}

// AllowsJoinEUI reports whether eui falls within a configured JoinEUI range. A
// plan with no ranges allows everything.
func (cp ChannelPlan) AllowsJoinEUI(eui uint64) bool {
	if len(cp.JoinEUIRanges) == 0 {
		return true
	}
	for _, r := range cp.JoinEUIRanges {
		if eui >= r[0] && eui <= r[1] {
			return true
		}
	}
	return false
}
