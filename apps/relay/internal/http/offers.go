package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nanobazaar/relay/internal/domain"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

const (
	offerDefaultTTL           = 7 * 24 * time.Hour
	offerMaxTTL               = 30 * 24 * time.Hour
	maxRequestSchemaHintBytes = 4096
	offerCancelledEventType   = "offer.cancelled"
)

type OfferHandler struct {
	Store *store.Store
	Clock func() time.Time
}

func NewOfferHandler(store *store.Store) *OfferHandler {
	return &OfferHandler{
		Store: store,
		Clock: time.Now,
	}
}

type offerCreateRequest struct {
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Tags              []string   `json:"tags"`
	PriceRaw          string     `json:"price_raw"`
	TurnaroundSeconds int64      `json:"turnaround_seconds"`
	ExpiresAt         *time.Time `json:"expires_at"`
	RequestSchemaHint string     `json:"request_schema_hint"`
}

type offerResponse struct {
	OfferID           string     `json:"offer_id"`
	SellerBotID       string     `json:"seller_bot_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Tags              []string   `json:"tags"`
	PriceRaw          string     `json:"price_raw"`
	TurnaroundSeconds int64      `json:"turnaround_seconds"`
	CreatedAt         time.Time  `json:"created_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	Status            string     `json:"status"`
	CancelledAt       *time.Time `json:"cancelled_at,omitempty"`
}

