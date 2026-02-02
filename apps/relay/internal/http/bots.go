package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nanobazaar/relay/internal/store/sqlc"
)

const headerBotID = "X-NBR-Bot-Id"

var base32LowerNoPad = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

type BotStore interface {
	CreateBot(ctx context.Context, arg sqlc.CreateBotParams) error
	GetBot(ctx context.Context, botID string) (sqlc.Bot, error)
	UpdateBotLastSeen(ctx context.Context, arg sqlc.UpdateBotLastSeenParams) error
	UpdateBotRevoke(ctx context.Context, arg sqlc.UpdateBotRevokeParams) (sqlc.Bot, error)
}

type BotHandler struct {
	Store BotStore
	Clock func() time.Time
}

func NewBotHandler(store BotStore) *BotHandler {
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

type botResponse struct {
	BotID                  string     `json:"bot_id"`
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
		writeJSON(w, http.StatusOK, botToResponse(existing))
		return
	}

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
	updated, err := h.Store.UpdateBotRevoke(r.Context(), sqlc.UpdateBotRevokeParams{
		BotID:     botID,
		RevokedAt: sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "bot not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	if !updated.RevokedAt.Valid {
		writeJSONError(w, http.StatusInternalServerError, "bot revoke failed")
		return
	}
	writeJSON(w, http.StatusOK, botRevokeResponse{
		BotID:     updated.BotID,
		Revoked:   true,
		RevokedAt: updated.RevokedAt.Time,
	})
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
