package main

import (
	"net/http"

	"github.com/t0mer/cylon/internal/api"
	"github.com/t0mer/cylon/internal/webui"
)

// spaHandler returns the embedded SPA handler. When no build is embedded (only
// the placeholder), it still mounts and returns a friendly notice.
func spaHandler() http.Handler {
	return api.SPAHandler(webui.FS())
}
