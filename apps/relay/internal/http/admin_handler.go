package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nanobazaar/relay/internal/domain"
	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

type AdminHandlerConfig struct {
	Store     *store.Store
	Metrics   *metrics.Registry
	StreamHub *StreamHub
}

type AdminHandler struct {
	Store     *store.Store
	Metrics   *metrics.Registry
	StreamHub *StreamHub
	Clock     func() time.Time
}

func NewAdminHandler(cfg AdminHandlerConfig) *AdminHandler {
	return &AdminHandler{
		Store:     cfg.Store,
		Metrics:   cfg.Metrics,
		StreamHub: cfg.StreamHub,
		Clock:     time.Now,
	}
}

type adminActionRequest struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

type adminOverviewResponse struct {
	Now string `json:"now"`

	Bots struct {
		Active  int64 `json:"active"`
		Revoked int64 `json:"revoked"`
		Total   int64 `json:"total"`
	} `json:"bots"`

	OffersByStatus map[string]int64 `json:"offers_by_status"`
	JobsByStatus   map[string]int64 `json:"jobs_by_status"`

	Payloads struct {
		Pending     int64 `json:"pending"`
		Total       int64 `json:"total"`
		StoredBytes int64 `json:"stored_bytes"`
	} `json:"payloads"`

	EventsTotal int64 `json:"events_total"`

	Stream struct {
		ActiveConns   int `json:"active_conns"`
		ActiveBots    int `json:"active_bots"`
		ActiveStreams int `json:"active_streams"`
	} `json:"stream"`

	NeedsAttention struct {
		RequestedStale int64 `json:"requested_stale"`
		ChargeExpired  int64 `json:"charge_expired"`
		PayloadPending int64 `json:"payload_pending"`
	} `json:"needs_attention"`
}

func (h *AdminHandler) Overview(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	now := h.now()

	var resp adminOverviewResponse
	resp.Now = now.UTC().Format(time.RFC3339Nano)
	resp.OffersByStatus = make(map[string]int64)
	resp.JobsByStatus = make(map[string]int64)

	ctx := r.Context()

	// Bots.
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM bots WHERE revoked_at IS NULL`).Scan(&resp.Bots.Active); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM bots WHERE revoked_at IS NOT NULL`).Scan(&resp.Bots.Revoked); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	resp.Bots.Total = resp.Bots.Active + resp.Bots.Revoked

	// Offers by status.
	offerRows, err := h.Store.DB.QueryContext(ctx, `SELECT status, COUNT(1) FROM offers GROUP BY status`)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	for offerRows.Next() {
		var status string
		var count int64
		if err := offerRows.Scan(&status, &count); err != nil {
			_ = offerRows.Close()
			writeJSONError(w, http.StatusInternalServerError, "overview failed")
			return
		}
		resp.OffersByStatus[status] = count
	}
	if err := offerRows.Err(); err != nil {
		_ = offerRows.Close()
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	_ = offerRows.Close()

	// Jobs by status.
	jobRows, err := h.Store.DB.QueryContext(ctx, `SELECT status, COUNT(1) FROM jobs GROUP BY status`)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	for jobRows.Next() {
		var status string
		var count int64
		if err := jobRows.Scan(&status, &count); err != nil {
			_ = jobRows.Close()
			writeJSONError(w, http.StatusInternalServerError, "overview failed")
			return
		}
		resp.JobsByStatus[status] = count
	}
	if err := jobRows.Err(); err != nil {
		_ = jobRows.Close()
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	_ = jobRows.Close()

	// Payloads.
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM payloads`).Scan(&resp.Payloads.Total); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM payloads WHERE fetched_at IS NULL`).Scan(&resp.Payloads.Pending); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(LENGTH(ciphertext_b64)), 0) FROM payloads`).Scan(&resp.Payloads.StoredBytes); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}

	// Events.
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM events`).Scan(&resp.EventsTotal); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}

	// Stream hub (best-effort, in-memory).
	if h.StreamHub != nil {
		stats := h.StreamHub.Stats()
		resp.Stream.ActiveConns = stats.ActiveConns
		resp.Stream.ActiveBots = stats.ActiveBots
		resp.Stream.ActiveStreams = stats.ActiveStreams
	}

	// Needs attention.
	staleCutoff := now.Add(-1 * time.Hour)
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM jobs WHERE status = 'REQUESTED' AND created_at < ?1`, staleCutoff).Scan(&resp.NeedsAttention.RequestedStale); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	if err := h.Store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM jobs WHERE status = 'CHARGE_CREATED' AND charge_expires_at IS NOT NULL AND charge_expires_at < ?1`, now).Scan(&resp.NeedsAttention.ChargeExpired); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "overview failed")
		return
	}
	resp.NeedsAttention.PayloadPending = resp.Payloads.Pending

	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) MetricsSnapshot(w http.ResponseWriter, _ *http.Request) {
	if h == nil || h.Metrics == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, h.Metrics.Snapshot())
}

type adminBotRow struct {
	BotID      string `json:"bot_id"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at,omitempty"`
	RevokedAt  string `json:"revoked_at,omitempty"`
	Revoked    bool   `json:"revoked"`
}

