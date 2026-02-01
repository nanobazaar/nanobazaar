package ratelimit

import (
	"math"
	"sync"
	"time"
)

const (
	BucketPollAck      = "poll_ack"
	BucketOfferSearch  = "offer_search"
	BucketWrites       = "writes"
	BucketPayloadFetch = "payload_fetch"
)

type BucketConfig struct {
	Rate  float64
	Burst int
}

type Config struct {
	PollAck      BucketConfig
	OfferSearch  BucketConfig
	Writes       BucketConfig
	PayloadFetch BucketConfig
}

type Option func(*Limiter)

type Limiter struct {
	buckets map[string]*bucket
	clock   func() time.Time
}

type bucket struct {
	rate    float64
	burst   float64
	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	tokens float64
	last   time.Time
}

type Result struct {
	Allowed           bool
	Limit             int
	Remaining         int
	ResetSeconds      int
	RetryAfterSeconds int
}

func NewLimiter(cfg Config, opts ...Option) *Limiter {
	l := &Limiter{
		buckets: map[string]*bucket{
			BucketPollAck: {
				rate:    cfg.PollAck.Rate,
				burst:   float64(cfg.PollAck.Burst),
				entries: make(map[string]*entry),
			},
			BucketOfferSearch: {
				rate:    cfg.OfferSearch.Rate,
				burst:   float64(cfg.OfferSearch.Burst),
				entries: make(map[string]*entry),
			},
			BucketWrites: {
				rate:    cfg.Writes.Rate,
				burst:   float64(cfg.Writes.Burst),
				entries: make(map[string]*entry),
			},
			BucketPayloadFetch: {
				rate:    cfg.PayloadFetch.Rate,
				burst:   float64(cfg.PayloadFetch.Burst),
				entries: make(map[string]*entry),
			},
		},
		clock: time.Now,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func WithClock(clock func() time.Time) Option {
	return func(l *Limiter) {
		if clock != nil {
			l.clock = clock
		}
	}
}

func (l *Limiter) Enabled(bucket string) bool {
	b := l.bucket(bucket)
	if b == nil {
		return false
	}
	return b.rate > 0 && b.burst > 0
}

func (l *Limiter) Allow(bucketName, key string) Result {
	b := l.bucket(bucketName)
	if b == nil || b.rate <= 0 || b.burst <= 0 {
		return Result{Allowed: true}
	}
	return b.allow(key, l.now())
}

func (l *Limiter) now() time.Time {
	if l.clock == nil {
		return time.Now()
	}
	return l.clock()
}

func (l *Limiter) bucket(name string) *bucket {
	if l == nil {
		return nil
	}
	return l.buckets[name]
}

func (b *bucket) allow(key string, now time.Time) Result {
	b.mu.Lock()
	defer b.mu.Unlock()

	state, ok := b.entries[key]
	if !ok {
		state = &entry{tokens: b.burst, last: now}
		b.entries[key] = state
	}

	elapsed := now.Sub(state.last).Seconds()
	if elapsed > 0 {
		state.tokens = math.Min(b.burst, state.tokens+elapsed*b.rate)
	}

	allowed := state.tokens >= 1
	if allowed {
		state.tokens -= 1
	}
	state.last = now

	remaining := int(math.Floor(state.tokens))
	if remaining < 0 {
		remaining = 0
	}

	resetSeconds := 0
	if b.rate > 0 {
		resetSeconds = int(math.Ceil((b.burst - state.tokens) / b.rate))
		if resetSeconds < 0 {
			resetSeconds = 0
		}
	}

	retryAfter := 0
	if !allowed && b.rate > 0 {
		retryAfter = int(math.Ceil((1 - state.tokens) / b.rate))
		if retryAfter < 1 {
			retryAfter = 1
		}
	}

	return Result{
		Allowed:           allowed,
		Limit:             int(b.burst),
		Remaining:         remaining,
		ResetSeconds:      resetSeconds,
		RetryAfterSeconds: retryAfter,
	}
}
