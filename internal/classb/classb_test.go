package classb

import (
	"testing"
	"time"
)

func TestPingNbAndPeriod(t *testing.T) {
	cases := []struct {
		periodicity uint8
		nb, period  int
	}{
		{0, 128, 32},
		{1, 64, 64},
		{7, 1, 4096},
	}
	for _, c := range cases {
		if got := PingNb(c.periodicity); got != c.nb {
			t.Errorf("PingNb(%d) = %d, want %d", c.periodicity, got, c.nb)
		}
		if got := PingPeriod(c.periodicity); got != c.period {
			t.Errorf("PingPeriod(%d) = %d, want %d", c.periodicity, got, c.period)
		}
	}
}

func TestPingOffsetKnownAnswer(t *testing.T) {
	// AES-128 of an all-zero block under an all-zero key is
	// 66e94bd4...; out[0]=0x66, out[1]=0xe9, so for periodicity 0
	// (pingPeriod 32): (0x66 + 0xe9*256) % 32 = 6.
	if got := PingOffset(0, [4]byte{}, 0); got != 6 {
		t.Errorf("PingOffset(0, zero, 0) = %d, want 6", got)
	}
}

func TestPingOffsetVariesWithDevAddr(t *testing.T) {
	a := PingOffset(1000, [4]byte{1, 2, 3, 4}, 2)
	b := PingOffset(1000, [4]byte{4, 3, 2, 1}, 2)
	if a == b {
		t.Errorf("ping offset identical for different DevAddrs (%d)", a)
	}
	if a >= PingPeriod(2) || b >= PingPeriod(2) {
		t.Errorf("offset out of range: a=%d b=%d period=%d", a, b, PingPeriod(2))
	}
}

func TestPingSlotsCountAndSpacing(t *testing.T) {
	slots := PingSlots(0, [4]byte{}, 7) // periodicity 7 => exactly 1 slot
	if len(slots) != 1 {
		t.Fatalf("PingSlots periodicity 7 = %d slots, want 1", len(slots))
	}
	if slots[0] < BeaconReserved {
		t.Errorf("first slot %v before BeaconReserved %v", slots[0], BeaconReserved)
	}

	many := PingSlots(0, [4]byte{}, 0) // 128 slots, spacing 32 slots
	if len(many) != 128 {
		t.Fatalf("periodicity 0 = %d slots, want 128", len(many))
	}
	gap := many[1] - many[0]
	if gap != time.Duration(PingPeriod(0))*PingSlotLen {
		t.Errorf("slot spacing = %v, want %v", gap, time.Duration(PingPeriod(0))*PingSlotLen)
	}
}

func TestNextPingSlot(t *testing.T) {
	slots := PingSlots(0, [4]byte{}, 0)
	next, ok := NextPingSlot(0, [4]byte{}, 0, 0)
	if !ok || next != slots[0] {
		t.Errorf("NextPingSlot(since 0) = (%v, %v), want first slot %v", next, ok, slots[0])
	}
	// since past the last slot => none.
	if _, ok := NextPingSlot(0, [4]byte{}, 0, slots[len(slots)-1]+time.Second); ok {
		t.Error("NextPingSlot past last slot = ok, want false")
	}
}