type adminBotListResponse struct {
	Bots       []adminBotRow `json:"bots"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

func (h *AdminHandler) ListBots(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	revoked, err := parseBoolPtr(strings.TrimSpace(r.URL.Query().Get("revoked")))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid revoked")
		return
	}

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
	var cursorCreatedAt *time.Time
	var cursorBotID *string
	if cursorValue != "" {
		createdAt, botID, err := decodeCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursorCreatedAt = &createdAt
		cursorBotID = &botID
	}

	where := "WHERE 1=1"
	args := make([]any, 0, 8)

	if q != "" {
		where += " AND bot_id LIKE ?"
		args = append(args, "%"+q+"%")
	}
	if revoked != nil {
		if *revoked {
			where += " AND revoked_at IS NOT NULL"
		} else {
			where += " AND revoked_at IS NULL"
		}
	}
	if cursorCreatedAt != nil && cursorBotID != nil {
		where += " AND (created_at < ? OR (created_at = ? AND bot_id < ?))"
		args = append(args, cursorCreatedAt.UTC(), cursorCreatedAt.UTC(), *cursorBotID)
	}
	args = append(args, limit+1)

	rows, err := h.Store.DB.QueryContext(r.Context(), `
SELECT bot_id, created_at, last_seen_at, revoked_at
FROM bots
`+where+`
ORDER BY created_at DESC, bot_id DESC
LIMIT ?`, args...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "bot list failed")
		return
	}
	defer rows.Close()

	resp := adminBotListResponse{Bots: make([]adminBotRow, 0, limit)}
	for rows.Next() {
		var botID string
		var createdAt time.Time
		var lastSeen sql.NullTime
		var revokedAt sql.NullTime
		if err := rows.Scan(&botID, &createdAt, &lastSeen, &revokedAt); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "bot list failed")
			return
		}
		row := adminBotRow{
			BotID:     botID,
			CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
			Revoked:   revokedAt.Valid,
		}
		if lastSeen.Valid {
			row.LastSeenAt = lastSeen.Time.UTC().Format(time.RFC3339Nano)
		}
		if revokedAt.Valid {
			row.RevokedAt = revokedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		resp.Bots = append(resp.Bots, row)
	}
	if err := rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "bot list failed")
		return
	}

	hasMore := len(resp.Bots) > limit
	if hasMore {
		resp.Bots = resp.Bots[:limit]
	}

	if hasMore && len(resp.Bots) > 0 {
		last := resp.Bots[len(resp.Bots)-1]
		createdAt, err := parseTime(last.CreatedAt)
		if err == nil {
			resp.NextCursor = encodeCursor(createdAt, last.BotID)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type adminBotDetailResponse struct {
	Bot             botResponse `json:"bot"`
	OffersTotal     int64       `json:"offers_total"`
	JobsTotal       int64       `json:"jobs_total"`
	PayloadsPending int64       `json:"payloads_pending"`
}

func (h *AdminHandler) GetBot(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	botID := chi.URLParam(r, "bot_id")
	if botID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing bot_id")
		return
	}
	bot, err := h.Store.GetBot(r.Context(), botID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "bot not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "bot lookup failed")
		return
	}

	var resp adminBotDetailResponse
	resp.Bot = botToResponse(bot)

	_ = h.Store.DB.QueryRowContext(r.Context(), `SELECT COUNT(1) FROM offers WHERE seller_bot_id = ?1`, botID).Scan(&resp.OffersTotal)
	_ = h.Store.DB.QueryRowContext(r.Context(), `SELECT COUNT(1) FROM jobs WHERE buyer_bot_id = ?1 OR seller_bot_id = ?1`, botID).Scan(&resp.JobsTotal)
	_ = h.Store.DB.QueryRowContext(r.Context(), `SELECT COUNT(1) FROM payloads WHERE recipient_bot_id = ?1 AND fetched_at IS NULL`, botID).Scan(&resp.PayloadsPending)

	writeJSON(w, http.StatusOK, resp)
}

type adminBotRevokeResponse struct {
	BotID     string `json:"bot_id"`
	Revoked   bool   `json:"revoked"`
	RevokedAt string `json:"revoked_at"`
	AuditID   int64  `json:"audit_id"`
}

func (h *AdminHandler) RevokeBot(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	botID := chi.URLParam(r, "bot_id")
	if botID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing bot_id")
		return
	}

	action, err := decodeAdminAction(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	now := h.now()

	tx, err := h.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()
	qtx := sqlc.New(tx)

	beforeBot, err := qtx.GetBot(ctx, botID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "bot not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "bot lookup failed")
		return
	}

	updated, err := qtx.UpdateBotRevoke(ctx, sqlc.UpdateBotRevokeParams{
		BotID:     botID,
		RevokedAt: sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	if !updated.RevokedAt.Valid {
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}

	revokedAt := updated.RevokedAt.Time
	cancelledAt := sql.NullTime{Time: revokedAt, Valid: true}

	offers, err := qtx.CancelOffersBySeller(ctx, sqlc.CancelOffersBySellerParams{
		SellerBotID: botID,
		CancelledAt: cancelledAt,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	for _, offer := range offers {
		if err := emitEventTx(ctx, qtx, h.StreamHub, offer.SellerBotID, offerCancelledEventType, map[string]any{
			"offer_id":     offer.OfferID,
			"cancelled_at": revokedAt.UTC().Format(time.RFC3339Nano),
			"cancelled_by": "admin",
			"reason":       action.Reason,
		}, true); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "event create failed")
			return
		}
	}

	jobs, err := qtx.CancelJobsByBot(ctx, sqlc.CancelJobsByBotParams{
		BotID:       botID,
		CancelledAt: cancelledAt,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	for _, job := range jobs {
		payload := map[string]any{
			"job_id":       job.JobID,
			"cancelled_at": revokedAt.UTC().Format(time.RFC3339Nano),
			"cancelled_by": "admin",
			"reason":       action.Reason,
		}
		recipients := []string{job.BuyerBotID, job.SellerBotID}
		if job.BuyerBotID == job.SellerBotID {
			recipients = []string{job.BuyerBotID}
		}
		for i, recipient := range recipients {
			emitJobStream := i == 0
			if err := emitEventTx(ctx, qtx, h.StreamHub, recipient, jobCancelledEventType, payload, emitJobStream); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "event create failed")
				return
			}
		}
	}

	auditID, err := h.Store.InsertAdminAuditTx(ctx, tx, store.AdminAuditEntry{
		Action:           "bot.revoke",
		TargetType:       "bot",
		TargetID:         botID,
		Reason:           action.Reason,
		Note:             action.Note,
		RequestID:        middleware.GetReqID(ctx),
		TokenFingerprint: adminTokenFingerprint(ctx),
		RemoteAddr:       r.RemoteAddr,
		UserAgent:        r.UserAgent(),
		Before:           botToResponse(beforeBot),
		After:            botToResponse(updated),
		CreatedAt:        now,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "audit failed")
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}

	writeJSON(w, http.StatusOK, adminBotRevokeResponse{
		BotID:     botID,
		Revoked:   true,
		RevokedAt: revokedAt.UTC().Format(time.RFC3339Nano),
		AuditID:   auditID,
	})
}

type adminOfferRow struct {
	OfferID           string   `json:"offer_id"`
	SellerBotID       string   `json:"seller_bot_id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Tags              []string `json:"tags"`
	PriceRaw          string   `json:"price_raw"`
	TurnaroundSeconds int64    `json:"turnaround_seconds"`
	CreatedAt         string   `json:"created_at"`
	ExpiresAt         string   `json:"expires_at,omitempty"`
	Status            string   `json:"status"`
	CancelledAt       string   `json:"cancelled_at,omitempty"`
	PurchaseCount     int64    `json:"purchase_count"`
}

