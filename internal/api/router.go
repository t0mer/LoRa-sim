package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter assembles the full HTTP surface: health, the REST API under /api,
// the WebSocket live feed at /ws, and the embedded SPA as a fallback. spa may be
// nil (no UI mounted).
func NewRouter(a *API, spa http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/healthz", a.healthz)

	r.Route("/api", func(r chi.Router) {
		r.Get("/gateway", a.getGateway)
		r.Put("/gateway", a.putGateway)

		r.Get("/tags", a.listTags)
		r.Post("/tags", a.createTags)
		r.Route("/tags/{id}", func(r chi.Router) {
			r.Get("/", a.getTag)
			r.Delete("/", a.deleteTag)
			r.Post("/join", a.joinTag)
			r.Post("/uplink", a.uplinkTag)
		})

		r.Get("/events", a.listEvents)
		r.Post("/scenarios/{name}/run", a.runScenario)
	})

	r.Get("/ws", a.hub.ServeWS)

	if spa != nil {
		r.Handle("/*", spa)
	}
	return r
}
