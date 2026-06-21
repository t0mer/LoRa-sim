// Command mock-lns emulates AWS IoT Core for LoRaWAN's LNS endpoint over a
// WebSocket, for offline development and CI. It answers the version handshake
// with a router_config and replies to join-requests with a signed join-accept.
package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/brocaar/lorawan"
	"github.com/spf13/pflag"

	"github.com/t0mer/cylon/internal/mocklns"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	listen := pflag.String("listen", ":7000", "WebSocket listen address")
	region := pflag.String("region", "EU863", "RF region advertised in router_config")
	appKeyHex := pflag.String("app-key", "", "device AppKey (32 hex chars) used to sign join-accepts (required)")
	netIDHex := pflag.String("netid", "000001", "NetID (6 hex chars)")
	devAddrHex := pflag.String("devaddr", "01020304", "DevAddr assigned in join-accept (8 hex chars)")
	rx2Freq := pflag.Uint32("rx2-freq", 869525000, "RX2 frequency in Hz")
	pflag.Parse()

	if *appKeyHex == "" {
		return fmt.Errorf("--app-key is required")
	}
	appKey, err := parseKey(*appKeyHex)
	if err != nil {
		return err
	}
	netID, err := parseNetID(*netIDHex)
	if err != nil {
		return err
	}
	devAddr, err := parseDevAddr(*devAddrHex)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := mocklns.New(mocklns.Config{
		Region: *region, NetID: netID, AppKey: appKey, DevAddr: devAddr,
		RX2Freq: *rx2Freq, Logger: logger,
	})

	logger.Info("mock-lns listening", "addr", *listen, "region", *region)
	return http.ListenAndServe(*listen, srv.Handler())
}

func parseKey(s string) (lorawan.AES128Key, error) {
	var k lorawan.AES128Key
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(k) {
		return k, fmt.Errorf("app-key must be 32 hex chars")
	}
	copy(k[:], b)
	return k, nil
}

func parseNetID(s string) (lorawan.NetID, error) {
	var n lorawan.NetID
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(n) {
		return n, fmt.Errorf("netid must be 6 hex chars")
	}
	copy(n[:], b)
	return n, nil
}

func parseDevAddr(s string) (lorawan.DevAddr, error) {
	var d lorawan.DevAddr
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(d) {
		return d, fmt.Errorf("devaddr must be 8 hex chars")
	}
	copy(d[:], b)
	return d, nil
}
