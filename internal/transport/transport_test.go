package transport

import (
	"sync"
	"testing"
	"time"
)

func TestClientServerRoundTrip(t *testing.T) {
	received := make(chan Envelope, 4)
	var once sync.Once
	done := make(chan struct{})

	srv, err := Listen("127.0.0.1:0", func(c *Conn) {
		for {
			env, err := c.Read()
			if err != nil {
				once.Do(func() { close(done) })
				return
			}
			received <- env
			if env.Type == TypeUp {
				// Echo a downlink back to the tag.
				_ = c.Write(Envelope{Type: TypeDown, DevEUI: env.DevEUI, Phy: "ddeeff", Window: WindowRX1})
			}
		}
	})
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer srv.Close()

	cli, err := Dial(srv.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer cli.Close()

	if err := cli.Write(Envelope{Type: TypeHello, DevEUI: "0102030405060708", Class: "A", Region: "EU868"}); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	if err := cli.Write(Envelope{Type: TypeUp, DevEUI: "0102030405060708", Phy: "aabbcc", Freq: 868100000, DR: 5}); err != nil {
		t.Fatalf("write up: %v", err)
	}

	down, err := cli.Read()
	if err != nil {
		t.Fatalf("read down: %v", err)
	}
	if down.Type != TypeDown || down.Phy != "ddeeff" || down.Window != WindowRX1 {
		t.Errorf("downlink = %+v", down)
	}

	hello := <-received
	if hello.Type != TypeHello || hello.Region != "EU868" {
		t.Errorf("hello = %+v", hello)
	}
	up := <-received
	if up.Type != TypeUp || up.Freq != 868100000 || up.DR != 5 {
		t.Errorf("up = %+v", up)
	}
}

func TestServerCloseUnblocks(t *testing.T) {
	srv, err := Listen("127.0.0.1:0", func(c *Conn) {
		for {
			if _, err := c.Read(); err != nil {
				return
			}
		}
	})
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	cli, _ := Dial(srv.Addr())
	defer cli.Close()

	closed := make(chan error, 1)
	go func() { closed <- srv.Close() }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return within 2s")
	}
}
