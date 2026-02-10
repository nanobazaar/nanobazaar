package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

type PollHandler struct {
	Store   *store.Store
	Metrics *metrics.Registry
	Clock   func() time.Time
}

func NewPollHandler(store *store.Store, metrics *metrics.Registry) *PollHandler {
	return &PollHandler{Store: store, Metrics: metrics, Clock: time.Now}
}

type pollEvent struct {
	EventID   int64           `json:"event_id"`
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
}

type pollResponse struct {
	Events             []pollEvent `json:"events"`
	LastAckedEventID   int64       `json:"last_acked_event_id"`
	MinEventIDRetained int64       `json:"min_event_id_retained"`
}

type pollGoneResponse struct {
	Code               string `json:"code"`
	Message            string `json:"message"`
	MinEventIDRetained int64  `json:"min_event_id_retained"`
	SuggestedResync    bool   `json:"suggested_resync"`
}

type pollAckRequest struct {
	UpToEventID int64 `json:"up_to_event_id"`
}

type pollAckResponse struct {
	LastAckedEventID int64 `json:"last_acked_event_id"`
}

func (h *PollHandler) Poll(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	sinceRaw := r.URL.Query().Get("since_event_id")
	sinceID, err := parseSinceEventID(sinceRaw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid since_event_id")
		return
	}

	lastAcked := int64(0)
	ack, err := h.Store.GetPollAck(r.Context(), caller)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			writeJSONInternalError(w, r, "poll ack lookup failed", err)
			return
		}
	} else {
		lastAcked = ack.LastAckedEventID
	}

	cursor := sinceID
	if strings.TrimSpace(sinceRaw) == "" {
		cursor = lastAcked
	}

	minEventID, err := h.getMinEventID(r.Context(), caller)
	if err != nil {
		writeJSONInternalError(w, r, "poll min lookup failed", err)
		return
	}
	if minEventID > 0 && cursor < minEventID-1 {
		if h.Metrics != nil {
			h.Metrics.IncPollGone()
		}
		logPollGone(caller, cursor, minEventID)
		writeJSON(w, http.StatusGone, pollGoneResponse{
			Code:               "gone",
			Message:            "cursor too old",
			MinEventIDRetained: minEventID,
			SuggestedResync:    true,
		})
		return
	}

	types := parseEventTypes(r.URL.Query().Get("types"))
	events, newestEventTime, err := h.fetchEvents(r.Context(), caller, cursor, limit, types)
	if err != nil {
		writeJSONInternalError(w, r, "poll failed", err)
		return
	}
	if h.Metrics != nil && newestEventTime != nil {
		h.Metrics.ObservePollLag(time.Since(*newestEventTime))
	}

	writeJSON(w, http.StatusOK, pollResponse{
		Events:             events,
		LastAckedEventID:   lastAcked,
		MinEventIDRetained: minEventID,
	})
}

func (h *PollHandler) Ack(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	var payload pollAckRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.UpToEventID < 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid up_to_event_id")
		return
	}

	current := int64(0)
	ack, err := h.Store.GetPollAck(r.Context(), caller)
	if err == nil {
		current = ack.LastAckedEventID
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeJSONInternalError(w, r, "poll ack lookup failed", err)
		return
	}

	if payload.UpToEventID > current {
		current = payload.UpToEventID
	}

	now := h.now()
	if err := h.Store.UpsertPollAck(r.Context(), sqlc.UpsertPollAckParams{
		RecipientBotID:   caller,
		LastAckedEventID: current,
		UpdatedAt:        now,
	}); err != nil {
		writeJSONInternalError(w, r, "poll ack update failed", err)
		return
	}

	if h.Metrics != nil && current > 0 {
		if createdAt, err := h.Store.GetEventCreatedAt(r.Context(), sqlc.GetEventCreatedAtParams{
			RecipientBotID: caller,
			EventID:        current,
		}); err == nil {
			h.Metrics.ObserveAckLag(now.Sub(createdAt))
		}
	}

	writeJSON(w, http.StatusOK, pollAckResponse{LastAckedEventID: current})
}

func (h *PollHandler) now() time.Time {
	if h.Clock == nil {
		return time.Now().UTC()
	}
	return h.Clock().UTC()
}

func (h *PollHandler) getMinEventID(ctx context.Context, recipient string) (int64, error) {
	min, err := h.Store.GetMinEventID(ctx, recipient)
	if err != nil {
		return 0, err
	}
	switch value := min.(type) {
	case nil:
		return 0, nil
	case int64:
		return value, nil
	case int32:
		return int64(value), nil
	case int:
		return int64(value), nil
	case []byte:
		parsed, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, nil
	}
}

func (h *PollHandler) fetchEvents(ctx context.Context, recipient string, sinceID int64, limit int, types map[string]struct{}) ([]pollEvent, *time.Time, error) {
	var (
		batch []sqlc.Event
		err   error
	)
	if len(types) > 0 {
		batch, err = h.Store.ListEventsAfterIDByTypes(ctx, sqlc.ListEventsAfterIDByTypesParams{
			RecipientBotID: recipient,
			SinceEventID:   sinceID,
			EventTypes:     mapKeys(types),
			Limit:          int64(limit),
		})
	} else {
		batch, err = h.Store.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
			RecipientBotID: recipient,
			SinceEventID:   sinceID,
			Limit:          int64(limit),
		})
	}
	if err != nil {
		return nil, nil, err
	}

	results := make([]pollEvent, 0, len(batch))
	var newest *time.Time
	for _, event := range batch {
		if !json.Valid([]byte(event.DataJson)) {
			return nil, nil, errors.New("invalid event payload")
		}
		results = append(results, pollEvent{
			EventID:   event.EventID,
			EventType: event.EventType,
			Data:      json.RawMessage(event.DataJson),
		})
		if newest == nil || event.CreatedAt.After(*newest) {
			t := event.CreatedAt
			newest = &t
		}
	}

	return results, newest, nil
}

func parseSinceEventID(value string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, errors.New("invalid since_event_id")
	}
	return parsed, nil
}

func parseEventTypes(raw string) map[string]struct{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make(map[string]struct{})
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func mapKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func logPollGone(botID string, sinceID, minEventID int64) {
	log.Printf("poll_gone bot_id=%s since_event_id=%d min_event_id_retained=%d", botID, sinceID, minEventID)
}
