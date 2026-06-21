package tag

import (
	"bytes"
	"testing"
)

func TestStaticGenerator(t *testing.T) {
	g, err := NewGenerator("static", `{"hex":"deadbeef"}`)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	for _, fcnt := range []uint32{0, 1, 99} {
		got, err := g.Next(fcnt)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, []byte{0xde, 0xad, 0xbe, 0xef}) {
			t.Errorf("static Next(%d) = % x, want deadbeef", fcnt, got)
		}
	}
}

func TestCounterGenerator(t *testing.T) {
	g, err := NewGenerator("counter", `{"size":2}`)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	got, _ := g.Next(0x1234)
	if !bytes.Equal(got, []byte{0x12, 0x34}) {
		t.Errorf("counter Next(0x1234) = % x, want 1234 (big-endian, size 2)", got)
	}
}

func TestCounterDefaultSize(t *testing.T) {
	g, _ := NewGenerator("counter", "")
	got, _ := g.Next(1)
	if len(got) != 4 {
		t.Errorf("default counter size = %d bytes, want 4", len(got))
	}
	if !bytes.Equal(got, []byte{0, 0, 0, 1}) {
		t.Errorf("counter Next(1) = % x, want 00000001", got)
	}
}

func TestRandomGenerator(t *testing.T) {
	g, err := NewGenerator("random", `{"len":8}`)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	a, _ := g.Next(0)
	b, _ := g.Next(0)
	if len(a) != 8 {
		t.Errorf("random len = %d, want 8", len(a))
	}
	if bytes.Equal(a, b) {
		t.Errorf("two random payloads identical; not random")
	}
}

func TestRampGenerator(t *testing.T) {
	g, _ := NewGenerator("ramp", `{"len":4}`)
	got, _ := g.Next(10)
	want := []byte{10, 11, 12, 13}
	if !bytes.Equal(got, want) {
		t.Errorf("ramp Next(10) = % x, want % x", got, want)
	}
}

func TestSineGenerator(t *testing.T) {
	g, err := NewGenerator("sine", `{"amplitude":100,"offset":1000,"period":4}`)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	// At fcnt=0 sin(0)=0 => sample = offset = 1000 = 0x03E8.
	got, _ := g.Next(0)
	if len(got) != 2 {
		t.Fatalf("sine output = %d bytes, want 2", len(got))
	}
	if got[0] != 0x03 || got[1] != 0xE8 {
		t.Errorf("sine Next(0) = % x, want 03E8 (offset)", got)
	}
}

func TestUnknownGenerator(t *testing.T) {
	if _, err := NewGenerator("nope", ""); err == nil {
		t.Errorf("NewGenerator(nope) = nil error, want error")
	}
}
