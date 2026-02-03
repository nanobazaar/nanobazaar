package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nanobazaar/relay/internal/store"
)

// StreamNotifier is used to emit best-effort wakeups when new events are appended.
// It must not leak plaintext/ciphertext; only stream identifiers are ever sent.
type StreamNotifier interface {
	NotifyEvent(ctx context.Context, recipientBotID, eventType string, data map[string]any)
}

// StreamHub is an in-memory registry for SSE subscriptions. It coalesces wakes and
// emits them best-effort; /poll remains authoritative.
type StreamHub struct {
	store *store.Store

	mu      sync.RWMutex
	streams map[string]map[*streamConn]struct{}

	cacheMu           sync.RWMutex
	botSigningPubkeys map[string]string // bot_id -> signing_pubkey_ed25519 (b64url)
}

func NewStreamHub(store *store.Store) *StreamHub {
	return &StreamHub{
		store:             store,
		streams:           make(map[string]map[*streamConn]struct{}),
		botSigningPubkeys: make(map[string]string),
	}
}

func (h *StreamHub) Register(conn *streamConn, streams []string) {
	if h == nil || conn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, stream := range streams {
		if stream == "" {
			continue
		}
		set := h.streams[stream]
		if set == nil {
			set = make(map[*streamConn]struct{})
			h.streams[stream] = set
		}
		set[conn] = struct{}{}
	}
}

func (h *StreamHub) Unregister(conn *streamConn) {
	if h == nil || conn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for stream := range conn.subscribed {
		set := h.streams[stream]
		if set == nil {
			continue
		}
		delete(set, conn)
		if len(set) == 0 {
			delete(h.streams, stream)
		}
	}
}

func (h *StreamHub) NotifyStream(stream string) {
	if h == nil || stream == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.streams[stream] {
		conn.markDirty(stream)
	}
}

func (h *StreamHub) NotifyEvent(ctx context.Context, recipientBotID, eventType string, data map[string]any) {
	if h == nil {
		return
	}

	// Best-effort: if we know the signing pubkey for this bot (because the bot
	// subscribed via SSE or used the default stream), also dirty the stable
	// seller inbox stream. Avoid DB reads here because this can be called inside
	// a write transaction.
	if recipientBotID != "" {
		h.cacheMu.RLock()
		pub := h.botSigningPubkeys[recipientBotID]
		h.cacheMu.RUnlock()
		if pub != "" {
			h.NotifyStream("seller:ed25519:" + pub)
		}
	}

	// If the event is job-related, dirty the per-job stream too.
	if data != nil {
		if jobID, ok := data["job_id"].(string); ok && jobID != "" {
			h.NotifyStream("job:" + jobID)
		}
	}
}

func (h *StreamHub) sellerStreamForBotID(ctx context.Context, botID string) (string, error) {
	if botID == "" {
		return "", errors.New("missing bot id")
	}
	h.cacheMu.RLock()
	cached := h.botSigningPubkeys[botID]
	h.cacheMu.RUnlock()
	if cached != "" {
		return "seller:ed25519:" + cached, nil
	}

	if h.store == nil {
		return "", errors.New("store unavailable")
	}
	bot, err := h.store.GetBot(ctx, botID)
	if err != nil {
		return "", err
	}
	if bot.SigningPubkeyEd25519 == "" {
		return "", errors.New("missing signing pubkey")
	}

	h.cacheMu.Lock()
	h.botSigningPubkeys[botID] = bot.SigningPubkeyEd25519
	h.cacheMu.Unlock()
	return "seller:ed25519:" + bot.SigningPubkeyEd25519, nil
}

type StreamHandler struct {
	Store             *store.Store
	Hub               *StreamHub
	Clock             func() time.Time
	FlushInterval     time.Duration
	KeepaliveInterval time.Duration
}

func NewStreamHandler(store *store.Store, hub *StreamHub) *StreamHandler {
	return &StreamHandler{
		Store:             store,
		Hub:               hub,
		Clock:             time.Now,
		FlushInterval:     500 * time.Millisecond,
		KeepaliveInterval: 20 * time.Second,
	}
}

