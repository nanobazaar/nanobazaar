package metrics

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Counter struct {
	value uint64
}

func (c *Counter) Inc() {
	atomic.AddUint64(&c.value, 1)
}

func (c *Counter) Add(n uint64) {
	atomic.AddUint64(&c.value, n)
}

func (c *Counter) Load() uint64 {
	return atomic.LoadUint64(&c.value)
}

type Gauge struct {
	value int64
}

func (g *Gauge) Set(v int64) {
	atomic.StoreInt64(&g.value, v)
}

func (g *Gauge) Add(delta int64) {
	atomic.AddInt64(&g.value, delta)
}

func (g *Gauge) Load() int64 {
	return atomic.LoadInt64(&g.value)
}

type CounterVec struct {
	mu       sync.Mutex
	counters map[string]*Counter
}

func NewCounterVec() *CounterVec {
	return &CounterVec{counters: make(map[string]*Counter)}
}

func (v *CounterVec) Inc(label string) {
	v.Add(label, 1)
}

func (v *CounterVec) Add(label string, n uint64) {
	v.mu.Lock()
	counter, ok := v.counters[label]
	if !ok {
		counter = &Counter{}
		v.counters[label] = counter
	}
	v.mu.Unlock()
	counter.Add(n)
}

func (v *CounterVec) Snapshot() map[string]uint64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make(map[string]uint64, len(v.counters))
	for label, counter := range v.counters {
		out[label] = counter.Load()
	}
	return out
}

type HistogramSnapshot struct {
	Buckets []float64 `json:"buckets"`
	Counts  []uint64  `json:"counts"`
	Sum     float64   `json:"sum"`
	Count   uint64    `json:"count"`
}

type Histogram struct {
	mu      sync.Mutex
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

func NewHistogram(buckets []float64) *Histogram {
	return &Histogram{
		buckets: append([]float64(nil), buckets...),
		counts:  make([]uint64, len(buckets)+1),
	}
}

func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sum += v
	h.count++
	idx := len(h.buckets)
	for i, bound := range h.buckets {
		if v <= bound {
			idx = i
			break
		}
	}
	h.counts[idx]++
}

func (h *Histogram) Snapshot() HistogramSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return HistogramSnapshot{
		Buckets: append([]float64(nil), h.buckets...),
		Counts:  append([]uint64(nil), h.counts...),
		Sum:     h.sum,
		Count:   h.count,
	}
}

type HistogramVec struct {
	mu      sync.Mutex
	buckets []float64
	items   map[string]*Histogram
}

func NewHistogramVec(buckets []float64) *HistogramVec {
	return &HistogramVec{
		buckets: append([]float64(nil), buckets...),
		items:   make(map[string]*Histogram),
	}
}

func (v *HistogramVec) Observe(label string, value float64) {
	v.mu.Lock()
	hist, ok := v.items[label]
	if !ok {
		hist = NewHistogram(v.buckets)
		v.items[label] = hist
	}
	v.mu.Unlock()
	hist.Observe(value)
}

func (v *HistogramVec) Snapshot() map[string]HistogramSnapshot {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make(map[string]HistogramSnapshot, len(v.items))
	for label, hist := range v.items {
		out[label] = hist.Snapshot()
	}
	return out
}

type Registry struct {
	requestsTotal       *CounterVec
	requestDurationMs   *HistogramVec
	authFailuresTotal   *CounterVec
	rateLimitRejections *CounterVec
	statusTotals        *CounterVec
	pollGone            *Counter
	pollLagMs           *Histogram
	ackLagMs            *Histogram
	pendingPayloads     *Gauge
	payloadBytes        *Gauge
}

func NewRegistry() *Registry {
	durationBuckets := []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2000, 5000}
	lagBuckets := []float64{100, 250, 500, 1000, 2000, 5000, 10000, 30000, 60000}
	return &Registry{
		requestsTotal:       NewCounterVec(),
		requestDurationMs:   NewHistogramVec(durationBuckets),
		authFailuresTotal:   NewCounterVec(),
		rateLimitRejections: NewCounterVec(),
		statusTotals:        NewCounterVec(),
		pollGone:            &Counter{},
		pollLagMs:           NewHistogram(lagBuckets),
		ackLagMs:            NewHistogram(lagBuckets),
		pendingPayloads:     &Gauge{},
		payloadBytes:        &Gauge{},
	}
}

func (r *Registry) RecordRequest(method, route string, status int, duration time.Duration) {
	if r == nil {
		return
	}
	label := method + " " + route + " " + strconv.Itoa(status)
	r.requestsTotal.Inc(label)
	endpoint := method + " " + route
	r.requestDurationMs.Observe(endpoint, float64(duration.Milliseconds()))
	switch status {
	case 409, 410, 429:
		r.statusTotals.Inc(strconv.Itoa(status))
	}
}

func (r *Registry) RecordAuthFailure(reason string) {
	if r == nil {
		return
	}
	r.authFailuresTotal.Inc(reason)
}

func (r *Registry) RecordRateLimit(bucket string) {
	if r == nil {
		return
	}
	r.rateLimitRejections.Inc(bucket)
}

func (r *Registry) IncPollGone() {
	if r == nil {
		return
	}
	r.pollGone.Inc()
}

func (r *Registry) ObservePollLag(d time.Duration) {
	if r == nil {
		return
	}
	r.pollLagMs.Observe(float64(d.Milliseconds()))
}

func (r *Registry) ObserveAckLag(d time.Duration) {
	if r == nil {
		return
	}
	r.ackLagMs.Observe(float64(d.Milliseconds()))
}

func (r *Registry) AddPayloadBytes(delta int64) {
	if r == nil {
		return
	}
	r.payloadBytes.Add(delta)
}

func (r *Registry) AddPendingPayloads(delta int64) {
	if r == nil {
		return
	}
	r.pendingPayloads.Add(delta)
}

func (r *Registry) SetPayloadStats(pending, bytes int64) {
	if r == nil {
		return
	}
	r.pendingPayloads.Set(pending)
	r.payloadBytes.Set(bytes)
}

func (r *Registry) Snapshot() map[string]any {
	if r == nil {
		return map[string]any{}
	}
	return map[string]any{
		"requests_total":              r.requestsTotal.Snapshot(),
		"request_duration_ms":         r.requestDurationMs.Snapshot(),
		"auth_failures_total":         r.authFailuresTotal.Snapshot(),
		"rate_limit_rejections_total": r.rateLimitRejections.Snapshot(),
		"status_totals":               r.statusTotals.Snapshot(),
		"poll_gone_total":             r.pollGone.Load(),
		"poll_lag_ms":                 r.pollLagMs.Snapshot(),
		"ack_lag_ms":                  r.ackLagMs.Snapshot(),
		"payload_pending":             r.pendingPayloads.Load(),
		"payload_stored_bytes":        r.payloadBytes.Load(),
	}
}
