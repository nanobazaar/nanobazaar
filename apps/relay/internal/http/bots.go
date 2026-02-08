package httpapi

import (
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"

	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

const headerBotID = "X-NBR-Bot-Id"

var base32LowerNoPad = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

type BotHandler struct {
	Store     *store.Store
	Clock     func() time.Time
	StreamHub StreamNotifier
}

func NewBotHandler(store *store.Store) *BotHandler {
	return &BotHandler{
		Store: store,
		Clock: time.Now,
	}
}

type botRegistrationRequest struct {
	SigningPubkeyEd25519   string `json:"signing_pubkey_ed25519"`
	EncryptionPubkeyX25519 string `json:"encryption_pubkey_x25519"`
	SigningKid             string `json:"signing_kid"`
	EncryptionKid          string `json:"encryption_kid"`
}

type botNameRequest struct {
	BotName *string `json:"bot_name"`
}

type botResponse struct {
	BotID                  string     `json:"bot_id"`
	BotName                string     `json:"bot_name,omitempty"`
	SigningPubkeyEd25519   string     `json:"signing_pubkey_ed25519"`
	EncryptionPubkeyX25519 string     `json:"encryption_pubkey_x25519"`
	SigningKid             string     `json:"signing_kid"`
	EncryptionKid          string     `json:"encryption_kid"`
	CreatedAt              time.Time  `json:"created_at"`
	LastSeenAt             *time.Time `json:"last_seen_at,omitempty"`
	Revoked                bool       `json:"revoked"`
	RevokedAt              *time.Time `json:"revoked_at,omitempty"`
}

type botRevokeResponse struct {
	BotID     string    `json:"bot_id"`
	Revoked   bool      `json:"revoked"`
	RevokedAt time.Time `json:"revoked_at"`
}

func (h *BotHandler) Register(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	var payload botRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.SigningPubkeyEd25519 == "" || payload.EncryptionPubkeyX25519 == "" || payload.SigningKid == "" || payload.EncryptionKid == "" {
		writeJSONError(w, http.StatusBadRequest, "missing fields")
		return
	}

	signingPub, err := decodeKey(payload.SigningPubkeyEd25519, ed25519.PublicKeySize)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid signing key")
		return
	}
	encryptionPub, err := decodeKey(payload.EncryptionPubkeyX25519, ed25519.PublicKeySize)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid encryption key")
		return
	}

	expectedSigningKid := kidFromKey(signingPub)
	if payload.SigningKid != expectedSigningKid {
		writeJSONError(w, http.StatusBadRequest, "signing_kid mismatch")
		return
	}
	expectedEncryptionKid := kidFromKey(encryptionPub)
	if payload.EncryptionKid != expectedEncryptionKid {
		writeJSONError(w, http.StatusBadRequest, "encryption_kid mismatch")
		return
	}

	botID := botIDFromSigningKey(signingPub)
	if caller := r.Header.Get(headerBotID); caller == "" || caller != botID {
		writeJSONError(w, http.StatusBadRequest, "bot_id mismatch")
		return
	}

	signingKeyCanonical := base64.RawURLEncoding.EncodeToString(signingPub)
	encryptionKeyCanonical := base64.RawURLEncoding.EncodeToString(encryptionPub)

	existing, err := h.Store.GetBot(r.Context(), botID)
	switch {
	case err == nil:
		if !botKeysMatch(existing, signingKeyCanonical, encryptionKeyCanonical, expectedSigningKid, expectedEncryptionKid) {
			writeJSONError(w, http.StatusConflict, "bot_id already pinned")
			return
		}
		now := h.now()
		_ = h.Store.UpdateBotLastSeen(r.Context(), sqlc.UpdateBotLastSeenParams{
			BotID:      botID,
			LastSeenAt: sql.NullTime{Time: now, Valid: true},
		})
		existing.LastSeenAt = sql.NullTime{Time: now, Valid: true}
		log.Printf("bot_register bot_id=%s existing=true", botID)
		writeJSON(w, http.StatusOK, botToResponse(existing))
		return
	case errors.Is(err, sql.ErrNoRows):
		// proceed
	default:
		writeJSONError(w, http.StatusInternalServerError, "bot lookup failed")
		return
	}

	now := h.now()
	createErr := h.Store.CreateBot(r.Context(), sqlc.CreateBotParams{
		BotID:                  botID,
		SigningPubkeyEd25519:   signingKeyCanonical,
		EncryptionPubkeyX25519: encryptionKeyCanonical,
		SigningKid:             expectedSigningKid,
		EncryptionKid:          expectedEncryptionKid,
		CreatedAt:              now,
		LastSeenAt:             sql.NullTime{Time: now, Valid: true},
	})
	if createErr != nil {
		if !isConstraintError(createErr) {
			writeJSONError(w, http.StatusInternalServerError, "bot create failed")
			return
		}
		existing, err = h.Store.GetBot(r.Context(), botID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "bot lookup failed")
			return
		}
		if !botKeysMatch(existing, signingKeyCanonical, encryptionKeyCanonical, expectedSigningKid, expectedEncryptionKid) {
			writeJSONError(w, http.StatusConflict, "bot_id already pinned")
			return
		}
		log.Printf("bot_register bot_id=%s existing=true", botID)
		writeJSON(w, http.StatusOK, botToResponse(existing))
		return
	}

	log.Printf("bot_register bot_id=%s existing=false", botID)
	writeJSON(w, http.StatusOK, botResponse{
		BotID:                  botID,
		SigningPubkeyEd25519:   signingKeyCanonical,
		EncryptionPubkeyX25519: encryptionKeyCanonical,
		SigningKid:             expectedSigningKid,
		EncryptionKid:          expectedEncryptionKid,
		CreatedAt:              now,
		LastSeenAt:             &now,
	})
}

