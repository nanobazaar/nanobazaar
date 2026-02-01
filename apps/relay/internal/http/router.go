package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/store"
)

type Option func(*JobHandler)

func WithClock(clock func() time.Time) Option {
	return func(handler *JobHandler) {
		handler.Clock = clock
	}
}

func NewRouter(verifier *auth.Verifier, store *store.Store, opts ...Option) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	bots := NewBotHandler(store)
	offers := NewOfferHandler(store)
	jobs := NewJobHandler(store)
	payloads := NewPayloadHandler(store)
	poll := NewPollHandler(store)
	for _, opt := range opts {
		opt(jobs)
	}

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz)

	r.Route("/v0", func(r chi.Router) {
		r.Use(auth.Middleware(verifier))

		r.Post("/bots", bots.Register)
		r.Get("/bots/{bot_id}", bots.Get)

		r.Post("/offers", offers.Create)
		r.Get("/offers/{offer_id}", offers.Get)
		r.Post("/offers/{offer_id}/cancel", offers.Cancel)
		r.Get("/offers", offers.List)

		r.Post("/jobs", jobs.Create)
		r.Get("/jobs/{job_id}", jobs.Get)
		r.Get("/jobs", jobs.List)
		r.Post("/jobs/{job_id}/cancel", jobs.Cancel)
		r.Post("/jobs/{job_id}/charge", jobs.Charge)
		r.Post("/jobs/{job_id}/mark_paid", jobs.MarkPaid)
		r.Post("/jobs/{job_id}/deliver", jobs.Deliver)

		r.Get("/payloads/{payload_id}", payloads.Get)
		r.Get("/payloads", payloads.List)

		r.Get("/poll", poll.Poll)
		r.Post("/poll/ack", poll.Ack)
	})

	return r
}
