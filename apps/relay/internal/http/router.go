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
	StreamHub    *StreamHub
	AdminToken   string
	AdminPublic  bool
}

func NewRouter(cfg RouterConfig, opts ...Option) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(errorLogMiddleware())
	r.Use(metricsMiddleware(cfg.Metrics))

	streamHub := cfg.StreamHub
	if streamHub == nil {
		streamHub = NewStreamHub()
	}

	bots := NewBotHandler(cfg.Store)
	offers := NewOfferHandler(cfg.Store)
	jobs := NewJobHandler(cfg.Store, cfg.Metrics)
	payloads := NewPayloadHandler(cfg.Store, cfg.Metrics)
	poll := NewPollHandler(cfg.Store, cfg.Metrics)
	stream := NewStreamHandler(streamHub)
	stats := NewStatsHandler(cfg.Store)
	if cfg.Verifier != nil && cfg.Verifier.Clock != nil {
		bots.Clock = cfg.Verifier.Clock
		offers.Clock = cfg.Verifier.Clock
		poll.Clock = cfg.Verifier.Clock
		stream.Clock = cfg.Verifier.Clock
	}
	bots.StreamHub = streamHub
	jobs.StreamHub = streamHub
	for _, opt := range opts {
		opt(jobs)
	}

	r.With(healthMiddleware(cfg.HealthPublic)).Get("/healthz", healthz)
	r.With(healthMiddleware(cfg.HealthPublic)).Get("/readyz", readyz)
	r.Get("/stats", stats.Get)
	r.With(rateLimitMiddleware(cfg.Limiter, ratelimit.BucketOfferSearch, cfg.Metrics)).Get("/market/offers", offers.PublicList)

	if cfg.AdminPublic {
		RegisterAdminRoutes(r, AdminRouterConfig{
			Store:      cfg.Store,
			Metrics:    cfg.Metrics,
			AdminToken: cfg.AdminToken,
			StreamHub:  streamHub,
			Mode:       "public_mount",
		})
	}

	r.Route("/v0", func(r chi.Router) {
		r.Use(auth.Middleware(cfg.Verifier))

		rlPoll := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketPollAck, cfg.Metrics)
		rlOffers := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketOfferSearch, cfg.Metrics)
		rlWrites := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketWrites, cfg.Metrics)
		rlPayloads := rateLimitMiddleware(cfg.Limiter, ratelimit.BucketPayloadFetch, cfg.Metrics)

		r.With(rlWrites).Post("/bots", bots.Register)
		r.Get("/bots/{bot_id}", bots.Get)
		r.With(rlWrites).Post("/bots/{bot_id}/revoke", bots.Revoke)
		r.With(rlWrites).Post("/bots/{bot_id}/name", bots.SetName)

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
		r.With(rlWrites).Post("/jobs/{job_id}/payment_sent", jobs.PaymentSent)
		r.With(rlWrites).Post("/jobs/{job_id}/charge/reissue", jobs.ReissueCharge)
		r.With(rlWrites).Post("/jobs/{job_id}/charge/reissue_request", jobs.ReissueChargeRequest)
		r.With(rlWrites).Post("/jobs/{job_id}/mark_paid", jobs.MarkPaid)
		r.With(rlWrites).Post("/jobs/{job_id}/deliver", jobs.Deliver)

		r.With(rlPayloads).Get("/payloads/{payload_id}", payloads.Get)
		r.With(rlPayloads).Get("/payloads", payloads.List)

		r.With(rlPoll).Get("/poll", poll.Poll)
		r.With(rlPoll).Post("/poll/ack", poll.Ack)
		r.With(rlPoll).Get("/stream", stream.Stream)
	})

	return r
}