type adminOfferListResponse struct {
	Offers     []adminOfferRow `json:"offers"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

const selectAdminOffersQuery = `
SELECT o.offer_id, o.seller_bot_id, o.title, o.description, o.tags_json, o.price_raw, o.turnaround_seconds, o.created_at, o.expires_at, o.status, o.cancelled_at, o.request_schema_hint,
	COALESCE(p.purchase_count, 0) AS purchase_count
FROM offers o
LEFT JOIN (
	SELECT offer_id, COUNT(1) AS purchase_count
	FROM jobs
	WHERE status IN ('PAID', 'DELIVERED')
	GROUP BY offer_id
) p ON p.offer_id = o.offer_id
`

func (h *AdminHandler) ListOffers(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	sellerBotID := strings.TrimSpace(r.URL.Query().Get("seller_bot_id"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	filterTags := parseTagFilter(r.URL.Query().Get("tags"))

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
	var cursorCreatedAt *time.Time
	var cursorOfferID *string
	if cursorValue != "" {
		createdAt, offerID, err := decodeCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursorCreatedAt = &createdAt
		cursorOfferID = &offerID
	}

	where := "WHERE 1=1"
	args := make([]any, 0, 16)
	if sellerBotID != "" {
		where += " AND o.seller_bot_id = ?"
		args = append(args, sellerBotID)
	}
	if status != "" {
		where += " AND o.status = ?"
		args = append(args, status)
	}
	if q != "" {
		where += " AND (o.offer_id LIKE ? OR o.title LIKE ? OR o.description LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	if len(filterTags) > 0 {
		where += fmt.Sprintf(" AND o.offer_id IN (SELECT offer_id FROM offer_tags WHERE tag IN (%s) GROUP BY offer_id HAVING COUNT(DISTINCT tag) = %d)", placeholders(len(filterTags)), len(filterTags))
		for _, tag := range filterTags {
			args = append(args, tag)
		}
	}
	if cursorCreatedAt != nil && cursorOfferID != nil {
		where += " AND (o.created_at < ? OR (o.created_at = ? AND o.offer_id < ?))"
		args = append(args, cursorCreatedAt.UTC(), cursorCreatedAt.UTC(), *cursorOfferID)
	}
	args = append(args, limit+1)

	rows, err := h.Store.DB.QueryContext(r.Context(), selectAdminOffersQuery+where+`
ORDER BY o.created_at DESC, o.offer_id DESC
LIMIT ?`, args...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer list failed")
		return
	}
	defer rows.Close()

	resp := adminOfferListResponse{Offers: make([]adminOfferRow, 0, limit)}
	expiredOffers := make([]string, 0)
	now := h.now()
	for rows.Next() {
		var offer sqlc.Offer
		var purchaseCount int64
		if err := rows.Scan(
			&offer.OfferID,
			&offer.SellerBotID,
			&offer.Title,
			&offer.Description,
			&offer.TagsJson,
			&offer.PriceRaw,
			&offer.TurnaroundSeconds,
			&offer.CreatedAt,
			&offer.ExpiresAt,
			&offer.Status,
			&offer.CancelledAt,
			&offer.RequestSchemaHint,
			&purchaseCount,
		); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "offer list failed")
			return
		}

		// Best-effort expiry persistence to keep the DB canonical.
		if offer.Status == "ACTIVE" || offer.Status == "PAUSED" {
			if offer.ExpiresAt.Valid && now.After(offer.ExpiresAt.Time) {
				offer.Status = "EXPIRED"
				expiredOffers = append(expiredOffers, offer.OfferID)
			}
		}

		tags := parseOfferTagsJSON(offer.TagsJson)
		row := adminOfferRow{
			OfferID:           offer.OfferID,
			SellerBotID:       offer.SellerBotID,
			Title:             offer.Title,
			Description:       offer.Description,
			Tags:              tags,
			PriceRaw:          offer.PriceRaw,
			TurnaroundSeconds: offer.TurnaroundSeconds,
			CreatedAt:         offer.CreatedAt.UTC().Format(time.RFC3339Nano),
			Status:            offer.Status,
			PurchaseCount:     purchaseCount,
		}
		if offer.ExpiresAt.Valid {
			row.ExpiresAt = offer.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if offer.CancelledAt.Valid {
			row.CancelledAt = offer.CancelledAt.Time.UTC().Format(time.RFC3339Nano)
		}
		resp.Offers = append(resp.Offers, row)
	}
	if err := rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer list failed")
		return
	}

	if len(expiredOffers) > 0 {
		for _, offerID := range expiredOffers {
			_ = h.Store.UpdateOfferExpire(r.Context(), offerID)
		}
	}

	hasMore := len(resp.Offers) > limit
	if hasMore {
		resp.Offers = resp.Offers[:limit]
	}
	if hasMore && len(resp.Offers) > 0 {
		last := resp.Offers[len(resp.Offers)-1]
		createdAt, err := parseTime(last.CreatedAt)
		if err == nil {
			resp.NextCursor = encodeCursor(createdAt, last.OfferID)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type adminOfferDetailResponse struct {
	OfferID           string   `json:"offer_id"`
	SellerBotID       string   `json:"seller_bot_id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Tags              []string `json:"tags"`
	PriceRaw          string   `json:"price_raw"`
	TurnaroundSeconds int64    `json:"turnaround_seconds"`
	CreatedAt         string   `json:"created_at"`
	ExpiresAt         string   `json:"expires_at,omitempty"`
	Status            string   `json:"status"`
	CancelledAt       string   `json:"cancelled_at,omitempty"`
	RequestSchemaHint string   `json:"request_schema_hint,omitempty"`
	PurchaseCount     int64    `json:"purchase_count"`
}