func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Hub == nil {
		writeJSONError(w, http.StatusInternalServerError, "stream unavailable")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	streamKeys, err := h.resolveAndAuthorizeStreams(r.Context(), caller, r.URL.Query().Get("streams"))
	if err != nil {
		var httpErr *streamHTTPError
		if errors.As(err, &httpErr) {
			writeJSONError(w, httpErr.Status, httpErr.Message)
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "stream unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	conn := newStreamConn(streamKeys)
	h.Hub.Register(conn, streamKeys)
	defer h.Hub.Unregister(conn)

	// Initial frame so clients know the connection is established.
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	keepaliveEvery := h.KeepaliveInterval
	if keepaliveEvery <= 0 {
		keepaliveEvery = 20 * time.Second
	}
	flushEvery := h.FlushInterval
	if flushEvery <= 0 {
		flushEvery = 500 * time.Millisecond
	}

	keepalive := time.NewTicker(keepaliveEvery)
	flush := time.NewTicker(flushEvery)
	defer keepalive.Stop()
	defer flush.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			now := h.now().Unix()
			_, _ = w.Write([]byte(fmt.Sprintf(": keepalive %d\n\n", now)))
			flusher.Flush()
		case <-flush.C:
			dirty := conn.drainDirty()
			if len(dirty) == 0 {
				continue
			}
			sort.Strings(dirty)
			payload, _ := json.Marshal(map[string]any{
				"streams": dirty,
				"hint":    "poll",
			})
			_, _ = w.Write([]byte("event: wake\n"))
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(payload)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

func (h *StreamHandler) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

func (h *StreamHandler) resolveAndAuthorizeStreams(ctx context.Context, caller, raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	streams := parseCSV(raw)
	if len(streams) == 0 {
		// Default to the stable seller inbox stream for the caller.
		defaultStream, err := h.Hub.sellerStreamForBotID(ctx, caller)
		if err != nil {
			return nil, err
		}
		streams = []string{defaultStream}
	}

	normalized := make([]string, 0, len(streams))
	seen := make(map[string]struct{}, len(streams))
	for _, stream := range streams {
		stream = strings.TrimSpace(stream)
		if stream == "" {
			continue
		}
		if _, ok := seen[stream]; ok {
			continue
		}
		seen[stream] = struct{}{}

		switch {
		case strings.HasPrefix(stream, "seller:ed25519:"):
			pub := strings.TrimPrefix(stream, "seller:ed25519:")
			pub = strings.TrimSpace(pub)
			pubBytes, err := decodeKey(pub, 32)
			if err != nil {
				return nil, &streamHTTPError{Status: http.StatusBadRequest, Message: "invalid seller stream"}
			}
			derived := botIDFromSigningKey(pubBytes)
			if derived != caller {
				return nil, &streamHTTPError{Status: http.StatusForbidden, Message: "forbidden"}
			}
			bot, err := h.Store.GetBot(ctx, caller)
			if err != nil {
				return nil, err
			}
			if bot.SigningPubkeyEd25519 != pub {
				return nil, &streamHTTPError{Status: http.StatusBadRequest, Message: "seller stream mismatch"}
			}
			h.Hub.cacheMu.Lock()
			h.Hub.botSigningPubkeys[caller] = pub
			h.Hub.cacheMu.Unlock()
			normalized = append(normalized, stream)
		case strings.HasPrefix(stream, "job:"):
			jobID := strings.TrimPrefix(stream, "job:")
			jobID = strings.TrimSpace(jobID)
			if jobID == "" {
				return nil, &streamHTTPError{Status: http.StatusBadRequest, Message: "invalid job stream"}
			}
			job, err := h.Store.GetJob(ctx, jobID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, &streamHTTPError{Status: http.StatusNotFound, Message: "job not found"}
				}
				return nil, err
			}
			if job.BuyerBotID != caller && job.SellerBotID != caller {
				return nil, &streamHTTPError{Status: http.StatusForbidden, Message: "forbidden"}
			}
			normalized = append(normalized, "job:"+jobID)
		default:
			return nil, &streamHTTPError{Status: http.StatusBadRequest, Message: "unknown stream"}
		}
	}

	if len(normalized) == 0 {
		return nil, &streamHTTPError{Status: http.StatusBadRequest, Message: "missing streams"}
	}
	return normalized, nil
}

func parseCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

type streamHTTPError struct {
	Status  int
	Message string
}

func (e *streamHTTPError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type streamConn struct {
	subscribed map[string]struct{}

	mu    sync.Mutex
	dirty map[string]struct{}
}

func newStreamConn(streams []string) *streamConn {
	subscribed := make(map[string]struct{}, len(streams))
	for _, stream := range streams {
		if stream != "" {
			subscribed[stream] = struct{}{}
		}
	}
	return &streamConn{
		subscribed: subscribed,
		dirty:      make(map[string]struct{}),
	}
}

func (c *streamConn) markDirty(stream string) {
	if c == nil || stream == "" {
		return
	}
	c.mu.Lock()
	c.dirty[stream] = struct{}{}
	c.mu.Unlock()
}

func (c *streamConn) drainDirty() []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.dirty) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.dirty))
	for stream := range c.dirty {
		out = append(out, stream)
	}
	c.dirty = make(map[string]struct{})
	return out
}