func (h *BotHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, botToResponse(bot))
}

func (h *BotHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}
	botID := chi.URLParam(r, "bot_id")
	if botID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing bot_id")
		return
	}
	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != botID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	now := h.now()
	ctx := r.Context()
	tx, err := h.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("bot_revoke_failed step=begin_tx bot_id=%s err=%v", botID, err)
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()
	qtx := sqlc.New(tx)

	updated, err := qtx.UpdateBotRevoke(ctx, sqlc.UpdateBotRevokeParams{
		BotID:     botID,
		RevokedAt: sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "bot not found")
			return
		}
		log.Printf("bot_revoke_failed step=update_bot bot_id=%s err=%v", botID, err)
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	if !updated.RevokedAt.Valid {
		log.Printf("bot_revoke_failed step=update_bot bot_id=%s err=revoked_at_invalid", botID)
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
		log.Printf("bot_revoke_failed step=cancel_offers bot_id=%s err=%v", botID, err)
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	for _, offer := range offers {
		if err := emitEventTx(ctx, qtx, h.StreamHub, offer.SellerBotID, offerCancelledEventType, map[string]any{
			"offer_id":     offer.OfferID,
			"cancelled_at": revokedAt.UTC().Format(time.RFC3339Nano),
		}, true); err != nil {
			log.Printf("bot_revoke_failed step=emit_offer_event bot_id=%s offer_id=%s err=%v", botID, offer.OfferID, err)
			writeJSONError(w, http.StatusInternalServerError, "event create failed")
			return
		}
	}

	jobs, err := qtx.CancelJobsByBot(ctx, sqlc.CancelJobsByBotParams{
		BotID:       botID,
		CancelledAt: cancelledAt,
	})
	if err != nil {
		log.Printf("bot_revoke_failed step=cancel_jobs bot_id=%s err=%v", botID, err)
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	for _, job := range jobs {
		payload := map[string]any{
			"job_id":       job.JobID,
			"cancelled_at": revokedAt.UTC().Format(time.RFC3339Nano),
		}
		recipients := []string{job.BuyerBotID, job.SellerBotID}
		if job.BuyerBotID == job.SellerBotID {
			recipients = []string{job.BuyerBotID}
		}
		for i, recipient := range recipients {
			emitJobStream := i == 0
			if err := emitEventTx(ctx, qtx, h.StreamHub, recipient, jobCancelledEventType, payload, emitJobStream); err != nil {
				log.Printf("bot_revoke_failed step=emit_job_event bot_id=%s job_id=%s recipient=%s err=%v", botID, job.JobID, recipient, err)
				writeJSONError(w, http.StatusInternalServerError, "event create failed")
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("bot_revoke_failed step=commit bot_id=%s err=%v", botID, err)
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}

	writeJSON(w, http.StatusOK, botRevokeResponse{
		BotID:     updated.BotID,
		Revoked:   true,
		RevokedAt: revokedAt,
	})
}

func (h *BotHandler) SetName(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "store unavailable")
		return
	}

	botID := chi.URLParam(r, "bot_id")
	if botID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing bot_id")
		return
	}

	caller := r.Header.Get(headerBotID)
	if caller == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing bot_id")
		return
	}
	if caller != botID {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	var payload botNameRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if payload.BotName == nil {
		writeJSONError(w, http.StatusBadRequest, "missing bot_name")
		return
	}

	name, err := normalizeBotName(*payload.BotName)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.Store.UpdateBotName(r.Context(), sqlc.UpdateBotNameParams{
		BotID:   botID,
		BotName: name,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "bot not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "bot update failed")
		return
	}

	writeJSON(w, http.StatusOK, botToResponse(updated))
}

func (h *BotHandler) now() time.Time {
	if h.Clock == nil {
		return time.Now()
	}
	return h.Clock()
}

func botIDFromSigningKey(pub []byte) string {
	sum := sha256.Sum256(pub)
	return "b" + base32LowerNoPad.EncodeToString(sum[:])
}

func kidFromKey(pub []byte) string {
	sum := sha256.Sum256(pub)
	return "b" + base32LowerNoPad.EncodeToString(sum[:16])
}

func decodeKey(value string, size int) ([]byte, error) {
	if value == "" {
		return nil, errors.New("empty key")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, err
		}
	}
	if len(decoded) != size {
		return nil, fmt.Errorf("unexpected key length %d", len(decoded))
	}
	return decoded, nil
}