func (h *AdminHandler) GetOffer(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	offerID := chi.URLParam(r, "offer_id")
	if offerID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing offer_id")
		return
	}

	offer, err := h.Store.GetOffer(r.Context(), offerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "offer not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "offer lookup failed")
		return
	}

	// Best-effort expiry persistence.
	if offer.Status == "ACTIVE" || offer.Status == "PAUSED" {
		if offer.ExpiresAt.Valid && h.now().After(offer.ExpiresAt.Time) {
			_ = h.Store.UpdateOfferExpire(r.Context(), offerID)
			offer.Status = "EXPIRED"
		}
	}

	tags := parseOfferTagsJSON(offer.TagsJson)
	var purchaseCount int64
	_ = h.Store.DB.QueryRowContext(r.Context(), `SELECT COUNT(1) FROM jobs WHERE offer_id = ?1 AND status IN ('PAID','DELIVERED')`, offerID).Scan(&purchaseCount)

	resp := adminOfferDetailResponse{
		OfferID:           offer.OfferID,
		SellerBotID:       offer.SellerBotID,
		Title:             offer.Title,
		Description:       offer.Description,
		Tags:              tags,
		PriceRaw:          offer.PriceRaw,
		TurnaroundSeconds: offer.TurnaroundSeconds,
		CreatedAt:         offer.CreatedAt.UTC().Format(time.RFC3339Nano),
		Status:            offer.Status,
		PurchaseCount:     purchaseCount,
	}
	if offer.ExpiresAt.Valid {
		resp.ExpiresAt = offer.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if offer.CancelledAt.Valid {
		resp.CancelledAt = offer.CancelledAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if offer.RequestSchemaHint.Valid {
		resp.RequestSchemaHint = offer.RequestSchemaHint.String
	}

	writeJSON(w, http.StatusOK, resp)
}

type adminOfferModerateResponse struct {
	Offer   adminOfferDetailResponse `json:"offer"`
	AuditID int64                    `json:"audit_id"`
}

func (h *AdminHandler) PauseOffer(w http.ResponseWriter, r *http.Request) {
	h.moderateOfferStatus(w, r, "offer.pause")
}

func (h *AdminHandler) ResumeOffer(w http.ResponseWriter, r *http.Request) {
	h.moderateOfferStatus(w, r, "offer.resume")
}

func (h *AdminHandler) CancelOffer(w http.ResponseWriter, r *http.Request) {
	h.moderateOfferStatus(w, r, "offer.cancel")
}

func (h *AdminHandler) moderateOfferStatus(w http.ResponseWriter, r *http.Request, actionName string) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	offerID := chi.URLParam(r, "offer_id")
	if offerID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing offer_id")
		return
	}

	action, err := decodeAdminAction(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	now := h.now()

	tx, err := h.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer update failed")
		return
	}
	defer func() { _ = tx.Rollback() }()
	qtx := sqlc.New(tx)

	beforeOffer, err := qtx.GetOffer(ctx, offerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "offer not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "offer lookup failed")
		return
	}

	// Best-effort expiry before moderation.
	if (beforeOffer.Status == "ACTIVE" || beforeOffer.Status == "PAUSED") && beforeOffer.ExpiresAt.Valid && now.After(beforeOffer.ExpiresAt.Time) {
		_ = qtx.UpdateOfferExpire(ctx, offerID)
		beforeOffer.Status = "EXPIRED"
	}

	switch actionName {
	case "offer.pause":
		if beforeOffer.Status == "CANCELLED" {
			writeJSONError(w, http.StatusConflict, "offer cancelled")
			return
		}
		if beforeOffer.Status == "EXPIRED" {
			writeJSONError(w, http.StatusConflict, "offer expired")
			return
		}
		if beforeOffer.Status == "ACTIVE" {
			if err := qtx.UpdateOfferPause(ctx, offerID); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "offer pause failed")
				return
			}
		}
	case "offer.resume":
		if beforeOffer.Status == "CANCELLED" {
			writeJSONError(w, http.StatusConflict, "offer cancelled")
			return
		}
		if beforeOffer.Status == "EXPIRED" {
			writeJSONError(w, http.StatusConflict, "offer expired")
			return
		}
		if beforeOffer.Status == "PAUSED" {
			if err := qtx.UpdateOfferResume(ctx, offerID); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "offer resume failed")
				return
			}
		}
	case "offer.cancel":
		if beforeOffer.Status == "EXPIRED" {
			writeJSONError(w, http.StatusConflict, "offer expired")
			return
		}
		if beforeOffer.Status == "ACTIVE" || beforeOffer.Status == "PAUSED" {
			if err := qtx.UpdateOfferCancel(ctx, sqlc.UpdateOfferCancelParams{
				OfferID:     offerID,
				CancelledAt: sql.NullTime{Time: now, Valid: true},
			}); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "offer cancel failed")
				return
			}
		}
	default:
		writeJSONError(w, http.StatusBadRequest, "invalid action")
		return
	}

	updated, err := qtx.GetOffer(ctx, offerID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer lookup failed")
		return
	}

	auditID, err := h.Store.InsertAdminAuditTx(ctx, tx, store.AdminAuditEntry{
		Action:           actionName,
		TargetType:       "offer",
		TargetID:         offerID,
		Reason:           action.Reason,
		Note:             action.Note,
		RequestID:        middleware.GetReqID(ctx),
		TokenFingerprint: adminTokenFingerprint(ctx),
		RemoteAddr:       r.RemoteAddr,
		UserAgent:        r.UserAgent(),
		Before:           offerAuditSnapshot(beforeOffer),
		After:            offerAuditSnapshot(updated),
		CreatedAt:        now,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "audit failed")
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer update failed")
		return
	}

	detail := adminOfferDetailResponseFromOffer(updated)
	writeJSON(w, http.StatusOK, adminOfferModerateResponse{Offer: detail, AuditID: auditID})
}

type adminJobRow struct {
	JobID             string `json:"job_id"`
	OfferID           string `json:"offer_id"`
	BuyerBotID        string `json:"buyer_bot_id"`
	SellerBotID       string `json:"seller_bot_id"`
	Status            string `json:"status"`
	PriceRaw          string `json:"price_raw"`
	TurnaroundSeconds int64  `json:"turnaround_seconds"`
	CreatedAt         string `json:"created_at"`
	JobExpiresAt      string `json:"job_expires_at"`

	ChargeID        string `json:"charge_id,omitempty"`
	ChargeAddress   string `json:"charge_address,omitempty"`
	ChargeAmountRaw string `json:"charge_amount_raw,omitempty"`
	ChargeExpiresAt string `json:"charge_expires_at,omitempty"`

	PaidAt      string `json:"paid_at,omitempty"`
	DeliveredAt string `json:"delivered_at,omitempty"`
	CancelledAt string `json:"cancelled_at,omitempty"`
	ExpiredAt   string `json:"expired_at,omitempty"`
}

