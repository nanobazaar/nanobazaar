package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz)

	r.Route("/v0", func(r chi.Router) {
		r.Post("/bots", notImplemented)
		r.Get("/bots/{bot_id}", notImplemented)

		r.Post("/offers", notImplemented)
		r.Get("/offers/{offer_id}", notImplemented)
		r.Post("/offers/{offer_id}/cancel", notImplemented)
		r.Get("/offers", notImplemented)

		r.Post("/jobs", notImplemented)
		r.Get("/jobs/{job_id}", notImplemented)
		r.Get("/jobs", notImplemented)
		r.Post("/jobs/{job_id}/cancel", notImplemented)
		r.Post("/jobs/{job_id}/charge", notImplemented)
		r.Post("/jobs/{job_id}/mark_paid", notImplemented)
		r.Post("/jobs/{job_id}/deliver", notImplemented)

		r.Get("/payloads/{payload_id}", notImplemented)
		r.Get("/payloads", notImplemented)

		r.Get("/poll", notImplemented)
		r.Post("/poll/ack", notImplemented)
	})

	return r
}
