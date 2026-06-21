package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// SPAHandler serves the embedded single-page app from fsys: static assets are
// served directly, and any other (non-API) path falls back to index.html so
// client-side routing works. If index.html is absent (only the placeholder is
// embedded), it returns a friendly notice.
func SPAHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(fsys, "index.html"); err != nil {
			http.Error(w, "Cylon UI is not built. Run `cd web && npm run build`.", http.StatusServiceUnavailable)
			return
		}
		upath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if upath == "" {
			upath = "index.html"
		}
		if _, err := fs.Stat(fsys, upath); err != nil {
			// Unknown path: serve index.html for SPA client-side routing.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
