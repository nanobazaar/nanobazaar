package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

type PayloadHandler struct {
	Store *store.Store
	Clock func() time.Time
}

func NewPayloadHandler(store *store.Store) *PayloadHandler {
	return &PayloadHandler{Store: store, Clock: time.Now}
}

type payloadEnvelopeResponse struct {
	PayloadID      string    `json:"payload_id"`
	JobID          string    `json:"job_id"`
	SenderBotID    string    `json:"sender_bot_id"`
	RecipientBotID string    `json:"recipient_bot_id"`
	PayloadKind    string    `json:"payload_kind"`
	EncAlg         string    `json:"enc_alg"`
	RecipientKid   string    `json:"recipient_kid"`
	CiphertextB64  string    `json:"ciphertext_b64"`
	CreatedAt      time.Time `json:"created_at"`
}

type payloadMetadataResponse struct {
	PayloadID   string    `json:"payload_id"`
	JobID       string    `json:"job_id"`
	PayloadKind string    `json:"payload_kind"`
	CreatedAt   time.Time `json:"created_at"`
}

type payloadListResponse struct {
	Payloads   []payloadMetadataResponse `json:"payloads"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}

type payloadMetadataRow struct {
	PayloadID   string
	JobID       string
	PayloadKind string
	CreatedAt   time.Time
}

type payloadCursor struct {
	Status    string    `json:"status"`
	JobID     string    `json:"job_id"`
	CreatedAt time.Time `json:"created_at"`
	PayloadID string    `json:"payload_id"`
}

func (h *PayloadHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	payloadID := chi.URLParam(r, "payload_id")
	if payloadID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing payload_id")
		return
	}

	recipient, err := h.Store.GetPayloadRecipient(r.Context(), payloadID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "payload not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "payload lookup failed")
		return
	}
	if recipient != caller {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	payload, err := h.Store.GetPayload(r.Context(), sqlc.GetPayloadParams{PayloadID: payloadID, RecipientBotID: caller})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "payload not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "payload lookup failed")
		return
	}

	now := h.now()
	if err := h.Store.MarkPayloadFetched(r.Context(), sqlc.MarkPayloadFetchedParams{
		PayloadID:      payloadID,
		RecipientBotID: caller,
		FetchedAt:      sql.NullTime{Time: now, Valid: true},
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "payload update failed")
		return
	}

	writeJSON(w, http.StatusOK, payloadEnvelopeResponse{
		PayloadID:      payload.PayloadID,
		JobID:          payload.JobID,
		SenderBotID:    payload.SenderBotID,
		RecipientBotID: payload.RecipientBotID,
		PayloadKind:    payload.PayloadKind,
		EncAlg:         payload.EncAlg,
		RecipientKid:   payload.RecipientKid,
		CiphertextB64:  payload.CiphertextB64,
		CreatedAt:      payload.CreatedAt,
	})
}

func (h *PayloadHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "unfetched"
	}
	if status != "unfetched" && status != "fetched" && status != "all" {
		writeJSONError(w, http.StatusBadRequest, "invalid status")
		return
	}

	jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
	var cursor *payloadCursor
	if cursorValue != "" {
		decoded, err := decodePayloadCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		if decoded.Status != status || decoded.JobID != jobID {
			writeJSONError(w, http.StatusBadRequest, "cursor mismatch")
			return
		}
		cursor = &decoded
	}

	rows, nextCursor, err := h.fetchPayloadMetadata(r.Context(), caller, status, jobID, cursor, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "payload list failed")
		return
	}

	resp := make([]payloadMetadataResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, payloadMetadataResponse{
			PayloadID:   row.PayloadID,
			JobID:       row.JobID,
			PayloadKind: row.PayloadKind,
			CreatedAt:   row.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, payloadListResponse{Payloads: resp, NextCursor: nextCursor})
}

func (h *PayloadHandler) fetchPayloadMetadata(ctx context.Context, recipient, status, jobID string, cursor *payloadCursor, limit int) ([]payloadMetadataRow, string, error) {
	queryLimit := int64(limit + 1)
	var rows []payloadMetadataRow
	var err error
	if cursor == nil {
		switch status {
		case "unfetched":
			result, rerr := h.Store.ListPayloadMetadataUnfetched(ctx, sqlc.ListPayloadMetadataUnfetchedParams{
				RecipientBotID: recipient,
				JobID:          jobID,
				Limit:          queryLimit,
			})
			rows = mapPayloadRowsUnfetched(result)
			err = rerr
		case "fetched":
			result, rerr := h.Store.ListPayloadMetadataFetched(ctx, sqlc.ListPayloadMetadataFetchedParams{
				RecipientBotID: recipient,
				JobID:          jobID,
				Limit:          queryLimit,
			})
			rows = mapPayloadRowsFetched(result)
			err = rerr
		default:
			result, rerr := h.Store.ListPayloadMetadataAll(ctx, sqlc.ListPayloadMetadataAllParams{
				RecipientBotID: recipient,
				JobID:          jobID,
				Limit:          queryLimit,
			})
			rows = mapPayloadRowsAll(result)
			err = rerr
		}
	} else {
		switch status {
		case "unfetched":
			result, rerr := h.Store.ListPayloadMetadataUnfetchedAfter(ctx, sqlc.ListPayloadMetadataUnfetchedAfterParams{
				RecipientBotID:  recipient,
				JobID:           jobID,
				CursorCreatedAt: cursor.CreatedAt,
				CursorPayloadID: cursor.PayloadID,
				Limit:           queryLimit,
			})
			rows = mapPayloadRowsUnfetchedAfter(result)
			err = rerr
		case "fetched":
			result, rerr := h.Store.ListPayloadMetadataFetchedAfter(ctx, sqlc.ListPayloadMetadataFetchedAfterParams{
				RecipientBotID:  recipient,
				JobID:           jobID,
				CursorCreatedAt: cursor.CreatedAt,
				CursorPayloadID: cursor.PayloadID,
				Limit:           queryLimit,
			})
			rows = mapPayloadRowsFetchedAfter(result)
			err = rerr
		default:
			result, rerr := h.Store.ListPayloadMetadataAllAfter(ctx, sqlc.ListPayloadMetadataAllAfterParams{
				RecipientBotID:  recipient,
				JobID:           jobID,
				CursorCreatedAt: cursor.CreatedAt,
				CursorPayloadID: cursor.PayloadID,
				Limit:           queryLimit,
			})
			rows = mapPayloadRowsAllAfter(result)
			err = rerr
		}
	}
	if err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(rows) > limit {
		last := rows[limit-1]
		encoded, err := encodePayloadCursor(payloadCursor{Status: status, JobID: jobID, CreatedAt: last.CreatedAt, PayloadID: last.PayloadID})
		if err != nil {
			return nil, "", err
		}
		nextCursor = encoded
		rows = rows[:limit]
	}
	return rows, nextCursor, nil
}

func (h *PayloadHandler) now() time.Time {
	if h.Clock == nil {
		return time.Now().UTC()
	}
	return h.Clock().UTC()
}

func encodePayloadCursor(cursor payloadCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodePayloadCursor(raw string) (payloadCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return payloadCursor{}, err
	}
	var cursor payloadCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return payloadCursor{}, err
	}
	return cursor, nil
}

func mapPayloadRowsUnfetched(rows []sqlc.ListPayloadMetadataUnfetchedRow) []payloadMetadataRow {
	mapped := make([]payloadMetadataRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, payloadMetadataRow{
			PayloadID:   row.PayloadID,
			JobID:       row.JobID,
			PayloadKind: row.PayloadKind,
			CreatedAt:   row.CreatedAt,
		})
	}
	return mapped
}

func mapPayloadRowsUnfetchedAfter(rows []sqlc.ListPayloadMetadataUnfetchedAfterRow) []payloadMetadataRow {
	mapped := make([]payloadMetadataRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, payloadMetadataRow{
			PayloadID:   row.PayloadID,
			JobID:       row.JobID,
			PayloadKind: row.PayloadKind,
			CreatedAt:   row.CreatedAt,
		})
	}
	return mapped
}

func mapPayloadRowsFetched(rows []sqlc.ListPayloadMetadataFetchedRow) []payloadMetadataRow {
	mapped := make([]payloadMetadataRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, payloadMetadataRow{
			PayloadID:   row.PayloadID,
			JobID:       row.JobID,
			PayloadKind: row.PayloadKind,
			CreatedAt:   row.CreatedAt,
		})
	}
	return mapped
}

func mapPayloadRowsFetchedAfter(rows []sqlc.ListPayloadMetadataFetchedAfterRow) []payloadMetadataRow {
	mapped := make([]payloadMetadataRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, payloadMetadataRow{
			PayloadID:   row.PayloadID,
			JobID:       row.JobID,
			PayloadKind: row.PayloadKind,
			CreatedAt:   row.CreatedAt,
		})
	}
	return mapped
}

func mapPayloadRowsAll(rows []sqlc.ListPayloadMetadataAllRow) []payloadMetadataRow {
	mapped := make([]payloadMetadataRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, payloadMetadataRow{
			PayloadID:   row.PayloadID,
			JobID:       row.JobID,
			PayloadKind: row.PayloadKind,
			CreatedAt:   row.CreatedAt,
		})
	}
	return mapped
}

func mapPayloadRowsAllAfter(rows []sqlc.ListPayloadMetadataAllAfterRow) []payloadMetadataRow {
	mapped := make([]payloadMetadataRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, payloadMetadataRow{
			PayloadID:   row.PayloadID,
			JobID:       row.JobID,
			PayloadKind: row.PayloadKind,
			CreatedAt:   row.CreatedAt,
		})
	}
	return mapped
}
