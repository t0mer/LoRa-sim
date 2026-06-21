// Package classb implements the LoRaWAN Class B beacon and ping-slot timing
// math: the beacon period, the per-device randomized ping offset (AES-based),
// and the resulting ping-slot schedule within a beacon period.
//
// Cylon has no real radio, so ping slots are used to schedule when the network
// may reach a Class B device; in non-real-time mode downlinks are delivered
// immediately, but the computed slot timing is still surfaced (GPS-aligned).
package classb

import (
	"crypto/aes"
	"encoding/binary"
	"time"
)

const (
	// BeaconPeriod is the interval between beacons (128 s).
	BeaconPeriod = 128 * time.Second
	// BeaconReserved is the reserved time at the start of a beacon period before
	// the first ping slot may occur.
	BeaconReserved = 2120 * time.Millisecond
	// PingSlotLen is the duration of a single ping slot (30 ms).
	PingSlotLen = 30 * time.Millisecond
)

// PingNb returns the number of ping slots per beacon period for a given ping
// periodicity (0..7): 2^(7-periodicity), i.e. 128 down to 1.
func PingNb(periodicity uint8) int {
	if periodicity > 7 {
		periodicity = 7
	}
	return 1 << (7 - periodicity)
}

// PingPeriod returns the spacing (in slots) between a device's ping slots:
// 4096 / PingNb = 2^(5+periodicity).
func PingPeriod(periodicity uint8) int {
	return 4096 / PingNb(periodicity)
}

// PingOffset computes the randomized ping-slot offset (in slots) for a device in
// the beacon period starting at beaconTime (GPS seconds):
//
//	rand = aes128(0, beaconTime[4 LE] || devAddr[4] || 0×8)
//	offset = (rand[0] + rand[1]·256) mod pingPeriod
func PingOffset(beaconTime uint32, devAddr [4]byte, periodicity uint8) int {
	block := make([]byte, 16)
	binary.LittleEndian.PutUint32(block[0:4], beaconTime)
	copy(block[4:8], devAddr[:])

	cipher, _ := aes.NewCipher(make([]byte, 16)) // zero key; err only on bad key size
	out := make([]byte, 16)
	cipher.Encrypt(out, block)

	return (int(out[0]) + int(out[1])*256) % PingPeriod(periodicity)
}

// PingSlots returns the offsets from the beacon start at which the device opens
// each of its ping slots during one beacon period.
func PingSlots(beaconTime uint32, devAddr [4]byte, periodicity uint8) []time.Duration {
	offset := PingOffset(beaconTime, devAddr, periodicity)
	period := PingPeriod(periodicity)
	nb := PingNb(periodicity)

	slots := make([]time.Duration, 0, nb)
	for n := 0; n < nb; n++ {
		slot := BeaconReserved + time.Duration(offset+n*period)*PingSlotLen
		slots = append(slots, slot)
	}
	return slots
}

// NextPingSlot returns the offset (from the beacon start) of the first ping slot
// at or after since, plus whether one exists in this beacon period.
func NextPingSlot(beaconTime uint32, devAddr [4]byte, periodicity uint8, since time.Duration) (time.Duration, bool) {
	for _, s := range PingSlots(beaconTime, devAddr, periodicity) {
		if s >= since {
			return s, true
		}
	}
	return 0, false
}
