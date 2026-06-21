package main

import "net/http"

// spaHandler returns the embedded SPA handler, or nil when no UI is embedded yet
// (the frontend is added in a later step). When nil, the router mounts no
// catch-all and serves only /healthz, /api, and /ws.
func spaHandler() http.Handler {
	return nil
}
