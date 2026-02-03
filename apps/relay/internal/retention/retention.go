package retention

import (
	"context"
	"database/sql"
	"log"
	"time"
)

const (
	nonceTTL        = 10 * time.Minute
	idempotencyTTL  = 30 * 24 * time.Hour
	offerTTL        = 30 * 24 * time.Hour
	jobTTL          = 30 * 24 * time.Hour
	payloadFetchTTL = 7 * 24 * time.Hour
	payloadTTL      = 30 * 24 * time.Hour
	eventTTL        = 30 * 24 * time.Hour
)

type Cleaner interface {
	DeleteNoncesBefore(ctx context.Context, cutoff time.Time) error
	DeleteIdempotencyBefore(ctx context.Context, cutoff time.Time) error
	DeleteOffersBefore(ctx context.Context, cutoff time.Time) error
	DeleteJobsTerminalBefore(ctx context.Context, cutoff sql.NullTime) error
	DeletePayloadsFetchedBefore(ctx context.Context, cutoff sql.NullTime) error
	DeletePayloadsBefore(ctx context.Context, cutoff time.Time) error
	DeleteEventsBefore(ctx context.Context, cutoff time.Time) error
	DeleteStreamEventsAckedBefore(ctx context.Context, cutoff time.Time) error
}

func Start(enabled bool, interval time.Duration, logger *log.Logger, cleaner Cleaner) func() {
	if !enabled || cleaner == nil {
		return func() {}
	}

	stopCh := make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runCleanup(cleaner, logger)
			case <-stopCh:
				return
			}
		}
	}()

	return func() { close(stopCh) }
}

func runCleanup(cleaner Cleaner, logger *log.Logger) {
	now := time.Now().UTC()
	ctx := context.Background()

	logErr := func(action string, err error) {
		if err == nil || logger == nil {
			return
		}
		logger.Printf("retention %s error: %v", action, err)
	}

	logErr("nonces", cleaner.DeleteNoncesBefore(ctx, now.Add(-nonceTTL)))
	logErr("idempotency", cleaner.DeleteIdempotencyBefore(ctx, now.Add(-idempotencyTTL)))
	logErr("payloads_fetched", cleaner.DeletePayloadsFetchedBefore(ctx, sql.NullTime{Time: now.Add(-payloadFetchTTL), Valid: true}))
	logErr("payloads", cleaner.DeletePayloadsBefore(ctx, now.Add(-payloadTTL)))
	logErr("events", cleaner.DeleteEventsBefore(ctx, now.Add(-eventTTL)))
	logErr("stream_events", cleaner.DeleteStreamEventsAckedBefore(ctx, now.Add(-eventTTL)))
	logErr("jobs", cleaner.DeleteJobsTerminalBefore(ctx, sql.NullTime{Time: now.Add(-jobTTL), Valid: true}))
	logErr("offers", cleaner.DeleteOffersBefore(ctx, now.Add(-offerTTL)))
}
