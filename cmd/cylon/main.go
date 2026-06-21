// Command cylon is the LoRaWAN simulator web-app binary.
//
// Phase 0 wires the CLI, bootstrap configuration, SQLite persistence with
// embedded migrations, first-run gateway-EUI generation, and a minimal health
// server. Later phases add the gateway/tag simulators, Basic Station client,
// REST/WebSocket API, and the embedded SPA.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
