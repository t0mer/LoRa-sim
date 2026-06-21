package mocklns

import (
	"net/http"

	"github.com/t0mer/cylon/internal/gateway/protocol"
)

// CUPSConfig configures the update-info response the mock CUPS server returns.
type CUPSConfig struct {
	TcURI  string // LNS WebSocket URI to hand the station
	TcCred []byte // tc credential bundle (trust || cert || key, PEM)
}

// CUPSHandler returns an http.HandlerFunc that emulates the AWS CUPS endpoint:
// it answers /update-info with the binary CUPS response pointing the station at
// the configured LNS and credentials. Serve it over (mutual) TLS for a faithful
// offline bootstrap.
func CUPSHandler(cfg CUPSConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The request body carries the station's current CRCs; we always send
		// the configured tc credentials regardless (the offline mock has no
		// firmware/signature concerns).
		resp := protocol.BuildCupsResponse(protocol.CupsResponse{
			TcURI:  cfg.TcURI,
			TcCred: cfg.TcCred,
		})
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(resp)
	}
}