func botKeysMatch(bot sqlc.Bot, signingKey, encryptionKey, signingKid, encryptionKid string) bool {
	return bot.SigningPubkeyEd25519 == signingKey &&
		bot.EncryptionPubkeyX25519 == encryptionKey &&
		bot.SigningKid == signingKid &&
		bot.EncryptionKid == encryptionKid
}

func botToResponse(bot sqlc.Bot) botResponse {
	botName := ""
	if bot.BotName.Valid {
		botName = bot.BotName.String
	}
	var lastSeen *time.Time
	if bot.LastSeenAt.Valid {
		t := bot.LastSeenAt.Time
		lastSeen = &t
	}
	var revokedAt *time.Time
	revoked := false
	if bot.RevokedAt.Valid {
		t := bot.RevokedAt.Time
		revokedAt = &t
		revoked = true
	}
	return botResponse{
		BotID:                  bot.BotID,
		BotName:                botName,
		SigningPubkeyEd25519:   bot.SigningPubkeyEd25519,
		EncryptionPubkeyX25519: bot.EncryptionPubkeyX25519,
		SigningKid:             bot.SigningKid,
		EncryptionKid:          bot.EncryptionKid,
		CreatedAt:              bot.CreatedAt,
		LastSeenAt:             lastSeen,
		Revoked:                revoked,
		RevokedAt:              revokedAt,
	}
}

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func normalizeBotName(value string) (sql.NullString, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}, nil
	}
	if len(trimmed) > 64 {
		return sql.NullString{}, errors.New("bot_name too long (max 64 bytes)")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return sql.NullString{}, errors.New("bot_name contains control characters")
		}
	}
	return sql.NullString{String: trimmed, Valid: true}, nil
}
