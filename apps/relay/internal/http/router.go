package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/ratelimit"
	"github.com/nanobazaar/relay/internal/store"
)

type Option func(*JobHandler)

func WithClock(clock func() time.Time) Option {
	return func(handler *JobHandler) {
		handler.Clock = clock
	}
}

type RouterConfig struct {
	Verifier     *auth.Verifier
	Store        *store.Store
	Metrics      *metrics.Registry
	Limiter      *ratelimit.Limiter
	HealthPublic bool
}

func NewRouter(cfg RouterConfig, opts ...Option) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(metricsMiddleware(cfg.Metrics))

	bots := NewBotHandler(cfg.Store)
	offers := NewOfferHandler(cfg.Store)
	jobs := NewJobHandler(cfg.Store, cfg.Metrics)
	payloads := NewPayloadHandler(cfg.Store, cfg.Metrics)
	poll := NewPollHandler(cfg.Store, cfg.Metrics)
	stats := NewStatsHandler(cfg.Store)
	for _, opt := range opts {
		opt(jobs)
	}

	r.With(healthMiddleware(cfg.HealthPublic)).Get("/healthz", healthz)
	r.With(healthMiddleware(cfg.HealthPublic)).Get("/readyz", readyz)
	r.Get("/stats", stats.Get)
	r.With(rateLimitMiddleware(cfg.Limiter, ratelimit.BucketOfferSearch, cfg.Metrics)).Get("/market/offers", offers.PublicList)

	r.Route("/v0", func(r chi.Router) {
		r.Use(auth.Middleware(cfg.Verifier))

		rlPoll := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketPollAck, cfg.Metrics)
		rlOffers := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketOfferSearch, cfg.Metrics)
		rlWrites := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketWrites, cfg.Metrics)
		rlPayloads := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketPayloadFetch, cfg.Metrics)

		r.With(rlWrites).Post("/bots", bots.Register)
		r.Get("/bots/{bot_id}", bots.Get)
		r.With(rlWrites).Post("/bots/{bot_id}/revoke", bots.Revoke)

		r.With(rlWrites).Post("/offers", offers.Create)
		r.Get("/offers/{offer_id}", offers.Get)
		r.With(rlWrites).Post("/offers/{offer_id}/cancel", offers.Cancel)
		r.With(rlWrites).Post("/offers/{offer_id}/pause", offers.Pause)
		r.With(rlWrites).Post("/offers/{offer_id}/resume", offers.Resume)
		r.With(rlOffers).Get("/offers", offers.List)

		r.With(rlWrites).Post("/jobs", jobs.Create)
		r.Get("/jobs/{job_id}", jobs.Get)
		r.Get("/jobs", jobs.List)
		r.With(rlWrites).Post("/jobs/{job_id}/cancel", jobs.Cancel)
		r.With(rlWrites).Post("/jobs/{job_id}/charge", jobs.Charge)
		r.With(rlWrites).Post("/jobs/{job_id}/mark_paid", jobs.MarkPaid)
		r.With(rlWrites).Post("/jobs/{job_id}/deliver", jobs.Deliver)

		r.With(rlPayloads).Get("/payloads/{payload_id}", payloads.Get)
		r.With(rlPayloads).Get("/payloads", payloads.List)

		r.With(rlPoll).Get("/poll", poll.Poll)
		r.With(rlPoll).Post("/poll/ack", poll.Ack)
	})

	return r
}
