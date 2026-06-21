// Package api serves cylon's HTTP surface. Phase 0 exposes only a health
// endpoint so that `cylon serve` is observably running; later phases add the
// full REST API, the WebSocket hub, and the embedded SPA.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// HealthResponse is the JSON body returned by GET /healthz.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	EUI     string `json:"eui"`
}

// NewRouter builds the HTTP handler. In Phase 0 it serves GET /healthz with the
// build version and the active gateway EUI.
func NewRouter(version, eui string) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:  "ok",
			Version: version,
			EUI:     eui,
		})
	})
	return r
}