type adminJobListResponse struct {
	Jobs       []adminJobRow `json:"jobs"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

func (h *AdminHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	offerID := strings.TrimSpace(r.URL.Query().Get("offer_id"))
	buyerBotID := strings.TrimSpace(r.URL.Query().Get("buyer_bot_id"))
	sellerBotID := strings.TrimSpace(r.URL.Query().Get("seller_bot_id"))
	jobIDQuery := strings.TrimSpace(r.URL.Query().Get("q"))

	statuses, _, err := parseStatusFilter(r.URL.Query()["status"])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var createdSince *time.Time
	if value := strings.TrimSpace(r.URL.Query().Get("created_since")); value != "" {
		parsed, err := parseTime(value)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid created_since")
			return
		}
		createdSince = &parsed
	}

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
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

	where := "WHERE 1=1"
	args := make([]any, 0, 24)
	if offerID != "" {
		where += " AND offer_id = ?"
		args = append(args, offerID)
	}
	if buyerBotID != "" {
		where += " AND buyer_bot_id = ?"
		args = append(args, buyerBotID)
	}
	if sellerBotID != "" {
		where += " AND seller_bot_id = ?"
		args = append(args, sellerBotID)
	}
	if jobIDQuery != "" {
		where += " AND job_id LIKE ?"
		args = append(args, "%"+jobIDQuery+"%")
	}
	if createdSince != nil {
		where += " AND created_at >= ?"
		args = append(args, createdSince.UTC())
	}
	if len(statuses) > 0 {
		where += fmt.Sprintf(" AND status IN (%s)", placeholders(len(statuses)))
		for _, status := range statuses {
			args = append(args, status)
		}
	}
	if cursorCreatedAt != nil && cursorJobID != nil {
		where += " AND (created_at < ? OR (created_at = ? AND job_id < ?))"
		args = append(args, cursorCreatedAt.UTC(), cursorCreatedAt.UTC(), *cursorJobID)
	}
	args = append(args, limit+1)

	rows, err := h.Store.DB.QueryContext(r.Context(), `
SELECT job_id, offer_id, buyer_bot_id, seller_bot_id, status, price_raw, turnaround_seconds, created_at, job_expires_at,
	request_payload_id,
	charge_id, charge_address, charge_amount_raw, charge_expires_at, charge_sig_ed25519,
	paid_at, delivered_at, cancelled_at, expired_at,
	payment_verifier, payment_block_hash, payment_observed_at, amount_raw_received
FROM jobs
`+where+`
ORDER BY created_at DESC, job_id DESC
LIMIT ?`, args...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job list failed")
		return
	}
	defer rows.Close()

	resp := adminJobListResponse{Jobs: make([]adminJobRow, 0, limit)}
	jobHandler := &JobHandler{Store: h.Store, StreamHub: h.StreamHub, Clock: h.Clock}
	for rows.Next() {
		var job sqlc.Job
		if err := rows.Scan(
			&job.JobID,
			&job.OfferID,
			&job.BuyerBotID,
			&job.SellerBotID,
			&job.Status,
			&job.PriceRaw,
			&job.TurnaroundSeconds,
			&job.CreatedAt,
			&job.JobExpiresAt,
			&job.RequestPayloadID,
			&job.ChargeID,
			&job.ChargeAddress,
			&job.ChargeAmountRaw,
			&job.ChargeExpiresAt,
			&job.ChargeSigEd25519,
			&job.PaidAt,
			&job.DeliveredAt,
			&job.CancelledAt,
			&job.ExpiredAt,
			&job.PaymentVerifier,
			&job.PaymentBlockHash,
			&job.PaymentObservedAt,
			&job.AmountRawReceived,
		); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "job list failed")
			return
		}

		// Apply expiry using the same logic as the public job endpoints (best-effort).
		updated, err := jobHandler.applyExpiry(r.Context(), job)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
			return
		}

		resp.Jobs = append(resp.Jobs, adminJobRowFromJob(updated))
	}
	if err := rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job list failed")
		return
	}

	hasMore := len(resp.Jobs) > limit
	if hasMore {
		resp.Jobs = resp.Jobs[:limit]
	}
	if hasMore && len(resp.Jobs) > 0 {
		last := resp.Jobs[len(resp.Jobs)-1]
		createdAt, err := parseTime(last.CreatedAt)
		if err == nil {
			resp.NextCursor = encodeCursor(createdAt, last.JobID)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type adminJobDetailResponse struct {
	Job sqlc.Job `json:"-"`

	JobID             string `json:"job_id"`
	OfferID           string `json:"offer_id"`
	BuyerBotID        string `json:"buyer_bot_id"`
	SellerBotID       string `json:"seller_bot_id"`
	Status            string `json:"status"`
	PriceRaw          string `json:"price_raw"`
	TurnaroundSeconds int64  `json:"turnaround_seconds"`
	CreatedAt         string `json:"created_at"`
	JobExpiresAt      string `json:"job_expires_at"`
	RequestPayloadID  string `json:"request_payload_id"`

	ChargeID        string `json:"charge_id,omitempty"`
	ChargeAddress   string `json:"charge_address,omitempty"`
	ChargeAmountRaw string `json:"charge_amount_raw,omitempty"`
	ChargeExpiresAt string `json:"charge_expires_at,omitempty"`
	ChargeSig       string `json:"charge_sig_ed25519,omitempty"`

	PaidAt      string `json:"paid_at,omitempty"`
	DeliveredAt string `json:"delivered_at,omitempty"`
	CancelledAt string `json:"cancelled_at,omitempty"`
	ExpiredAt   string `json:"expired_at,omitempty"`

	PaymentVerifier   string `json:"payment_verifier,omitempty"`
	PaymentBlockHash  string `json:"payment_block_hash,omitempty"`
	PaymentObservedAt string `json:"payment_observed_at,omitempty"`
	AmountRawReceived string `json:"amount_raw_received,omitempty"`
	PayloadsPending   int64  `json:"payloads_pending"`
	PayloadsTotal     int64  `json:"payloads_total"`
}

func (h *AdminHandler) GetJob(w http.ResponseWriter, r *http.Request) {
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

	jobHandler := &JobHandler{Store: h.Store, StreamHub: h.StreamHub, Clock: h.Clock}
	job, err = jobHandler.applyExpiry(r.Context(), job)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job expiry failed")
		return
	}

	resp := adminJobDetailResponse{
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
	}
	if job.ChargeID.Valid {
		resp.ChargeID = job.ChargeID.String
	}
	if job.ChargeAddress.Valid {
		resp.ChargeAddress = job.ChargeAddress.String
	}
	if job.ChargeAmountRaw.Valid {
		resp.ChargeAmountRaw = job.ChargeAmountRaw.String
	}
	if job.ChargeExpiresAt.Valid {
		resp.ChargeExpiresAt = job.ChargeExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.ChargeSigEd25519.Valid {
		resp.ChargeSig = job.ChargeSigEd25519.String
	}
	if job.PaidAt.Valid {
		resp.PaidAt = job.PaidAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.DeliveredAt.Valid {
		resp.DeliveredAt = job.DeliveredAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.CancelledAt.Valid {
		resp.CancelledAt = job.CancelledAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.ExpiredAt.Valid {
		resp.ExpiredAt = job.ExpiredAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.PaymentVerifier.Valid {
		resp.PaymentVerifier = job.PaymentVerifier.String
	}
	if job.PaymentBlockHash.Valid {
		resp.PaymentBlockHash = job.PaymentBlockHash.String
	}
	if job.PaymentObservedAt.Valid {
		resp.PaymentObservedAt = job.PaymentObservedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.AmountRawReceived.Valid {
		resp.AmountRawReceived = job.AmountRawReceived.String
	}

	_ = h.Store.DB.QueryRowContext(r.Context(), `SELECT COUNT(1) FROM payloads WHERE job_id = ?1`, jobID).Scan(&resp.PayloadsTotal)
	_ = h.Store.DB.QueryRowContext(r.Context(), `SELECT COUNT(1) FROM payloads WHERE job_id = ?1 AND fetched_at IS NULL`, jobID).Scan(&resp.PayloadsPending)

	writeJSON(w, http.StatusOK, resp)
}

type adminJobModerateResponse struct {
	Job     adminJobRow `json:"job"`
	AuditID int64       `json:"audit_id"`
}

func (h *AdminHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	h.moderateJob(w, r, "job.cancel")
}

func (h *AdminHandler) ExpireJob(w http.ResponseWriter, r *http.Request) {
	h.moderateJob(w, r, "job.expire")
}

func (h *AdminHandler) moderateJob(w http.ResponseWriter, r *http.Request, actionName string) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing job_id")
		return
	}

	action, err := decodeAdminAction(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	now := h.now()

	tx, err := h.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job update failed")
		return
	}
	defer func() { _ = tx.Rollback() }()
	qtx := sqlc.New(tx)

	beforeJob, err := qtx.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}

	previousStatus := beforeJob.Status

	switch actionName {
	case "job.cancel":
		switch beforeJob.Status {
		case string(domain.JobCancelled):
			// idempotent
		case string(domain.JobExpired), string(domain.JobDelivered):
			writeJSONError(w, http.StatusConflict, "job not mutable")
			return
		case string(domain.JobPaid):
			writeJSONError(w, http.StatusConflict, "job not cancellable")
			return
		default:
			// Allow cancellation of REQUESTED and CHARGE_CREATED.
			if beforeJob.Status != string(domain.JobRequested) && beforeJob.Status != string(domain.JobChargeCreated) {
				writeJSONError(w, http.StatusConflict, "job not cancellable")
				return
			}
			if _, err := tx.ExecContext(ctx, `UPDATE jobs SET status = 'CANCELLED', cancelled_at = ?1 WHERE job_id = ?2 AND status IN ('REQUESTED', 'CHARGE_CREATED')`, sql.NullTime{Time: now, Valid: true}, jobID); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "job cancel failed")
				return
			}
		}
	case "job.expire":
		switch beforeJob.Status {
		case string(domain.JobExpired):
			// idempotent
		case string(domain.JobCancelled), string(domain.JobDelivered):
			writeJSONError(w, http.StatusConflict, "job not mutable")
			return
		default:
			if err := qtx.UpdateJobExpire(ctx, sqlc.UpdateJobExpireParams{JobID: jobID, ExpiredAt: sql.NullTime{Time: now, Valid: true}}); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "job expire failed")
				return
			}
		}
	default:
		writeJSONError(w, http.StatusBadRequest, "invalid action")
		return
	}

	updated, err := qtx.GetJob(ctx, jobID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job lookup failed")
		return
	}

	// Emit events for state transitions.
	if actionName == "job.cancel" && updated.Status == string(domain.JobCancelled) && !beforeJob.CancelledAt.Valid {
		payload := map[string]any{
			"job_id":       updated.JobID,
			"cancelled_at": updated.CancelledAt.Time.UTC().Format(time.RFC3339Nano),
			"cancelled_by": "admin",
			"reason":       action.Reason,
		}
		recipients := []string{updated.BuyerBotID, updated.SellerBotID}
		if updated.BuyerBotID == updated.SellerBotID {
			recipients = []string{updated.BuyerBotID}
		}
		for i, recipient := range recipients {
			emitJobStream := i == 0
			if err := emitEventTx(ctx, qtx, h.StreamHub, recipient, jobCancelledEventType, payload, emitJobStream); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "event create failed")
				return
			}
		}
	}
	if actionName == "job.expire" && updated.Status == string(domain.JobExpired) && !beforeJob.ExpiredAt.Valid {
		payload := map[string]any{
			"job_id":          updated.JobID,
			"expired_at":      updated.ExpiredAt.Time.UTC().Format(time.RFC3339Nano),
			"previous_status": previousStatus,
			"expired_by":      "admin",
			"reason":          action.Reason,
		}
		recipients := []string{updated.BuyerBotID, updated.SellerBotID}
		if updated.BuyerBotID == updated.SellerBotID {
			recipients = []string{updated.BuyerBotID}
		}
		for i, recipient := range recipients {
			emitJobStream := i == 0
			if err := emitEventTx(ctx, qtx, h.StreamHub, recipient, jobExpiredEventType, payload, emitJobStream); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "event create failed")
				return
			}
		}
	}

	auditID, err := h.Store.InsertAdminAuditTx(ctx, tx, store.AdminAuditEntry{
		Action:           actionName,
		TargetType:       "job",
		TargetID:         jobID,
		Reason:           action.Reason,
		Note:             action.Note,
		RequestID:        middleware.GetReqID(ctx),
		TokenFingerprint: adminTokenFingerprint(ctx),
		RemoteAddr:       r.RemoteAddr,
		UserAgent:        r.UserAgent(),
		Before:           jobAuditSnapshot(beforeJob),
		After:            jobAuditSnapshot(updated),
		CreatedAt:        now,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "audit failed")
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "job update failed")
		return
	}

	writeJSON(w, http.StatusOK, adminJobModerateResponse{
		Job:     adminJobRowFromJob(updated),
		AuditID: auditID,
	})
}

type adminPayloadRow struct {
	PayloadID          string `json:"payload_id"`
	JobID              string `json:"job_id"`
	SenderBotID        string `json:"sender_bot_id"`
	RecipientBotID     string `json:"recipient_bot_id"`
	PayloadKind        string `json:"payload_kind"`
	CreatedAt          string `json:"created_at"`
	FetchedAt          string `json:"fetched_at,omitempty"`
	CiphertextB64Bytes int64  `json:"ciphertext_b64_bytes"`
}

type adminPayloadListResponse struct {
	Payloads   []adminPayloadRow `json:"payloads"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

func (h *AdminHandler) ListPayloads(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	status := strings.TrimSpace(r.URL.Query().Get("status")) // unfetched|fetched|all
	if status == "" {
		status = "unfetched"
	}
	if status != "unfetched" && status != "fetched" && status != "all" {
		writeJSONError(w, http.StatusBadRequest, "invalid status")
		return
	}

	jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))
	recipientBotID := strings.TrimSpace(r.URL.Query().Get("recipient_bot_id"))
	senderBotID := strings.TrimSpace(r.URL.Query().Get("sender_bot_id"))

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
	var cursorCreatedAt *time.Time
	var cursorPayloadID *string
	if cursorValue != "" {
		createdAt, payloadID, err := decodeCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursorCreatedAt = &createdAt
		cursorPayloadID = &payloadID
	}

	where := "WHERE 1=1"
	args := make([]any, 0, 20)
	if status == "unfetched" {
		where += " AND fetched_at IS NULL"
	} else if status == "fetched" {
		where += " AND fetched_at IS NOT NULL"
	}
	if jobID != "" {
		where += " AND job_id = ?"
		args = append(args, jobID)
	}
	if recipientBotID != "" {
		where += " AND recipient_bot_id = ?"
		args = append(args, recipientBotID)
	}
	if senderBotID != "" {
		where += " AND sender_bot_id = ?"
		args = append(args, senderBotID)
	}
	if cursorCreatedAt != nil && cursorPayloadID != nil {
		where += " AND (created_at < ? OR (created_at = ? AND payload_id < ?))"
		args = append(args, cursorCreatedAt.UTC(), cursorCreatedAt.UTC(), *cursorPayloadID)
	}
	args = append(args, limit+1)

	rows, err := h.Store.DB.QueryContext(r.Context(), `
SELECT payload_id, job_id, sender_bot_id, recipient_bot_id, payload_kind, created_at, fetched_at, LENGTH(ciphertext_b64)
FROM payloads
`+where+`
ORDER BY created_at DESC, payload_id DESC
LIMIT ?`, args...)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "payload list failed")
		return
	}
	defer rows.Close()

	resp := adminPayloadListResponse{Payloads: make([]adminPayloadRow, 0, limit)}
	for rows.Next() {
		var row adminPayloadRow
		var createdAt time.Time
		var fetchedAt sql.NullTime
		if err := rows.Scan(
			&row.PayloadID,
			&row.JobID,
			&row.SenderBotID,
			&row.RecipientBotID,
			&row.PayloadKind,
			&createdAt,
			&fetchedAt,
			&row.CiphertextB64Bytes,
		); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "payload list failed")
			return
		}
		row.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		if fetchedAt.Valid {
			row.FetchedAt = fetchedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		resp.Payloads = append(resp.Payloads, row)
	}
	if err := rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "payload list failed")
		return
	}

	hasMore := len(resp.Payloads) > limit
	if hasMore {
		resp.Payloads = resp.Payloads[:limit]
	}
	if hasMore && len(resp.Payloads) > 0 {
		last := resp.Payloads[len(resp.Payloads)-1]
		createdAt, err := parseTime(last.CreatedAt)
		if err == nil {
			resp.NextCursor = encodeCursor(createdAt, last.PayloadID)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type adminEventRow struct {
	EventID        int64           `json:"event_id"`
	RecipientBotID string          `json:"recipient_bot_id"`
	EventType      string          `json:"event_type"`
	Data           json.RawMessage `json:"data"`
	CreatedAt      string          `json:"created_at"`
}

type adminEventListResponse struct {
	Events []adminEventRow `json:"events"`
}

func (h *AdminHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil || h.Store.DB == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	recipientBotID := strings.TrimSpace(r.URL.Query().Get("recipient_bot_id"))
	sinceRaw := strings.TrimSpace(r.URL.Query().Get("since_event_id"))
	sinceID, err := parseSinceEventID(sinceRaw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid since_event_id")
		return
	}

	types := parseEventTypes(r.URL.Query().Get("types"))

	var rows *sql.Rows
	if recipientBotID != "" && len(types) > 0 {
		typeList := make([]string, 0, len(types))
		for t := range types {
			typeList = append(typeList, t)
		}
		// We can reuse the sqlc query if needed, but this admin path is intentionally simple.
		query := fmt.Sprintf(`
SELECT event_id, recipient_bot_id, event_type, data_json, created_at
FROM events
WHERE recipient_bot_id = ?1
	AND event_id > ?2
	AND event_type IN (%s)
ORDER BY event_id ASC
LIMIT ?%d`, placeholders(len(typeList)), 3+len(typeList))
		args := []any{recipientBotID, sinceID}
		for _, t := range typeList {
			args = append(args, t)
		}
		args = append(args, limit)
		rows, err = h.Store.DB.QueryContext(r.Context(), query, args...)
	} else if recipientBotID != "" {
		rows, err = h.Store.DB.QueryContext(r.Context(), `
SELECT event_id, recipient_bot_id, event_type, data_json, created_at
FROM events
WHERE recipient_bot_id = ?1
	AND event_id > ?2
ORDER BY event_id ASC
LIMIT ?3`, recipientBotID, sinceID, limit)
	} else {
		rows, err = h.Store.DB.QueryContext(r.Context(), `
SELECT event_id, recipient_bot_id, event_type, data_json, created_at
FROM events
WHERE event_id > ?1
ORDER BY event_id ASC
LIMIT ?2`, sinceID, limit)
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "event list failed")
		return
	}
	defer rows.Close()

	resp := adminEventListResponse{Events: make([]adminEventRow, 0, limit)}
	for rows.Next() {
		var row adminEventRow
		var dataJSON string
		var createdAt time.Time
		if err := rows.Scan(&row.EventID, &row.RecipientBotID, &row.EventType, &dataJSON, &createdAt); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "event list failed")
			return
		}
		row.Data = json.RawMessage(dataJSON)
		row.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		resp.Events = append(resp.Events, row)
	}
	if err := rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "event list failed")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

