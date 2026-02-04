package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nanobazaar/relay/internal/store/sqlc"
)

const (
	maxBatchStreams     = 64
	maxBatchTotalEvents = 1000
)

type batchPollStream struct {
	Stream string `json:"stream"`
	Since  int64  `json:"since"`
}

type batchPollRequest struct {
	Streams []batchPollStream `json:"streams"`
	Limit   int               `json:"limit"`
}

type batchPollResult struct {
	Stream string      `json:"stream"`
	Events []pollEvent `json:"events"`
	Next   int64       `json:"next"`
}

type batchPollResponse struct {
	Results []batchPollResult `json:"results"`
}

type streamAckRequest struct {
	Stream string `json:"stream"`
	Ack    int64  `json:"ack"`
}

type streamAckResponse struct {
	Stream string `json:"stream"`
	Ack    int64  `json:"ack"`
}

func (h *PollHandler) Batch(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	var payload batchPollRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(payload.Streams) == 0 {
		writeJSONError(w, http.StatusBadRequest, "missing streams")
		return
	}
	if len(payload.Streams) > maxBatchStreams {
		writeJSONError(w, http.StatusBadRequest, "too many streams")
		return
	}

	limit, err := parseBatchLimit(payload.Limit)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	seen := make(map[string]struct{}, len(payload.Streams))
	streams := make([]batchPollStream, 0, len(payload.Streams))
	for _, entry := range payload.Streams {
		if entry.Since < 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid since")
			return
		}
		normalized, err := authorizeStreamKey(r.Context(), h.Store, caller, entry.Stream)
		if err != nil {
			var httpErr *streamHTTPError
			if errors.As(err, &httpErr) {
				writeJSONError(w, httpErr.Status, httpErr.Message)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		streams = append(streams, batchPollStream{Stream: normalized, Since: entry.Since})
	}
	if len(streams) == 0 {
		writeJSONError(w, http.StatusBadRequest, "missing streams")
		return
	}

	remaining := maxBatchTotalEvents
	results := make([]batchPollResult, 0, len(streams))
	for _, entry := range streams {
		allow := limit
		if remaining < allow {
			allow = remaining
		}
		if allow <= 0 {
			results = append(results, batchPollResult{
				Stream: entry.Stream,
				Events: nil,
				Next:   entry.Since,
			})
			continue
		}
		events, err := h.fetchStreamEvents(r.Context(), entry.Stream, entry.Since, allow)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "poll batch failed")
			return
		}
		remaining -= len(events)
		next := entry.Since
		if len(events) > 0 {
			next = events[len(events)-1].EventID
		}
		results = append(results, batchPollResult{
			Stream: entry.Stream,
			Events: events,
			Next:   next,
		})
	}

	writeJSON(w, http.StatusOK, batchPollResponse{Results: results})
}

func (h *PollHandler) AckStream(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	var payload streamAckRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.Ack < 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid ack")
		return
	}
	streamKey, err := authorizeStreamKey(r.Context(), h.Store, caller, payload.Stream)
	if err != nil {
		var httpErr *streamHTTPError
		if errors.As(err, &httpErr) {
			writeJSONError(w, httpErr.Status, httpErr.Message)
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	current := int64(0)
	ack, err := h.Store.GetStreamAck(r.Context(), streamKey)
	if err == nil {
		current = ack.AckCursor
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeJSONError(w, http.StatusInternalServerError, "stream ack lookup failed")
		return
	}

	if payload.Ack > current {
		current = payload.Ack
	}

	if err := h.Store.UpsertStreamAck(r.Context(), sqlc.UpsertStreamAckParams{
		StreamKey: streamKey,
		AckCursor: current,
		UpdatedAt: h.now(),
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "stream ack update failed")
		return
	}

	writeJSON(w, http.StatusOK, streamAckResponse{Stream: streamKey, Ack: current})
}

func (h *PollHandler) fetchStreamEvents(ctx context.Context, streamKey string, since int64, limit int) ([]pollEvent, error) {
	batch, err := h.Store.ListStreamEventsAfterCursor(ctx, sqlc.ListStreamEventsAfterCursorParams{
		StreamKey:   streamKey,
		SinceCursor: since,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]pollEvent, 0, len(batch))
	for _, event := range batch {
		if !json.Valid([]byte(event.PayloadJson)) {
			return nil, errors.New("invalid event payload")
		}
		results = append(results, pollEvent{
			EventID:   event.Cursor,
			EventType: event.EventType,
			Data:      json.RawMessage(event.PayloadJson),
		})
	}
	return results, nil
}

func parseBatchLimit(value int) (int, error) {
	if value == 0 {
		return 50, nil
	}
	if value < 0 {
		return 0, errors.New("invalid limit")
	}
	if value > 200 {
		return 200, nil
	}
	return value, nil
}
