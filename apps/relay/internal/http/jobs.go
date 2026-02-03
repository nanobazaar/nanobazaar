package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nanobazaar/relay/internal/domain"
	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

const (
	encAlgSealBox                = "libsodium.crypto_box_seal.x25519.xsalsa20poly1305"
	jobDefaultExpiry             = 48 * time.Hour
	jobMaxExpiry                 = 7 * 24 * time.Hour
	chargeMaxExpiry              = 24 * time.Hour
	jobRequestedEventType        = "job.requested"
	jobChargeCreatedEventType    = "job.charge_created"
	jobPaidEventType             = "job.paid"
	jobPayloadAvailableEventType = "job.payload_available"
	jobCancelledEventType        = "job.cancelled"
	jobExpiredEventType          = "job.expired"
	payloadKindRequest           = "request"
	payloadKindDeliver           = "deliverable"
	payloadKindMessage           = "message"
	cursorDelimiter              = "|"
	maxPayloadBytes              = 64 * 1024
)

var jobStatusSet = map[string]struct{}{
	string(domain.JobRequested):     {},
	string(domain.JobChargeCreated): {},
	string(domain.JobPaid):          {},
	string(domain.JobDelivered):     {},
	string(domain.JobCancelled):     {},
	string(domain.JobExpired):       {},
}

// JobHandler handles job lifecycle endpoints.
type JobHandler struct {
	Store     *store.Store
	Metrics   *metrics.Registry
	Clock     func() time.Time
	StreamHub StreamNotifier
}

func NewJobHandler(store *store.Store, metrics *metrics.Registry) *JobHandler {
	return &JobHandler{Store: store, Metrics: metrics, Clock: time.Now}
}

type jobCreateRequest struct {
	JobID          string               `json:"job_id"`
	OfferID        string               `json:"offer_id"`
	JobExpiresAt   string               `json:"job_expires_at"`
	RequestPayload payloadEnvelopeInput `json:"request_payload"`
}

type chargeCreateRequest struct {
	ChargeID        string `json:"charge_id"`
	Address         string `json:"address"`
	AmountRaw       string `json:"amount_raw"`
	ChargeExpiresAt string `json:"charge_expires_at"`
	ChargeSig       string `json:"charge_sig_ed25519"`
}

type markPaidRequest struct {
	Verifier          string `json:"verifier"`
	PaymentBlockHash  string `json:"payment_block_hash"`
	ObservedAt        string `json:"observed_at"`
	AmountRawReceived string `json:"amount_raw_received"`
}

type deliverRequest struct {
	Payload payloadEnvelopeInput `json:"payload"`
}

type payloadEnvelopeInput struct {
	PayloadID     string `json:"payload_id"`
	PayloadKind   string `json:"payload_kind"`
	EncAlg        string `json:"enc_alg"`
	RecipientKid  string `json:"recipient_kid"`
	CiphertextB64 string `json:"ciphertext_b64"`
}

type jobResponse struct {
	JobID             string          `json:"job_id"`
	OfferID           string          `json:"offer_id"`
	BuyerBotID        string          `json:"buyer_bot_id"`
	SellerBotID       string          `json:"seller_bot_id"`
	Status            string          `json:"status"`
	PriceRaw          string          `json:"price_raw"`
	TurnaroundSeconds int64           `json:"turnaround_seconds"`
	CreatedAt         string          `json:"created_at"`
	JobExpiresAt      string          `json:"job_expires_at"`
	RequestPayloadID  string          `json:"request_payload_id,omitempty"`
	Charge            *chargeResponse `json:"charge,omitempty"`
	PaidAt            string          `json:"paid_at,omitempty"`
	DeliveredAt       string          `json:"delivered_at,omitempty"`
}

type chargeResponse struct {
	ChargeID        string `json:"charge_id"`
	Address         string `json:"address"`
	AmountRaw       string `json:"amount_raw"`
	ChargeExpiresAt string `json:"charge_expires_at"`
	ChargeSig       string `json:"charge_sig_ed25519"`
}