type offerListResponse struct {
	Offers     []offerResponse `json:"offers"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type publicOfferResponse struct {
	OfferID       string    `json:"offer_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Tags          []string  `json:"tags"`
	PriceRaw      string    `json:"price_raw"`
	PurchaseCount int       `json:"purchase_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type publicOfferListResponse struct {
	Offers     []publicOfferResponse `json:"offers"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

type offerCursor struct {
	Sort              string     `json:"sort"`
	CreatedAt         time.Time  `json:"created_at"`
	OfferID           string     `json:"offer_id"`
	Score             int        `json:"score,omitempty"`
	PurchaseCount     int        `json:"purchase_count,omitempty"`
	PriceRaw          string     `json:"price_raw,omitempty"`
	TurnaroundSeconds int64      `json:"turnaround_seconds,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	Query             string     `json:"q,omitempty"`
}

type offerEntry struct {
	Offer         sqlc.Offer
	Tags          []string
	Score         int
	Price         *big.Int
	PurchaseCount int
}

func (h *OfferHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	var payload offerCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}

	sellerBotID := r.Header.Get(headerBotID)
	if sellerBotID == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}

	normalized, err := normalizeOfferCreate(payload)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := h.now()
	expiresAt := normalized.ExpiresAt
	if expiresAt == nil {
		def := now.Add(offerDefaultTTL)
		expiresAt = &def
	}
	if expiresAt.Before(now) {
		writeJSONError(w, http.StatusBadRequest, "expires_at in past")
		return
	}
	if expiresAt.Sub(now) > offerMaxTTL {
		writeJSONError(w, http.StatusBadRequest, "expires_at exceeds max ttl")
		return
	}

	offerID, err := newOfferID(now)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer_id generation failed")
		return
	}

	if err := h.insertOffer(r.Context(), offerID, sellerBotID, normalized, now, *expiresAt); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer create failed")
		return
	}

	resp := offerResponse{
		OfferID:           offerID,
		SellerBotID:       sellerBotID,
		Title:             normalized.Title,
		Description:       normalized.Description,
		Tags:              normalized.Tags,
		PriceRaw:          normalized.PriceRaw,
		TurnaroundSeconds: normalized.TurnaroundSeconds,
		CreatedAt:         now,
		ExpiresAt:         expiresAt,
		Status:            string(domain.OfferActive),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *OfferHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	if err := h.expireOfferIfNeeded(r.Context(), &offer); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer expire failed")
		return
	}

	tags, err := h.loadOfferTags(r.Context(), offer)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer tags failed")
		return
	}

	writeJSON(w, http.StatusOK, offerToResponse(offer, tags))
}

func (h *OfferHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != offer.SellerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.expireOfferIfNeeded(r.Context(), &offer); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer expire failed")
		return
	}

	switch offer.Status {
	case string(domain.OfferCancelled):
		tags, err := h.loadOfferTags(r.Context(), offer)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "offer tags failed")
			return
		}
		writeJSON(w, http.StatusOK, offerToResponse(offer, tags))
		return
	case string(domain.OfferExpired):
		writeJSONError(w, http.StatusConflict, "offer expired")
		return
	}

	now := h.now()
	if err := h.Store.UpdateOfferCancel(r.Context(), sqlc.UpdateOfferCancelParams{
		CancelledAt: sql.NullTime{Time: now, Valid: true},
		OfferID:     offerID,
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer cancel failed")
		return
	}

	offer, err = h.Store.GetOffer(r.Context(), offerID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer lookup failed")
		return
	}
	tags, err := h.loadOfferTags(r.Context(), offer)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer tags failed")
		return
	}
	writeJSON(w, http.StatusOK, offerToResponse(offer, tags))
}

func (h *OfferHandler) Pause(w http.ResponseWriter, r *http.Request) {
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

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != offer.SellerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.expireOfferIfNeeded(r.Context(), &offer); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer expire failed")
		return
	}

	switch offer.Status {
	case string(domain.OfferCancelled):
		writeJSONError(w, http.StatusConflict, "offer cancelled")
		return
	case string(domain.OfferExpired):
		writeJSONError(w, http.StatusConflict, "offer expired")
		return
	case string(domain.OfferPaused):
		tags, err := h.loadOfferTags(r.Context(), offer)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "offer tags failed")
			return
		}
		writeJSON(w, http.StatusOK, offerToResponse(offer, tags))
		return
	}

	if err := h.Store.UpdateOfferPause(r.Context(), offer.OfferID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer pause failed")
		return
	}

	offer, err = h.Store.GetOffer(r.Context(), offerID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer lookup failed")
		return
	}
	tags, err := h.loadOfferTags(r.Context(), offer)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer tags failed")
		return
	}
	writeJSON(w, http.StatusOK, offerToResponse(offer, tags))
}

func (h *OfferHandler) Resume(w http.ResponseWriter, r *http.Request) {
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

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != offer.SellerBotID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.expireOfferIfNeeded(r.Context(), &offer); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer expire failed")
		return
	}

	switch offer.Status {
	case string(domain.OfferCancelled):
		writeJSONError(w, http.StatusConflict, "offer cancelled")
		return
	case string(domain.OfferExpired):
		writeJSONError(w, http.StatusConflict, "offer expired")
		return
	case string(domain.OfferActive):
		tags, err := h.loadOfferTags(r.Context(), offer)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "offer tags failed")
			return
		}
		writeJSON(w, http.StatusOK, offerToResponse(offer, tags))
		return
	}

	if err := h.Store.UpdateOfferResume(r.Context(), offer.OfferID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer resume failed")
		return
	}

	offer, err = h.Store.GetOffer(r.Context(), offerID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer lookup failed")
		return
	}
	tags, err := h.loadOfferTags(r.Context(), offer)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer tags failed")
		return
	}
	writeJSON(w, http.StatusOK, offerToResponse(offer, tags))
}

func (h *OfferHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortParam := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sortParam == "" {
		if query != "" {
			sortParam = "relevance"
		} else {
			sortParam = "newest"
		}
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
	var cursor *offerCursor
	if cursorValue != "" {
		decoded, err := decodeOfferCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		if decoded.Sort != "" && decoded.Sort != sortParam {
			writeJSONError(w, http.StatusBadRequest, "cursor sort mismatch")
			return
		}
		if decoded.Query != "" && decoded.Query != query {
			writeJSONError(w, http.StatusBadRequest, "cursor query mismatch")
			return
		}
		cursor = &decoded
	}

	sellerBotID := strings.TrimSpace(r.URL.Query().Get("seller_bot_id"))
	if mineParam := strings.TrimSpace(r.URL.Query().Get("mine")); mineParam != "" {
		mine, err := strconv.ParseBool(mineParam)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid mine flag")
			return
		}
		if mine {
			sellerBotID = r.Header.Get(headerBotID)
		}
	}

	includePaused := false
	if includeParam := strings.TrimSpace(r.URL.Query().Get("include_paused")); includeParam != "" {
		val, err := strconv.ParseBool(includeParam)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid include_paused flag")
			return
		}
		includePaused = val
	}

	filterTags := parseTagFilter(r.URL.Query().Get("tags"))

	entries, err := h.loadOfferEntries(r.Context(), sellerBotID, query, filterTags, includePaused)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer list failed")
		return
	}

	if err := sortOfferEntries(entries, sortParam); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if cursor != nil {
		start := findCursorStart(entries, *cursor, sortParam)
		if start >= len(entries) {
			writeJSON(w, http.StatusOK, offerListResponse{Offers: []offerResponse{}})
			return
		}
		entries = entries[start:]
	}

	nextCursor := ""
	if len(entries) > limit {
		next, err := encodeOfferCursor(cursorFromEntry(entries[limit-1], sortParam, query))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "cursor encode failed")
			return
		}
		nextCursor = next
		entries = entries[:limit]
	}

	respOffers := make([]offerResponse, 0, len(entries))
	for _, entry := range entries {
		respOffers = append(respOffers, offerToResponse(entry.Offer, entry.Tags))
	}

	writeJSON(w, http.StatusOK, offerListResponse{
		Offers:     respOffers,
		NextCursor: nextCursor,
	})
}

func (h *OfferHandler) PublicList(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortParam := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sortParam == "" {
		if query != "" {
			sortParam = "relevance"
		} else {
			sortParam = "newest"
		}
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	cursorValue := strings.TrimSpace(r.URL.Query().Get("cursor"))
	var cursor *offerCursor
	if cursorValue != "" {
		decoded, err := decodeOfferCursor(cursorValue)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		if decoded.Sort != "" && decoded.Sort != sortParam {
			writeJSONError(w, http.StatusBadRequest, "cursor sort mismatch")
			return
		}
		if decoded.Query != "" && decoded.Query != query {
			writeJSONError(w, http.StatusBadRequest, "cursor query mismatch")
			return
		}
		cursor = &decoded
	}

	sellerBotID := strings.TrimSpace(r.URL.Query().Get("seller_bot_id"))
	filterTags := parseTagFilter(r.URL.Query().Get("tags"))

	entries, err := h.loadOfferEntries(r.Context(), sellerBotID, query, filterTags, false)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "offer list failed")
		return
	}

	if err := sortOfferEntries(entries, sortParam); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if cursor != nil {
		start := findCursorStart(entries, *cursor, sortParam)
		if start >= len(entries) {
			writeJSON(w, http.StatusOK, publicOfferListResponse{Offers: []publicOfferResponse{}})
			return
		}
		entries = entries[start:]
	}

	nextCursor := ""
	if len(entries) > limit {
		next, err := encodeOfferCursor(cursorFromEntry(entries[limit-1], sortParam, query))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "cursor encode failed")
			return
		}
		nextCursor = next
		entries = entries[:limit]
	}

	respOffers := make([]publicOfferResponse, 0, len(entries))
	for _, entry := range entries {
		respOffers = append(respOffers, publicOfferToResponse(entry))
	}

	writeJSON(w, http.StatusOK, publicOfferListResponse{
		Offers:     respOffers,
		NextCursor: nextCursor,
	})
}

func (h *OfferHandler) insertOffer(ctx context.Context, offerID, sellerBotID string, payload offerCreateRequest, createdAt time.Time, expiresAt time.Time) error {
	tx, err := h.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	q := h.Store.Queries.WithTx(tx)
	if err := q.CreateOffer(ctx, sqlc.CreateOfferParams{
		OfferID:           offerID,
		SellerBotID:       sellerBotID,
		Title:             payload.Title,
		Description:       payload.Description,
		TagsJson:          mustJSON(payload.Tags),
		PriceRaw:          payload.PriceRaw,
		TurnaroundSeconds: payload.TurnaroundSeconds,
		CreatedAt:         createdAt,
		ExpiresAt:         sql.NullTime{Time: expiresAt, Valid: true},
		Status:            string(domain.OfferActive),
		CancelledAt:       sql.NullTime{},
		RequestSchemaHint: sql.NullString{String: payload.RequestSchemaHint, Valid: payload.RequestSchemaHint != ""},
	}); err != nil {
		return err
	}

	for _, tag := range payload.Tags {
		if err := q.InsertOfferTag(ctx, sqlc.InsertOfferTagParams{
			OfferID: offerID,
			Tag:     tag,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (h *OfferHandler) loadOfferEntries(ctx context.Context, sellerBotID, query string, filterTags []string, includePaused bool) ([]offerEntry, error) {
	if query != "" {
		return h.loadOfferEntriesSearch(ctx, sellerBotID, query, filterTags, includePaused)
	}
	rows, err := h.Store.DB.QueryContext(ctx, selectOffersQuery, sellerBotID, includePausedFlag(includePaused))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]offerEntry, 0)
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
			return nil, err
		}

		if err := h.expireOfferIfNeeded(ctx, &offer); err != nil {
			return nil, err
		}
		if !offerStatusVisible(offer.Status, includePaused) {
			continue
		}

		tags, err := h.loadOfferTags(ctx, offer)
		if err != nil {
			return nil, err
		}

		if len(filterTags) > 0 && !offerHasTags(tags, filterTags) {
			continue
		}

		entries = append(entries, offerEntry{
			Offer:         offer,
			Tags:          tags,
			Score:         0,
			Price:         parsePrice(offer.PriceRaw),
			PurchaseCount: int(purchaseCount),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *OfferHandler) loadOfferEntriesSearch(ctx context.Context, sellerBotID, query string, filterTags []string, includePaused bool) ([]offerEntry, error) {
	rows, err := h.Store.DB.QueryContext(ctx, selectOffersSearchQuery, query, sellerBotID, includePausedFlag(includePaused))
	if err != nil {
		if isFTSUnavailable(err) {
			return h.loadOfferEntriesSearchFallback(ctx, sellerBotID, query, filterTags, includePaused)
		}
		return nil, err
	}
	defer rows.Close()

	entries := make([]offerEntry, 0)
	for rows.Next() {
		var offer sqlc.Offer
		var purchaseCount int64
		var score int64
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
			&score,
		); err != nil {
			return nil, err
		}

		if err := h.expireOfferIfNeeded(ctx, &offer); err != nil {
			return nil, err
		}
		if !offerStatusVisible(offer.Status, includePaused) {
			continue
		}

		tags, err := h.loadOfferTags(ctx, offer)
		if err != nil {
			return nil, err
		}

		if len(filterTags) > 0 && !offerHasTags(tags, filterTags) {
			continue
		}

		entries = append(entries, offerEntry{
			Offer:         offer,
			Tags:          tags,
			Score:         int(score),
			Price:         parsePrice(offer.PriceRaw),
			PurchaseCount: int(purchaseCount),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *OfferHandler) loadOfferEntriesSearchFallback(ctx context.Context, sellerBotID, query string, filterTags []string, includePaused bool) ([]offerEntry, error) {
	rows, err := h.Store.DB.QueryContext(ctx, selectOffersQuery, sellerBotID, includePausedFlag(includePaused))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]offerEntry, 0)
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
			return nil, err
		}

		if err := h.expireOfferIfNeeded(ctx, &offer); err != nil {
			return nil, err
		}
		if !offerStatusVisible(offer.Status, includePaused) {
			continue
		}

		tags, err := h.loadOfferTags(ctx, offer)
		if err != nil {
			return nil, err
		}

		if len(filterTags) > 0 && !offerHasTags(tags, filterTags) {
			continue
		}

		score := scoreOffer(offer, tags, query)
		if score == 0 {
			continue
		}

		entries = append(entries, offerEntry{
			Offer:         offer,
			Tags:          tags,
			Score:         score,
			Price:         parsePrice(offer.PriceRaw),
			PurchaseCount: int(purchaseCount),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *OfferHandler) loadOfferTags(ctx context.Context, offer sqlc.Offer) ([]string, error) {
	if offer.TagsJson != "" {
		var tags []string
		if err := json.Unmarshal([]byte(offer.TagsJson), &tags); err == nil {
			return tags, nil
		}
	}
	return h.Store.ListOfferTags(ctx, offer.OfferID)
}

func (h *OfferHandler) expireOfferIfNeeded(ctx context.Context, offer *sqlc.Offer) error {
	if offer.Status != string(domain.OfferActive) && offer.Status != string(domain.OfferPaused) {
		return nil
	}
	if offer.ExpiresAt.Valid && h.now().After(offer.ExpiresAt.Time) {
		if err := h.Store.UpdateOfferExpire(ctx, offer.OfferID); err != nil {
			return err
		}
		offer.Status = string(domain.OfferExpired)
	}
	return nil
}

func (h *OfferHandler) now() time.Time {
	if h.Clock == nil {
		return time.Now()
	}
	return h.Clock()
}

func offerToResponse(offer sqlc.Offer, tags []string) offerResponse {
	var expiresAt *time.Time
	if offer.ExpiresAt.Valid {
		t := offer.ExpiresAt.Time
		expiresAt = &t
	}
	var cancelledAt *time.Time
	if offer.CancelledAt.Valid {
		t := offer.CancelledAt.Time
		cancelledAt = &t
	}
	return offerResponse{
		OfferID:           offer.OfferID,
		SellerBotID:       offer.SellerBotID,
		Title:             offer.Title,
		Description:       offer.Description,
		Tags:              tags,
		PriceRaw:          offer.PriceRaw,
		TurnaroundSeconds: offer.TurnaroundSeconds,
		CreatedAt:         offer.CreatedAt,
		ExpiresAt:         expiresAt,
		Status:            offer.Status,
		CancelledAt:       cancelledAt,
	}
}

func publicOfferToResponse(entry offerEntry) publicOfferResponse {
	return publicOfferResponse{
		OfferID:       entry.Offer.OfferID,
		Title:         entry.Offer.Title,
		Description:   entry.Offer.Description,
		Tags:          entry.Tags,
		PriceRaw:      entry.Offer.PriceRaw,
		PurchaseCount: entry.PurchaseCount,
		CreatedAt:     entry.Offer.CreatedAt,
	}
}

func normalizeOfferCreate(payload offerCreateRequest) (offerCreateRequest, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" || len(title) > 80 {
		return offerCreateRequest{}, errors.New("invalid title")
	}
	description := strings.TrimSpace(payload.Description)
	if description == "" || len(description) > 800 {
		return offerCreateRequest{}, errors.New("invalid description")
	}
	if payload.PriceRaw == "" {
		return offerCreateRequest{}, errors.New("missing price_raw")
	}
	if payload.TurnaroundSeconds <= 0 {
		return offerCreateRequest{}, errors.New("invalid turnaround_seconds")
	}
	if len(payload.Tags) == 0 {
		return offerCreateRequest{}, errors.New("missing tags")
	}
	if len(payload.Tags) > 8 {
		return offerCreateRequest{}, errors.New("too many tags")
	}

	tags := make([]string, 0, len(payload.Tags))
	seen := make(map[string]struct{})
	for _, tag := range payload.Tags {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			return offerCreateRequest{}, errors.New("invalid tag")
		}
		if len(normalized) > 24 {
			return offerCreateRequest{}, errors.New("tag too long")
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		tags = append(tags, normalized)
	}

	if len(tags) == 0 {
		return offerCreateRequest{}, errors.New("missing tags")
	}
	if !isNumeric(payload.PriceRaw) {
		return offerCreateRequest{}, errors.New("invalid price_raw")
	}

	requestSchemaHint := strings.TrimSpace(payload.RequestSchemaHint)
	if len(requestSchemaHint) > maxRequestSchemaHintBytes {
		return offerCreateRequest{}, errors.New("request_schema_hint too long")
	}

	return offerCreateRequest{
		Title:             title,
		Description:       description,
		Tags:              tags,
		PriceRaw:          payload.PriceRaw,
		TurnaroundSeconds: payload.TurnaroundSeconds,
		ExpiresAt:         payload.ExpiresAt,
		RequestSchemaHint: requestSchemaHint,
	}, nil
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return 50, nil
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		return 0, errors.New("invalid limit")
	}
	if val > 200 {
		val = 200
	}
	return val, nil
}

func parseTagFilter(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	filter := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filter = append(filter, trimmed)
	}
	return filter
}

func includePausedFlag(includePaused bool) int {
	if includePaused {
		return 1
	}
	return 0
}

func offerStatusVisible(status string, includePaused bool) bool {
	if status == string(domain.OfferActive) {
		return true
	}
	return includePaused && status == string(domain.OfferPaused)
}

func isFTSUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "fts5") || strings.Contains(msg, "offers_fts")
}

func offerHasTags(tags []string, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	lookup := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		lookup[strings.ToLower(tag)] = struct{}{}
	}
	for _, want := range filter {
		if _, ok := lookup[strings.ToLower(want)]; !ok {
			return false
		}
	}
	return true
}

func scoreOffer(offer sqlc.Offer, tags []string, query string) int {
	if query == "" {
		return 0
	}
	needle := strings.ToLower(query)
	score := 0
	if strings.Contains(strings.ToLower(offer.Title), needle) {
		score += 3
	}
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), needle) {
			score += 2
			break
		}
	}
	if strings.Contains(strings.ToLower(offer.Description), needle) {
		score += 1
	}
	return score
}

func sortOfferEntries(entries []offerEntry, sortParam string) error {
	switch sortParam {
	case "newest":
		sort.SliceStable(entries, func(i, j int) bool {
			return compareNewest(entries[i], entries[j]) < 0
		})
	case "relevance":
		sort.SliceStable(entries, func(i, j int) bool {
			return compareRelevance(entries[i], entries[j]) < 0
		})
	case "most_purchased":
		sort.SliceStable(entries, func(i, j int) bool {
			return compareMostPurchased(entries[i], entries[j]) < 0
		})
	case "price_asc":
		sort.SliceStable(entries, func(i, j int) bool {
			return comparePrice(entries[i], entries[j], true) < 0
		})
	case "price_desc":
		sort.SliceStable(entries, func(i, j int) bool {
			return comparePrice(entries[i], entries[j], false) < 0
		})
	case "turnaround_asc":
		sort.SliceStable(entries, func(i, j int) bool {
			return compareTurnaround(entries[i], entries[j]) < 0
		})
	case "expires_asc":
		sort.SliceStable(entries, func(i, j int) bool {
			return compareExpires(entries[i], entries[j]) < 0
		})
	default:
		return errors.New("invalid sort")
	}
	return nil
}

func compareNewest(a, b offerEntry) int {
	if a.Offer.CreatedAt.After(b.Offer.CreatedAt) {
		return -1
	}
	if a.Offer.CreatedAt.Before(b.Offer.CreatedAt) {
		return 1
	}
	if a.Offer.OfferID > b.Offer.OfferID {
		return -1
	}
	if a.Offer.OfferID < b.Offer.OfferID {
		return 1
	}
	return 0
}

func compareRelevance(a, b offerEntry) int {
	if a.Score > b.Score {
		return -1
	}
	if a.Score < b.Score {
		return 1
	}
	return compareNewest(a, b)
}

func compareMostPurchased(a, b offerEntry) int {
	if a.PurchaseCount > b.PurchaseCount {
		return -1
	}
	if a.PurchaseCount < b.PurchaseCount {
		return 1
	}
	return compareNewest(a, b)
}

func comparePrice(a, b offerEntry, asc bool) int {
	cmp := a.Price.Cmp(b.Price)
	if cmp != 0 {
		if asc {
			if cmp < 0 {
				return -1
			}
			return 1
		}
		if cmp > 0 {
			return -1
		}
		return 1
	}
	return compareNewest(a, b)
}

func compareTurnaround(a, b offerEntry) int {
	if a.Offer.TurnaroundSeconds < b.Offer.TurnaroundSeconds {
		return -1
	}
	if a.Offer.TurnaroundSeconds > b.Offer.TurnaroundSeconds {
		return 1
	}
	return compareNewest(a, b)
}

func compareExpires(a, b offerEntry) int {
	left := expiresTime(a.Offer)
	right := expiresTime(b.Offer)
	if left.Before(right) {
		return -1
	}
	if left.After(right) {
		return 1
	}
	return compareNewest(a, b)
}

func expiresTime(offer sqlc.Offer) time.Time {
	if offer.ExpiresAt.Valid {
		return offer.ExpiresAt.Time
	}
	return time.Time{}
}

func findCursorStart(entries []offerEntry, cursor offerCursor, sortParam string) int {
	for i, entry := range entries {
		if compareEntryToCursor(entry, cursor, sortParam) > 0 {
			return i
		}
	}
	return len(entries)
}

func compareEntryToCursor(entry offerEntry, cursor offerCursor, sortParam string) int {
	switch sortParam {
	case "newest":
		return compareEntryNewestCursor(entry, cursor)
	case "relevance":
		return compareEntryRelevanceCursor(entry, cursor)
	case "most_purchased":
		return compareEntryMostPurchasedCursor(entry, cursor)
	case "price_asc", "price_desc":
		return compareEntryPriceCursor(entry, cursor, sortParam == "price_asc")
	case "turnaround_asc":
		return compareEntryTurnaroundCursor(entry, cursor)
	case "expires_asc":
		return compareEntryExpiresCursor(entry, cursor)
	default:
		return 1
	}
}

func compareEntryNewestCursor(entry offerEntry, cursor offerCursor) int {
	created := cursor.CreatedAt
	if entry.Offer.CreatedAt.After(created) {
		return -1
	}
	if entry.Offer.CreatedAt.Before(created) {
		return 1
	}
	if entry.Offer.OfferID > cursor.OfferID {
		return -1
	}
	if entry.Offer.OfferID < cursor.OfferID {
		return 1
	}
	return 0
}

func compareEntryRelevanceCursor(entry offerEntry, cursor offerCursor) int {
	if entry.Score > cursor.Score {
		return -1
	}
	if entry.Score < cursor.Score {
		return 1
	}
	return compareEntryNewestCursor(entry, cursor)
}

func compareEntryMostPurchasedCursor(entry offerEntry, cursor offerCursor) int {
	if entry.PurchaseCount > cursor.PurchaseCount {
		return -1
	}
	if entry.PurchaseCount < cursor.PurchaseCount {
		return 1
	}
	return compareEntryNewestCursor(entry, cursor)
}

func compareEntryPriceCursor(entry offerEntry, cursor offerCursor, asc bool) int {
	curPrice := parsePrice(cursor.PriceRaw)
	cmp := entry.Price.Cmp(curPrice)
	if cmp != 0 {
		if asc {
			if cmp < 0 {
				return -1
			}
			return 1
		}
		if cmp > 0 {
			return -1
		}
		return 1
	}
	return compareEntryNewestCursor(entry, cursor)
}

func compareEntryTurnaroundCursor(entry offerEntry, cursor offerCursor) int {
	if entry.Offer.TurnaroundSeconds < cursor.TurnaroundSeconds {
		return -1
	}
	if entry.Offer.TurnaroundSeconds > cursor.TurnaroundSeconds {
		return 1
	}
	return compareEntryNewestCursor(entry, cursor)
}

func compareEntryExpiresCursor(entry offerEntry, cursor offerCursor) int {
	cur := time.Time{}
	if cursor.ExpiresAt != nil {
		cur = *cursor.ExpiresAt
	}
	entryExpires := expiresTime(entry.Offer)
	if entryExpires.Before(cur) {
		return -1
	}
	if entryExpires.After(cur) {
		return 1
	}
	return compareEntryNewestCursor(entry, cursor)
}

func cursorFromEntry(entry offerEntry, sortParam, query string) offerCursor {
	cursor := offerCursor{
		Sort:      sortParam,
		CreatedAt: entry.Offer.CreatedAt,
		OfferID:   entry.Offer.OfferID,
		Query:     query,
	}
	switch sortParam {
	case "relevance":
		cursor.Score = entry.Score
	case "most_purchased":
		cursor.PurchaseCount = entry.PurchaseCount
	case "price_asc", "price_desc":
		cursor.PriceRaw = entry.Offer.PriceRaw
	case "turnaround_asc":
		cursor.TurnaroundSeconds = entry.Offer.TurnaroundSeconds
	case "expires_asc":
		if entry.Offer.ExpiresAt.Valid {
			exp := entry.Offer.ExpiresAt.Time
			cursor.ExpiresAt = &exp
		}
	}
	return cursor
}

func encodeOfferCursor(cursor offerCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeOfferCursor(raw string) (offerCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return offerCursor{}, err
	}
	var cursor offerCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return offerCursor{}, err
	}
	return cursor, nil
}

func parsePrice(raw string) *big.Int {
	val, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return big.NewInt(0)
	}
	return val
}

func isNumeric(raw string) bool {
	if raw == "" {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func mustJSON(tags []string) string {
	data, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func newOfferID(now time.Time) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("offer_%019d_%x", now.UnixNano(), buf), nil
}

const selectOffersQuery = `
SELECT o.offer_id, o.seller_bot_id, o.title, o.description, o.tags_json, o.price_raw, o.turnaround_seconds, o.created_at, o.expires_at, o.status, o.cancelled_at, o.request_schema_hint,
	COALESCE(p.purchase_count, 0) AS purchase_count
FROM offers o
LEFT JOIN (
	SELECT offer_id, COUNT(1) AS purchase_count
	FROM jobs
	WHERE status IN ('PAID', 'DELIVERED')
	GROUP BY offer_id
) p ON p.offer_id = o.offer_id
WHERE (?1 = '' OR o.seller_bot_id = ?1)
	AND (o.status = 'ACTIVE' OR (?2 = 1 AND o.status = 'PAUSED'))`

const selectOffersSearchQuery = `
SELECT o.offer_id, o.seller_bot_id, o.title, o.description, o.tags_json, o.price_raw, o.turnaround_seconds, o.created_at, o.expires_at, o.status, o.cancelled_at, o.request_schema_hint,
	COALESCE(p.purchase_count, 0) AS purchase_count,
	CAST((1000000.0 / (1.0 + bm25(offers_fts, 10.0, 2.0, 5.0))) AS INTEGER) AS score
FROM offers_fts
JOIN offers o ON o.rowid = offers_fts.rowid
LEFT JOIN (
	SELECT offer_id, COUNT(1) AS purchase_count
	FROM jobs
	WHERE status IN ('PAID', 'DELIVERED')
	GROUP BY offer_id
) p ON p.offer_id = o.offer_id
WHERE offers_fts MATCH ?1
	AND (?2 = '' OR o.seller_bot_id = ?2)
	AND (o.status = 'ACTIVE' OR (?3 = 1 AND o.status = 'PAUSED'))`
