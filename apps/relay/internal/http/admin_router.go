package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/store"
)

type AdminRouterConfig struct {
	Store      *store.Store
	Metrics    *metrics.Registry
	AdminToken string
	StreamHub  *StreamHub
	// Mode is returned by GET /admin/meta to make it obvious how the admin API is exposed.
	// Examples: "public_mount", "separate_listener".
	Mode string
}

func RegisterAdminRoutes(r chi.Router, cfg AdminRouterConfig) {
	mode := cfg.Mode
	if mode == "" {
		mode = "unknown"
	}

	h := NewAdminHandler(AdminHandlerConfig{
		Store:     cfg.Store,
		Metrics:   cfg.Metrics,
		StreamHub: cfg.StreamHub,
	})

	r.Route("/admin", func(r chi.Router) {
		r.Use(adminAuthMiddleware(cfg.AdminToken))

		r.Get("/meta", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"mode": mode})
		})

		r.Get("/overview", h.Overview)
		r.Get("/metrics", h.MetricsSnapshot)

		r.Get("/bots", h.ListBots)
		r.Get("/bots/{bot_id}", h.GetBot)
		r.Post("/bots/{bot_id}/revoke", h.RevokeBot)

		r.Get("/offers", h.ListOffers)
		r.Get("/offers/{offer_id}", h.GetOffer)
		r.Post("/offers/{offer_id}/pause", h.PauseOffer)
		r.Post("/offers/{offer_id}/resume", h.ResumeOffer)
		r.Post("/offers/{offer_id}/cancel", h.CancelOffer)

		r.Get("/jobs", h.ListJobs)
		r.Get("/jobs/{job_id}", h.GetJob)
		r.Post("/jobs/{job_id}/cancel", h.CancelJob)
		r.Post("/jobs/{job_id}/expire", h.ExpireJob)

		r.Get("/payloads", h.ListPayloads)
		r.Get("/events", h.ListEvents)
		r.Get("/audit", h.ListAudit)
	})
}

func NewAdminRouter(cfg AdminRouterConfig) http.Handler {
	if cfg.Mode == "" {
		cfg.Mode = "separate_listener"
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(errorLogMiddleware())
	r.Use(metricsMiddleware(cfg.Metrics))
	RegisterAdminRoutes(r, cfg)

	return r
}