type jobListResponse struct {
	Jobs       []jobResponse `json:"jobs"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type deliverResponse struct {
	PayloadID   string `json:"payload_id"`
	JobStatus   string `json:"job_status,omitempty"`
	DeliveredAt string `json:"delivered_at,omitempty"`
}

func (h *JobHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	var payload jobCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.JobID == "" || payload.OfferID == "" || payload.RequestPayload.PayloadID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing fields")
		return
	}
	payloadBytes, err := validatePayloadInput(payload.RequestPayload, map[string]struct{}{payloadKindRequest: {}})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	botID := r.Header.Get(headerBotID)
	if botID == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	offer, err := h.Store.GetOffer(r.Context(), payload.OfferID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusBadRequest, "offer not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "offer lookup failed")
		return
	}
	if offer.Status != string(domain.OfferActive) {
		writeJSONError(w, http.StatusConflict, "offer not active")
		return
	}

	now := h.now()
	jobExpiresAt := now.Add(jobDefaultExpiry)
	if payload.JobExpiresAt != "" {
		parsed, err := parseTime(payload.JobExpiresAt)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid job_expires_at")
			return
		}
		if parsed.Before(now) {
			writeJSONError(w, http.StatusBadRequest, "job_expires_at in past")
			return
		}
		if parsed.After(now.Add(jobMaxExpiry)) {
			writeJSONError(w, http.StatusBadRequest, "job_expires_at too far")
			return
		}
		jobExpiresAt = parsed
	}

	tx, err := h.Store.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job create failed")
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

	qtx := sqlc.New(tx)
	createErr := qtx.CreateJob(r.Context(), sqlc.CreateJobParams{
		JobID:             payload.JobID,
		OfferID:           payload.OfferID,
		BuyerBotID:        botID,
		SellerBotID:       offer.SellerBotID,
		Status:            string(domain.JobRequested),
		PriceRaw:          offer.PriceRaw,
		TurnaroundSeconds: offer.TurnaroundSeconds,
		CreatedAt:         now,
		JobExpiresAt:      jobExpiresAt,
		RequestPayloadID:  payload.RequestPayload.PayloadID,
		ChargeID:          sql.NullString{},
		ChargeAddress:     sql.NullString{},
		ChargeAmountRaw:   sql.NullString{},
		ChargeExpiresAt:   sql.NullTime{},
		ChargeSigEd25519:  sql.NullString{},
		PaidAt:            sql.NullTime{},
		DeliveredAt:       sql.NullTime{},
		CancelledAt:       sql.NullTime{},
		ExpiredAt:         sql.NullTime{},
		PaymentVerifier:   sql.NullString{},
		PaymentBlockHash:  sql.NullString{},
		PaymentObservedAt: sql.NullTime{},
		AmountRawReceived: sql.NullString{},
	})
	if createErr != nil {
		if isConstraintError(createErr) {
			writeJSONError(w, http.StatusConflict, "job_id already exists")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "job create failed")
		return
	}

	payloadErr := qtx.CreatePayload(r.Context(), sqlc.CreatePayloadParams{
		PayloadID:      payload.RequestPayload.PayloadID,
		RecipientBotID: offer.SellerBotID,
		JobID:          payload.JobID,
		SenderBotID:    botID,
		PayloadKind:    payload.RequestPayload.PayloadKind,
		EncAlg:         payload.RequestPayload.EncAlg,
		RecipientKid:   payload.RequestPayload.RecipientKid,
		CiphertextB64:  payload.RequestPayload.CiphertextB64,
		CreatedAt:      now,
		FetchedAt:      sql.NullTime{},
	})
	if payloadErr != nil {
		if isConstraintError(payloadErr) {
			writeJSONError(w, http.StatusConflict, "payload_id already used")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "payload create failed")
		return
	}

	if err := emitEventTx(r.Context(), qtx, h.StreamHub, offer.SellerBotID, jobRequestedEventType, map[string]any{
		"job_id":             payload.JobID,
		"offer_id":           payload.OfferID,
		"buyer_bot_id":       botID,
		"seller_bot_id":      offer.SellerBotID,
		"price_raw":          offer.PriceRaw,
		"turnaround_seconds": offer.TurnaroundSeconds,
		"request_payload_id": payload.RequestPayload.PayloadID,
		"job_expires_at":     jobExpiresAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "event create failed")
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job create failed")
		return
	}

	if h.Metrics != nil {
		h.Metrics.AddPayloadBytes(int64(payloadBytes))
		h.Metrics.AddPendingPayloads(1)
	}

	job, err := h.Store.GetJob(r.Context(), payload.JobID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, jobToResponse(job))
}

func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing job_id")
		return
	}
	job, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	job, err = h.applyExpiry(r.Context(), job)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != job.BuyerBotID && caller != job.SellerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeJSON(w, http.StatusOK, jobToResponse(job))
}

func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	role := r.URL.Query().Get("role")
	if role != "buyer" && role != "seller" {
		writeJSONError(w, http.StatusBadRequest, "invalid role")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	statuses, statusFilter, err := parseStatusFilter(r.URL.Query()["status"])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var createdSince *time.Time
	if value := r.URL.Query().Get("created_since"); value != "" {
		parsed, err := parseTime(value)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid created_since")
			return
		}
		createdSince = &parsed
	}

	cursorValue := r.URL.Query().Get("cursor")
	var cursorCreatedAt *time.Time
	var cursorJobID *string
	if cursorValue != "" {
		createdAt, jobID, err := decodeCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursorCreatedAt = &createdAt
		cursorJobID = &jobID
	}

	jobs := make([]jobResponse, 0, limit)
	var nextCursor string
	var batchCursorCreatedAt *time.Time
	var batchCursorJobID *string
	batchCursorCreatedAt = cursorCreatedAt
	batchCursorJobID = cursorJobID

	for len(jobs) < limit {
		batch, err := h.fetchJobs(r.Context(), role, caller, createdSince, batchCursorCreatedAt, batchCursorJobID, limit, statuses)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "job list failed")
			return
		}
		if len(batch) == 0 {
			break
		}

		usedAllBatch := true
		for _, job := range batch {
			updated, err := h.applyExpiry(r.Context(), job)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
				return
			}
			if statusFilter != nil && !statusFilter[updated.Status] {
				continue
			}
			jobs = append(jobs, jobToResponse(updated))
			if len(jobs) == limit {
				usedAllBatch = false
				break
			}
		}

		last := batch[len(batch)-1]
		batchCursorCreatedAt = &last.CreatedAt
		batchCursorJobID = &last.JobID

		if len(jobs) == limit {
			hasMore := !usedAllBatch || len(batch) == limit
			if hasMore {
				lastJob := jobs[len(jobs)-1]
				parsedCreatedAt, err := parseTime(lastJob.CreatedAt)
				if err == nil {
					nextCursor = encodeCursor(parsedCreatedAt, lastJob.JobID)
				}
			}
			break
		}

		if len(batch) < limit {
			break
		}
	}

	writeJSON(w, http.StatusOK, jobListResponse{Jobs: jobs, NextCursor: nextCursor})
}

func (h *JobHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing job_id")
		return
	}
	job, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	job, err = h.applyExpiry(r.Context(), job)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != job.BuyerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if isTerminalJob(job.Status) {
		writeJSONError(w, http.StatusConflict, "job not mutable")
		return
	}
	if job.Status != string(domain.JobRequested) {
		writeJSONError(w, http.StatusConflict, "job not cancellable")
		return
	}
	if job.ChargeID.Valid {
		writeJSONError(w, http.StatusConflict, "charge already attached")
		return
	}

	if err := h.Store.UpdateJobCancel(r.Context(), sqlc.UpdateJobCancelParams{JobID: jobID, CancelledAt: sql.NullTime{Time: h.now(), Valid: true}}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job cancel failed")
		return
	}
	updated, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	if err := emitEvent(r.Context(), h.Store, h.StreamHub, updated.SellerBotID, jobCancelledEventType, map[string]any{
		"job_id":       updated.JobID,
		"cancelled_at": updated.CancelledAt.Time.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "event create failed")
		return
	}
	writeJSON(w, http.StatusOK, jobToResponse(updated))
}

func (h *JobHandler) Charge(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing job_id")
		return
	}
	job, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	job, err = h.applyExpiry(r.Context(), job)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != job.SellerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if isTerminalJob(job.Status) {
		writeJSONError(w, http.StatusConflict, "job not mutable")
		return
	}
	if job.Status != string(domain.JobRequested) {
		writeJSONError(w, http.StatusConflict, "job not chargeable")
		return
	}
	if job.ChargeID.Valid {
		writeJSONError(w, http.StatusConflict, "charge already attached")
		return
	}

	var payload chargeCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.ChargeID == "" || payload.Address == "" || payload.AmountRaw == "" || payload.ChargeExpiresAt == "" || payload.ChargeSig == "" {
		writeJSONError(w, http.StatusBadRequest, "missing fields")
		return
	}

	chargeExpiresAt, err := parseTime(payload.ChargeExpiresAt)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid charge_expires_at")
		return
	}
	now := h.now()
	if chargeExpiresAt.Before(now) {
		writeJSONError(w, http.StatusBadRequest, "charge_expires_at in past")
		return
	}
	if chargeExpiresAt.After(now.Add(chargeMaxExpiry)) {
		writeJSONError(w, http.StatusBadRequest, "charge_expires_at too far")
		return
	}

	count, err := h.Store.CountActiveJobsByChargeAddress(r.Context(), sqlc.CountActiveJobsByChargeAddressParams{
		SellerBotID:   job.SellerBotID,
		ChargeAddress: sql.NullString{String: payload.Address, Valid: true},
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "charge validation failed")
		return
	}
	if count > 0 {
		writeJSONError(w, http.StatusConflict, "charge address already in use")
		return
	}

	if err := h.Store.UpdateJobCharge(r.Context(), sqlc.UpdateJobChargeParams{
		JobID:            jobID,
		ChargeID:         sql.NullString{String: payload.ChargeID, Valid: true},
		ChargeAddress:    sql.NullString{String: payload.Address, Valid: true},
		ChargeAmountRaw:  sql.NullString{String: payload.AmountRaw, Valid: true},
		ChargeExpiresAt:  sql.NullTime{Time: chargeExpiresAt, Valid: true},
		ChargeSigEd25519: sql.NullString{String: payload.ChargeSig, Valid: true},
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "charge attach failed")
		return
	}

	updated, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}

	if err := emitEvent(r.Context(), h.Store, h.StreamHub, updated.BuyerBotID, jobChargeCreatedEventType, map[string]any{
		"job_id":             updated.JobID,
		"charge_id":          payload.ChargeID,
		"address":            payload.Address,
		"amount_raw":         payload.AmountRaw,
		"charge_expires_at":  chargeExpiresAt.UTC().Format(time.RFC3339Nano),
		"charge_sig_ed25519": payload.ChargeSig,
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "event create failed")
		return
	}

	writeJSON(w, http.StatusOK, jobToResponse(updated))
}

func (h *JobHandler) MarkPaid(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing job_id")
		return
	}
	job, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	job, err = h.applyExpiry(r.Context(), job)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != job.SellerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if isTerminalJob(job.Status) {
		writeJSONError(w, http.StatusConflict, "job not mutable")
		return
	}
	if job.Status != string(domain.JobChargeCreated) {
		writeJSONError(w, http.StatusConflict, "job not chargeable")
		return
	}
	if job.ChargeExpiresAt.Valid && h.now().After(job.ChargeExpiresAt.Time) {
		writeJSONError(w, http.StatusConflict, "charge expired")
		return
	}

	var payload markPaidRequest
	body, _ := readAll(r)
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json")
			return
		}
	}

	var observedAt sql.NullTime
	if payload.ObservedAt != "" {
		parsed, err := parseTime(payload.ObservedAt)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid observed_at")
			return
		}
		observedAt = sql.NullTime{Time: parsed, Valid: true}
	}

	params := sqlc.UpdateJobMarkPaidParams{
		JobID:             jobID,
		PaidAt:            sql.NullTime{Time: h.now(), Valid: true},
		PaymentVerifier:   nullString(payload.Verifier),
		PaymentBlockHash:  nullString(payload.PaymentBlockHash),
		PaymentObservedAt: observedAt,
		AmountRawReceived: nullString(payload.AmountRawReceived),
	}
	if err := h.Store.UpdateJobMarkPaid(r.Context(), params); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "mark paid failed")
		return
	}
	updated, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}

	eventPayload := map[string]any{
		"job_id":  updated.JobID,
		"paid_at": updated.PaidAt.Time.UTC().Format(time.RFC3339Nano),
	}
	if payload.Verifier != "" {
		eventPayload["verifier"] = payload.Verifier
	}
	if payload.PaymentBlockHash != "" {
		eventPayload["payment_block_hash"] = payload.PaymentBlockHash
	}
	if payload.ObservedAt != "" {
		eventPayload["observed_at"] = payload.ObservedAt
	}
	if payload.AmountRawReceived != "" {
		eventPayload["amount_raw_received"] = payload.AmountRawReceived
	}
	if err := emitEvent(r.Context(), h.Store, h.StreamHub, updated.BuyerBotID, jobPaidEventType, eventPayload); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "event create failed")
		return
	}

	writeJSON(w, http.StatusOK, jobToResponse(updated))
}

func (h *JobHandler) Deliver(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing job_id")
		return
	}
	job, err := h.Store.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	job, err = h.applyExpiry(r.Context(), job)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != job.SellerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if isTerminalJob(job.Status) {
		writeJSONError(w, http.StatusConflict, "job not mutable")
		return
	}

	var payload deliverRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.Payload.PayloadID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing fields")
		return
	}
	payloadBytes, err := validatePayloadInput(payload.Payload, map[string]struct{}{payloadKindDeliver: {}, payloadKindMessage: {}})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if payload.Payload.PayloadKind == payloadKindDeliver && job.Status != string(domain.JobPaid) {
		writeJSONError(w, http.StatusConflict, "job not paid")
		return
	}

	now := h.now()
	if payload.Payload.PayloadKind == payloadKindDeliver {
		resp, err := h.deliverAndUpdate(r.Context(), job, payload.Payload, now)
		if err != nil {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if h.Metrics != nil {
			h.Metrics.AddPayloadBytes(int64(payloadBytes))
			h.Metrics.AddPendingPayloads(1)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if err := h.insertPayload(r.Context(), job, payload.Payload, now); err != nil {
		if errors.Is(err, errConflict) {
			writeJSONError(w, http.StatusConflict, "payload_id already used")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "payload create failed")
		return
	}
	if h.Metrics != nil {
		h.Metrics.AddPayloadBytes(int64(payloadBytes))
		h.Metrics.AddPendingPayloads(1)
	}

	updated, err := h.Store.GetJob(r.Context(), job.JobID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, deliverResponse{
		PayloadID:   payload.Payload.PayloadID,
		JobStatus:   updated.Status,
		DeliveredAt: formatTime(updated.DeliveredAt),
	})
}

var errConflict = errors.New("conflict")

func (h *JobHandler) deliverAndUpdate(ctx context.Context, job sqlc.Job, payload payloadEnvelopeInput, now time.Time) (deliverResponse, error) {
	tx, err := h.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return deliverResponse{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	qtx := sqlc.New(tx)
	if err := qtx.CreatePayload(ctx, sqlc.CreatePayloadParams{
		PayloadID:      payload.PayloadID,
		RecipientBotID: job.BuyerBotID,
		JobID:          job.JobID,
		SenderBotID:    job.SellerBotID,
		PayloadKind:    payload.PayloadKind,
		EncAlg:         payload.EncAlg,
		RecipientKid:   payload.RecipientKid,
		CiphertextB64:  payload.CiphertextB64,
		CreatedAt:      now,
		FetchedAt:      sql.NullTime{},
	}); err != nil {
		if isConstraintError(err) {
			return deliverResponse{}, errConflict
		}
		return deliverResponse{}, err
	}

	if err := qtx.UpdateJobDeliver(ctx, sqlc.UpdateJobDeliverParams{JobID: job.JobID, DeliveredAt: sql.NullTime{Time: now, Valid: true}}); err != nil {
		return deliverResponse{}, err
	}

	if err := emitEventTx(ctx, qtx, h.StreamHub, job.BuyerBotID, jobPayloadAvailableEventType, map[string]any{
		"job_id":       job.JobID,
		"payload_id":   payload.PayloadID,
		"payload_kind": payload.PayloadKind,
	}); err != nil {
		return deliverResponse{}, err
	}

	updated, err := qtx.GetJob(ctx, job.JobID)
	if err != nil {
		return deliverResponse{}, err
	}
	if updated.Status != string(domain.JobDelivered) {
		return deliverResponse{}, errConflict
	}

	if err := tx.Commit(); err != nil {
		return deliverResponse{}, err
	}

	return deliverResponse{
		PayloadID:   payload.PayloadID,
		JobStatus:   updated.Status,
		DeliveredAt: formatTime(updated.DeliveredAt),
	}, nil
}

func (h *JobHandler) insertPayload(ctx context.Context, job sqlc.Job, payload payloadEnvelopeInput, now time.Time) error {
	err := h.Store.CreatePayload(ctx, sqlc.CreatePayloadParams{
		PayloadID:      payload.PayloadID,
		RecipientBotID: job.BuyerBotID,
		JobID:          job.JobID,
		SenderBotID:    job.SellerBotID,
		PayloadKind:    payload.PayloadKind,
		EncAlg:         payload.EncAlg,
		RecipientKid:   payload.RecipientKid,
		CiphertextB64:  payload.CiphertextB64,
		CreatedAt:      now,
		FetchedAt:      sql.NullTime{},
	})
	if err != nil {
		if isConstraintError(err) {
			return errConflict
		}
		return err
	}
	if err := emitEvent(ctx, h.Store, h.StreamHub, job.BuyerBotID, jobPayloadAvailableEventType, map[string]any{
		"job_id":       job.JobID,
		"payload_id":   payload.PayloadID,
		"payload_kind": payload.PayloadKind,
	}); err != nil {
		return err
	}
	return nil
}

func (h *JobHandler) fetchJobs(ctx context.Context, role, caller string, createdSince *time.Time, cursorCreatedAt *time.Time, cursorJobID *string, limit int, statuses []string) ([]sqlc.Job, error) {
	if role == "buyer" {
		if len(statuses) > 0 {
			if createdSince != nil {
				if cursorCreatedAt != nil && cursorJobID != nil {
					return h.Store.ListJobsByBuyerSinceAfterWithStatus(ctx, sqlc.ListJobsByBuyerSinceAfterWithStatusParams{
						BuyerBotID:      caller,
						CreatedSince:    *createdSince,
						CursorCreatedAt: *cursorCreatedAt,
						CursorJobID:     *cursorJobID,
						Statuses:        statuses,
						Limit:           int64(limit),
					})
				}
				return h.Store.ListJobsByBuyerSinceWithStatus(ctx, sqlc.ListJobsByBuyerSinceWithStatusParams{
					BuyerBotID:   caller,
					CreatedSince: *createdSince,
					Statuses:     statuses,
					Limit:        int64(limit),
				})
			}
			if cursorCreatedAt != nil && cursorJobID != nil {
				return h.Store.ListJobsByBuyerNewestAfterWithStatus(ctx, sqlc.ListJobsByBuyerNewestAfterWithStatusParams{
					BuyerBotID:      caller,
					CursorCreatedAt: *cursorCreatedAt,
					CursorJobID:     *cursorJobID,
					Statuses:        statuses,
					Limit:           int64(limit),
				})
			}
			return h.Store.ListJobsByBuyerNewestWithStatus(ctx, sqlc.ListJobsByBuyerNewestWithStatusParams{
				BuyerBotID: caller,
				Statuses:   statuses,
				Limit:      int64(limit),
			})
		}

		if createdSince != nil {
			if cursorCreatedAt != nil && cursorJobID != nil {
				return h.Store.ListJobsByBuyerSinceAfter(ctx, sqlc.ListJobsByBuyerSinceAfterParams{
					BuyerBotID:      caller,
					CreatedSince:    *createdSince,
					CursorCreatedAt: *cursorCreatedAt,
					CursorJobID:     *cursorJobID,
					Limit:           int64(limit),
				})
			}
			return h.Store.ListJobsByBuyerSince(ctx, sqlc.ListJobsByBuyerSinceParams{
				BuyerBotID:   caller,
				CreatedSince: *createdSince,
				Limit:        int64(limit),
			})
		}
		if cursorCreatedAt != nil && cursorJobID != nil {
			return h.Store.ListJobsByBuyerNewestAfter(ctx, sqlc.ListJobsByBuyerNewestAfterParams{
				BuyerBotID:      caller,
				CursorCreatedAt: *cursorCreatedAt,
				CursorJobID:     *cursorJobID,
				Limit:           int64(limit),
			})
		}
		return h.Store.ListJobsByBuyerNewest(ctx, sqlc.ListJobsByBuyerNewestParams{
			BuyerBotID: caller,
			Limit:      int64(limit),
		})
	}

	if len(statuses) > 0 {
		if createdSince != nil {
			if cursorCreatedAt != nil && cursorJobID != nil {
				return h.Store.ListJobsBySellerSinceAfterWithStatus(ctx, sqlc.ListJobsBySellerSinceAfterWithStatusParams{
					SellerBotID:     caller,
					CreatedSince:    *createdSince,
					CursorCreatedAt: *cursorCreatedAt,
					CursorJobID:     *cursorJobID,
					Statuses:        statuses,
					Limit:           int64(limit),
				})
			}
			return h.Store.ListJobsBySellerSinceWithStatus(ctx, sqlc.ListJobsBySellerSinceWithStatusParams{
				SellerBotID:  caller,
				CreatedSince: *createdSince,
				Statuses:     statuses,
				Limit:        int64(limit),
			})
		}
		if cursorCreatedAt != nil && cursorJobID != nil {
			return h.Store.ListJobsBySellerNewestAfterWithStatus(ctx, sqlc.ListJobsBySellerNewestAfterWithStatusParams{
				SellerBotID:     caller,
				CursorCreatedAt: *cursorCreatedAt,
				CursorJobID:     *cursorJobID,
				Statuses:        statuses,
				Limit:           int64(limit),
			})
		}
		return h.Store.ListJobsBySellerNewestWithStatus(ctx, sqlc.ListJobsBySellerNewestWithStatusParams{
			SellerBotID: caller,
			Statuses:    statuses,
			Limit:       int64(limit),
		})
	}

	if createdSince != nil {
		if cursorCreatedAt != nil && cursorJobID != nil {
			return h.Store.ListJobsBySellerSinceAfter(ctx, sqlc.ListJobsBySellerSinceAfterParams{
				SellerBotID:     caller,
				CreatedSince:    *createdSince,
				CursorCreatedAt: *cursorCreatedAt,
				CursorJobID:     *cursorJobID,
				Limit:           int64(limit),
			})
		}
		return h.Store.ListJobsBySellerSince(ctx, sqlc.ListJobsBySellerSinceParams{
			SellerBotID:  caller,
			CreatedSince: *createdSince,
			Limit:        int64(limit),
		})
	}
	if cursorCreatedAt != nil && cursorJobID != nil {
		return h.Store.ListJobsBySellerNewestAfter(ctx, sqlc.ListJobsBySellerNewestAfterParams{
			SellerBotID:     caller,
			CursorCreatedAt: *cursorCreatedAt,
			CursorJobID:     *cursorJobID,
			Limit:           int64(limit),
		})
	}
	return h.Store.ListJobsBySellerNewest(ctx, sqlc.ListJobsBySellerNewestParams{
		SellerBotID: caller,
		Limit:       int64(limit),
	})
}

func (h *JobHandler) applyExpiry(ctx context.Context, job sqlc.Job) (sqlc.Job, error) {
	if isTerminalJob(job.Status) {
		return job, nil
	}
	now := h.now()
	if now.After(job.JobExpiresAt) {
		return h.expireJob(ctx, job, now)
	}
	if job.Status == string(domain.JobChargeCreated) && job.ChargeExpiresAt.Valid && now.After(job.ChargeExpiresAt.Time) {
		return h.expireJob(ctx, job, now)
	}
	if job.Status == string(domain.JobPaid) && job.PaidAt.Valid {
		deadline := job.PaidAt.Time.Add(time.Duration(job.TurnaroundSeconds) * time.Second).Add(7 * 24 * time.Hour)
		if now.After(deadline) {
			return h.expireJob(ctx, job, now)
		}
	}
	return job, nil
}

func (h *JobHandler) expireJob(ctx context.Context, job sqlc.Job, now time.Time) (sqlc.Job, error) {
	previous := job.Status
	if err := h.Store.UpdateJobExpire(ctx, sqlc.UpdateJobExpireParams{JobID: job.JobID, ExpiredAt: sql.NullTime{Time: now, Valid: true}}); err != nil {
		return job, err
	}
	updated, err := h.Store.GetJob(ctx, job.JobID)
	if err != nil {
		return job, err
	}
	if updated.Status == string(domain.JobExpired) {
		if err := h.emitJobExpired(ctx, updated, now, previous); err != nil {
			return updated, err
		}
	}
	return updated, nil
}

func (h *JobHandler) emitJobExpired(ctx context.Context, job sqlc.Job, now time.Time, previousStatus string) error {
	data := map[string]any{
		"job_id":     job.JobID,
		"expired_at": now.UTC().Format(time.RFC3339Nano),
	}
	if previousStatus != "" {
		data["previous_status"] = previousStatus
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	recipients := []string{job.BuyerBotID, job.SellerBotID}
	if job.BuyerBotID == job.SellerBotID {
		recipients = []string{job.BuyerBotID}
	}
	for _, recipient := range recipients {
		if err := h.Store.CreateEvent(ctx, sqlc.CreateEventParams{
			RecipientBotID: recipient,
			EventType:      jobExpiredEventType,
			DataJson:       string(payload),
			CreatedAt:      now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h *JobHandler) now() time.Time {
	if h.Clock == nil {
		return time.Now().UTC()
	}
	return h.Clock().UTC()
}

func parseStatusFilter(values []string) ([]string, map[string]bool, error) {
	if len(values) == 0 {
		return nil, nil, nil
	}
	seen := make(map[string]struct{})
	statuses := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		normalized := strings.ToUpper(value)
		if _, ok := jobStatusSet[normalized]; !ok {
			return nil, nil, fmt.Errorf("invalid status")
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		statuses = append(statuses, normalized)
	}
	if len(statuses) == 0 {
		return nil, nil, nil
	}
	filter := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		filter[status] = true
	}
	return statuses, filter, nil
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("empty time")
	}
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, errors.New("timestamp must be UTC")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	canonical := parsed.UTC().Format(time.RFC3339Nano)
	if canonical != value {
		return time.Time{}, errors.New("timestamp must be canonical RFC3339")
	}
	return parsed.UTC(), nil
}

func encodeCursor(createdAt time.Time, jobID string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + cursorDelimiter + jobID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(value string) (time.Time, string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(decoded), cursorDelimiter, 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	createdAt, err := parseTime(parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	if parts[1] == "" {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	return createdAt, parts[1], nil
}

func validatePayloadInput(payload payloadEnvelopeInput, allowedKinds map[string]struct{}) (int, error) {
	if payload.PayloadID == "" || payload.PayloadKind == "" || payload.EncAlg == "" || payload.RecipientKid == "" || payload.CiphertextB64 == "" {
		return 0, errors.New("missing fields")
	}
	if payload.EncAlg != encAlgSealBox {
		return 0, errors.New("invalid enc_alg")
	}
	if _, ok := allowedKinds[payload.PayloadKind]; !ok {
		return 0, errors.New("invalid payload_kind")
	}
	decodedLen, err := decodeCiphertextB64(payload.CiphertextB64, maxPayloadBytes)
	if err != nil {
		return 0, err
	}
	return decodedLen, nil
}

func decodeCiphertextB64(value string, maxBytes int) (int, error) {
	if value == "" {
		return 0, errors.New("invalid ciphertext_b64")
	}
	if strings.Contains(value, "=") {
		return 0, errors.New("invalid ciphertext_b64")
	}
	if maxBytes > 0 {
		maxEncoded := base64.RawURLEncoding.EncodedLen(maxBytes)
		if len(value) > maxEncoded {
			return 0, errors.New("ciphertext_b64 too large")
		}
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, errors.New("invalid ciphertext_b64")
	}
	if maxBytes > 0 && len(decoded) > maxBytes {
		return 0, errors.New("ciphertext_b64 too large")
	}
	return len(decoded), nil
}

func jobToResponse(job sqlc.Job) jobResponse {
	resp := jobResponse{
		JobID:             job.JobID,
		OfferID:           job.OfferID,
		BuyerBotID:        job.BuyerBotID,
		SellerBotID:       job.SellerBotID,
		Status:            job.Status,
		PriceRaw:          job.PriceRaw,
		TurnaroundSeconds: job.TurnaroundSeconds,
		CreatedAt:         job.CreatedAt.UTC().Format(time.RFC3339Nano),
		JobExpiresAt:      job.JobExpiresAt.UTC().Format(time.RFC3339Nano),
		RequestPayloadID:  job.RequestPayloadID,
		PaidAt:            formatTime(job.PaidAt),
		DeliveredAt:       formatTime(job.DeliveredAt),
	}
	if job.ChargeID.Valid {
		resp.Charge = &chargeResponse{
			ChargeID:        job.ChargeID.String,
			Address:         job.ChargeAddress.String,
			AmountRaw:       job.ChargeAmountRaw.String,
			ChargeExpiresAt: formatTime(job.ChargeExpiresAt),
			ChargeSig:       job.ChargeSigEd25519.String,
		}
	}
	return resp
}

func formatTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func isTerminalJob(status string) bool {
	return status == string(domain.JobCancelled) || status == string(domain.JobExpired) || status == string(domain.JobDelivered)
}

func readAll(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	return io.ReadAll(r.Body)
}

func emitEvent(ctx context.Context, st *store.Store, notifier StreamNotifier, recipient, eventType string, data map[string]any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := st.CreateEvent(ctx, sqlc.CreateEventParams{
		RecipientBotID: recipient,
		EventType:      eventType,
		DataJson:       string(payload),
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return err
	}
	if notifier != nil {
		notifier.NotifyEvent(ctx, recipient, eventType, data)
	}
	return nil
}

func emitEventTx(ctx context.Context, qtx *sqlc.Queries, notifier StreamNotifier, recipient, eventType string, data map[string]any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := qtx.CreateEvent(ctx, sqlc.CreateEventParams{
		RecipientBotID: recipient,
		EventType:      eventType,
		DataJson:       string(payload),
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		return err
	}
	if notifier != nil {
		notifier.NotifyEvent(ctx, recipient, eventType, data)
	}
	return nil
}
