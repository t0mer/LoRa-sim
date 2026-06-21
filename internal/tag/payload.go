package tag

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// Generator produces an uplink payload for a given uplink frame counter.
// Implementations must be safe to call repeatedly.
type Generator interface {
	Next(fcnt uint32) ([]byte, error)
}

// NewGenerator builds a payload Generator from a tag's payload_type and the
// JSON payload_config column. An empty config uses per-type defaults.
//
// Supported types:
//
//	static  {"hex": "deadbeef"}              fixed bytes
//	counter {"size": 4}                      fcnt big-endian in size bytes
//	random  {"len": 8}                       len cryptographically-random bytes
//	ramp    {"len": 4}                       bytes fcnt, fcnt+1, … (mod 256)
//	sine    {"amplitude","offset","period"}  uint16 big-endian sample
func NewGenerator(typ, configJSON string) (Generator, error) {
	raw := []byte(configJSON)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	switch typ {
	case "static":
		var c struct {
			Hex string `json:"hex"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("static payload config: %w", err)
		}
		b, err := hex.DecodeString(c.Hex)
		if err != nil {
			return nil, fmt.Errorf("static payload hex: %w", err)
		}
		return &staticGen{data: b}, nil

	case "counter":
		var c struct {
			Size int `json:"size"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("counter payload config: %w", err)
		}
		if c.Size <= 0 {
			c.Size = 4
		}
		return &counterGen{size: c.Size}, nil

	case "random":
		var c struct {
			Len int `json:"len"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("random payload config: %w", err)
		}
		if c.Len <= 0 {
			c.Len = 8
		}
		return &randomGen{n: c.Len}, nil

	case "ramp":
		var c struct {
			Len int `json:"len"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("ramp payload config: %w", err)
		}
		if c.Len <= 0 {
			c.Len = 4
		}
		return &rampGen{n: c.Len}, nil

	case "sine":
		var c struct {
			Amplitude float64 `json:"amplitude"`
			Offset    float64 `json:"offset"`
			Period    int     `json:"period"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("sine payload config: %w", err)
		}
		if c.Amplitude == 0 {
			c.Amplitude = 100
		}
		if c.Period <= 0 {
			c.Period = 60
		}
		return &sineGen{amplitude: c.Amplitude, offset: c.Offset, period: c.Period}, nil

	default:
		return nil, fmt.Errorf("unknown payload type %q", typ)
	}
}

type staticGen struct{ data []byte }

func (g *staticGen) Next(uint32) ([]byte, error) {
	out := make([]byte, len(g.data))
	copy(out, g.data)
	return out, nil
}

type counterGen struct{ size int }

func (g *counterGen) Next(fcnt uint32) ([]byte, error) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, fcnt)
	if g.size >= 4 {
		out := make([]byte, g.size)
		copy(out[g.size-4:], buf)
		return out, nil
	}
	return buf[4-g.size:], nil
}

type randomGen struct{ n int }

func (g *randomGen) Next(uint32) ([]byte, error) {
	b := make([]byte, g.n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("random payload: %w", err)
	}
	return b, nil
}

type rampGen struct{ n int }

func (g *rampGen) Next(fcnt uint32) ([]byte, error) {
	b := make([]byte, g.n)
	for i := range b {
		b[i] = byte((fcnt + uint32(i)) & 0xff)
	}
	return b, nil
}

type sineGen struct {
	amplitude float64
	offset    float64
	period    int
}

func (g *sineGen) Next(fcnt uint32) ([]byte, error) {
	phase := 2 * math.Pi * float64(int(fcnt)%g.period) / float64(g.period)
	v := g.offset + g.amplitude*math.Sin(phase)
	v = math.Round(v)
	if v < 0 {
		v = 0
	}
	if v > math.MaxUint16 {
		v = math.MaxUint16
	}
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, uint16(v))
	return out, nil
}
