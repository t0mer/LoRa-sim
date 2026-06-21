package api

import (
	"encoding/json"
	"net/http"

	"github.com/t0mer/cylon/internal/gateway"
	"github.com/t0mer/cylon/internal/sim"
	"github.com/t0mer/cylon/internal/store"
)

// API holds the dependencies for the REST handlers. orch and gw are nil when the
// gateway is disabled (no LNS configured); handlers that need them return 503.
type API struct {
	store   *store.Store
	hub     *Hub
	orch    *sim.Orchestrator
	gw      *gateway.Gateway
	version string
	eui     string
}

// NewAPI builds the API. orch and gw may be nil (gateway disabled).
func NewAPI(st *store.Store, hub *Hub, orch *sim.Orchestrator, gw *gateway.Gateway, version, eui string) *API {
	return &API{store: st, hub: hub, orch: orch, gw: gw, version: version, eui: eui}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
