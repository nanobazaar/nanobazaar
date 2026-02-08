package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// StreamNotifier is used to emit best-effort wakeups when new events are appended.
// It must not leak plaintext/ciphertext; only stream identifiers are ever sent.
type StreamNotifier interface {
	NotifyEvent(ctx context.Context, recipientBotID, eventType string, data map[string]any)
}

// StreamHub is an in-memory registry for SSE subscriptions. It coalesces wakes and
// emits them best-effort; /poll remains authoritative.
//
// Post-simplification: subscriptions are per-bot_id (no stream keys). Wakeups only
// tell clients to poll.
type StreamHub struct {
	mu      sync.RWMutex
	conns   map[*streamConn]struct{}
	botConn map[string]map[*streamConn]struct{}
}

type StreamHubStats struct {
	ActiveConns   int `json:"active_conns"`
	ActiveBots    int `json:"active_bots"`
	ActiveStreams int `json:"active_streams"`
}

func NewStreamHub() *StreamHub {
	return &StreamHub{
		conns:   make(map[*streamConn]struct{}),
		botConn: make(map[string]map[*streamConn]struct{}),
	}
}

func (h *StreamHub) Stats() StreamHubStats {
	if h == nil {
		return StreamHubStats{}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return StreamHubStats{
		ActiveConns:   len(h.conns),
		ActiveBots:    len(h.botConn),
		ActiveStreams: 0,
	}
}

func (h *StreamHub) Register(conn *streamConn) (int, int) {
	if h == nil || conn == nil {
		return 0, 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[conn] = struct{}{}
	if conn.botID != "" {
		set := h.botConn[conn.botID]
		if set == nil {
			set = make(map[*streamConn]struct{})
			h.botConn[conn.botID] = set
		}
		set[conn] = struct{}{}
	}
	return len(h.conns), len(h.botConn)
}

func (h *StreamHub) Unregister(conn *streamConn) (int, int) {
	if h == nil || conn == nil {
		return 0, 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, conn)
	if conn.botID != "" {
		set := h.botConn[conn.botID]
		if set != nil {
			delete(set, conn)
			if len(set) == 0 {
				delete(h.botConn, conn.botID)
			}
		}
	}
	return len(h.conns), len(h.botConn)
}

func (h *StreamHub) NotifyEvent(ctx context.Context, recipientBotID, eventType string, data map[string]any) {
	if h == nil {
		return
	}
	if recipientBotID != "" {
		h.mu.RLock()
		defer h.mu.RUnlock()
		for conn := range h.botConn[recipientBotID] {
			conn.markDirty()
		}
	}
}

type StreamHandler struct {
	Hub               *StreamHub
	Clock             func() time.Time
	FlushInterval     time.Duration
	KeepaliveInterval time.Duration
}

func NewStreamHandler(hub *StreamHub) *StreamHandler {
	return &StreamHandler{
		Hub:               hub,
		Clock:             time.Now,
		FlushInterval:     500 * time.Millisecond,
		KeepaliveInterval: 20 * time.Second,
	}
}

func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Hub == nil {
		writeJSONError(w, http.StatusInternalServerError, "stream unavailable")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	if strings.TrimSpace(r.URL.Query().Get("streams")) != "" {
		writeJSONError(w, http.StatusBadRequest, "streams query parameter is no longer supported")
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

	conn := newStreamConn(caller)
	activeConns, activeBots := h.Hub.Register(conn)
	remoteAddr := r.RemoteAddr
	log.Printf("stream_connected bot_id=%s active_conns=%d active_bots=%d remote_addr=%s", caller, activeConns, activeBots, remoteAddr)
	defer func() {
		activeConns, activeBots := h.Hub.Unregister(conn)
		log.Printf("stream_disconnected bot_id=%s active_conns=%d active_bots=%d remote_addr=%s", caller, activeConns, activeBots, remoteAddr)
	}()

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
			if !conn.drainDirty() {
				continue
			}
			payload := []byte(`{"hint":"poll"}`)
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

type streamConn struct {
	botID string

	mu    sync.Mutex
	dirty bool
}

func newStreamConn(botID string) *streamConn {
	return &streamConn{
		botID: botID,
	}
}

func (c *streamConn) markDirty() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.dirty = true
	c.mu.Unlock()
}

func (c *streamConn) drainDirty() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	dirty := c.dirty
	c.dirty = false
	return dirty
}
