package api

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is the JSON body returned by GET /healthz.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	EUI     string `json:"eui"`
}

func (a *API) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Version: a.version, EUI: a.eui})
}