type adminAuditRow struct {
	ID         int64  `json:"id"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Note       string `json:"note,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	TokenFP    string `json:"token_fingerprint,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	BeforeJSON string `json:"before_json,omitempty"`
	AfterJSON  string `json:"after_json,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type adminAuditListResponse struct {
	Entries    []adminAuditRow `json:"entries"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

func (h *AdminHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	targetType := strings.TrimSpace(r.URL.Query().Get("target_type"))
	targetID := strings.TrimSpace(r.URL.Query().Get("target_id"))

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
	var cursor *store.AdminAuditCursor
	if cursorValue != "" {
		createdAt, idStr, err := decodeCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursor = &store.AdminAuditCursor{CreatedAt: createdAt, ID: id}
	}

	rows, next, err := h.Store.ListAdminAudit(r.Context(), limit, cursor, targetType, targetID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "audit list failed")
		return
	}

	resp := adminAuditListResponse{Entries: make([]adminAuditRow, 0, len(rows))}
	for _, row := range rows {
		resp.Entries = append(resp.Entries, adminAuditRow{
			ID:         row.ID,
			Action:     row.Action,
			TargetType: row.TargetType,
			TargetID:   row.TargetID,
			Reason:     row.Reason,
			Note:       row.Note,
			RequestID:  row.RequestID,
			TokenFP:    row.TokenFingerprint,
			RemoteAddr: row.RemoteAddr,
			UserAgent:  row.UserAgent,
			BeforeJSON: row.BeforeJSON,
			AfterJSON:  row.AfterJSON,
			CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	if next != nil {
		resp.NextCursor = encodeCursor(next.CreatedAt, strconv.FormatInt(next.ID, 10))
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) now() time.Time {
	if h == nil || h.Clock == nil {
		return time.Now().UTC()
	}
	return h.Clock().UTC()
}

func decodeAdminAction(r *http.Request) (adminActionRequest, error) {
	if r == nil {
		return adminActionRequest{}, errors.New("invalid request")
	}
	var payload adminActionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return adminActionRequest{}, errors.New("invalid json")
	}
	payload.Reason = strings.TrimSpace(payload.Reason)
	payload.Note = strings.TrimSpace(payload.Note)
	if payload.Reason == "" {
		return adminActionRequest{}, errors.New("missing reason")
	}
	if len(payload.Reason) > 5000 {
		return adminActionRequest{}, errors.New("reason too long")
	}
	if len(payload.Note) > 20000 {
		return adminActionRequest{}, errors.New("note too long")
	}
	return payload, nil
}

func parseOfferTagsJSON(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "?")
	}
	return strings.Join(out, ",")
}

func parseBoolPtr(raw string) (*bool, error) {
	if raw == "" {
		return nil, nil
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func adminOfferDetailResponseFromOffer(offer sqlc.Offer) adminOfferDetailResponse {
	tags := parseOfferTagsJSON(offer.TagsJson)
	resp := adminOfferDetailResponse{
		OfferID:           offer.OfferID,
		SellerBotID:       offer.SellerBotID,
		Title:             offer.Title,
		Description:       offer.Description,
		Tags:              tags,
		PriceRaw:          offer.PriceRaw,
		TurnaroundSeconds: offer.TurnaroundSeconds,
		CreatedAt:         offer.CreatedAt.UTC().Format(time.RFC3339Nano),
		Status:            offer.Status,
	}
	if offer.ExpiresAt.Valid {
		resp.ExpiresAt = offer.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if offer.CancelledAt.Valid {
		resp.CancelledAt = offer.CancelledAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if offer.RequestSchemaHint.Valid {
		resp.RequestSchemaHint = offer.RequestSchemaHint.String
	}
	return resp
}

func offerAuditSnapshot(offer sqlc.Offer) map[string]any {
	out := map[string]any{
		"offer_id": offer.OfferID,
		"status":   offer.Status,
	}
	if offer.CancelledAt.Valid {
		out["cancelled_at"] = offer.CancelledAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if offer.ExpiresAt.Valid {
		out["expires_at"] = offer.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	out["seller_bot_id"] = offer.SellerBotID
	return out
}

func jobAuditSnapshot(job sqlc.Job) map[string]any {
	out := map[string]any{
		"job_id":        job.JobID,
		"status":        job.Status,
		"buyer_bot_id":  job.BuyerBotID,
		"seller_bot_id": job.SellerBotID,
		"offer_id":      job.OfferID,
	}
	if job.CancelledAt.Valid {
		out["cancelled_at"] = job.CancelledAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.ExpiredAt.Valid {
		out["expired_at"] = job.ExpiredAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.PaidAt.Valid {
		out["paid_at"] = job.PaidAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.DeliveredAt.Valid {
		out["delivered_at"] = job.DeliveredAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func adminJobRowFromJob(job sqlc.Job) adminJobRow {
	row := adminJobRow{
		JobID:             job.JobID,
		OfferID:           job.OfferID,
		BuyerBotID:        job.BuyerBotID,
		SellerBotID:       job.SellerBotID,
		Status:            job.Status,
		PriceRaw:          job.PriceRaw,
		TurnaroundSeconds: job.TurnaroundSeconds,
		CreatedAt:         job.CreatedAt.UTC().Format(time.RFC3339Nano),
		JobExpiresAt:      job.JobExpiresAt.UTC().Format(time.RFC3339Nano),
	}
	if job.ChargeID.Valid {
		row.ChargeID = job.ChargeID.String
	}
	if job.ChargeAddress.Valid {
		row.ChargeAddress = job.ChargeAddress.String
	}
	if job.ChargeAmountRaw.Valid {
		row.ChargeAmountRaw = job.ChargeAmountRaw.String
	}
	if job.ChargeExpiresAt.Valid {
		row.ChargeExpiresAt = job.ChargeExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.PaidAt.Valid {
		row.PaidAt = job.PaidAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.DeliveredAt.Valid {
		row.DeliveredAt = job.DeliveredAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.CancelledAt.Valid {
		row.CancelledAt = job.CancelledAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if job.ExpiredAt.Valid {
		row.ExpiredAt = job.ExpiredAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return row
}
