package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

const (
	headerBotID       = "X-NBR-Bot-Id"
	headerTimestamp   = "X-NBR-Timestamp"
	headerNonce       = "X-NBR-Nonce"
	headerBodySHA256  = "X-NBR-Body-SHA256"
	headerSignature   = "X-NBR-Signature"
	headerIdempotency = "X-Idempotency-Key"
)

type Store interface {
	GetBot(ctx context.Context, botID string) (sqlc.Bot, error)
	CountNonce(ctx context.Context, arg sqlc.CountNonceParams) (int64, error)
	InsertNonce(ctx context.Context, arg sqlc.InsertNonceParams) error
	DeleteNoncesBefore(ctx context.Context, cutoff time.Time) error
	GetIdempotency(ctx context.Context, arg sqlc.GetIdempotencyParams) (sqlc.IdempotencyKey, error)
	InsertIdempotency(ctx context.Context, arg sqlc.InsertIdempotencyParams) error
	DeleteIdempotencyBefore(ctx context.Context, cutoff time.Time) error
}

type Verifier struct {
	Store          Store
	Clock          func() time.Time
	NonceTTL       time.Duration
	ReplayWindow   time.Duration
	IdempotencyTTL time.Duration
	Metrics        *metrics.Registry
}

type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	return e.Message
}

func NewVerifier(store Store) *Verifier {
	return &Verifier{
		Store:          store,
		Clock:          time.Now,
		NonceTTL:       10 * time.Minute,
		ReplayWindow:   5 * time.Minute,
		IdempotencyTTL: 30 * 24 * time.Hour,
	}
}

func Middleware(v *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v == nil || v.Store == nil {
				next.ServeHTTP(w, r)
				return
			}

			bodyBytes, bodyHash, err := readBodyAndHash(r)
			if err != nil {
				writeError(w, &HTTPError{Status: http.StatusBadRequest, Message: "invalid body"})
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			if err := v.verifyRequest(r, bodyBytes, bodyHash); err != nil {
				if v.Metrics != nil {
					v.Metrics.RecordAuthFailure(err.Message)
				}
				logAuthFailure(r, err.Message)
				writeError(w, err)
				return
			}

			if !isMutating(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			botID := r.Header.Get(headerBotID)
			endpoint := r.Method + " " + canonicalPath(r)
			key, idemErr := v.resolveIdempotencyKey(r, bodyBytes)
			if idemErr != nil {
				writeError(w, idemErr)
				return
			}

			stored, err := v.Store.GetIdempotency(r.Context(), sqlc.GetIdempotencyParams{
				BotID:          botID,
				Endpoint:       endpoint,
				IdempotencyKey: key,
			})
			switch {
			case err == nil:
				if stored.RequestHash != bodyHash {
					writeError(w, &HTTPError{Status: http.StatusConflict, Message: "idempotency collision"})
					return
				}
				writeStoredResponse(w, stored)
				return
			case errors.Is(err, sql.ErrNoRows):
				// proceed
			default:
				logInternalError(r, "idempotency_lookup_failed", http.StatusInternalServerError, err)
				writeError(w, &HTTPError{Status: http.StatusInternalServerError, Message: "idempotency lookup failed"})
				return
			}

			recorder := newResponseRecorder()
			next.ServeHTTP(recorder, r)

			headersJSON, _ := json.Marshal(recorder.header)
			if err := v.Store.InsertIdempotency(r.Context(), sqlc.InsertIdempotencyParams{
				BotID:           botID,
				Endpoint:        endpoint,
				IdempotencyKey:  key,
				RequestHash:     bodyHash,
				ResponseCode:    int64(recorder.statusCode()),
				ResponseBody:    recorder.body.String(),
				ResponseHeaders: string(headersJSON),
				CreatedAt:       v.now(),
			}); err != nil {
				logInternalError(r, "idempotency_insert_failed", http.StatusInternalServerError, err)
				writeError(w, &HTTPError{Status: http.StatusInternalServerError, Message: "idempotency insert failed"})
				return
			}

			recorder.writeTo(w)
		})
	}
}

func (v *Verifier) verifyRequest(r *http.Request, bodyBytes []byte, bodyHash string) *HTTPError {
	botID := r.Header.Get(headerBotID)
	timestamp := r.Header.Get(headerTimestamp)
	nonce := r.Header.Get(headerNonce)
	sigHeader := r.Header.Get(headerSignature)
	bodyHeader := r.Header.Get(headerBodySHA256)

	if botID == "" || timestamp == "" || nonce == "" || sigHeader == "" || bodyHeader == "" {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "missing auth headers"}
	}
	if bodyHeader != bodyHash {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "body hash mismatch"}
	}

	timeVal, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || !strings.HasSuffix(timestamp, "Z") {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "invalid timestamp"}
	}
	if !withinWindow(v.now(), timeVal, v.ReplayWindow) {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "stale timestamp"}
	}

	pubkey, authErr := v.resolveSigningKey(r, bodyBytes)
	if authErr != nil {
		return authErr
	}

	canonical := canonicalString(r.Method, canonicalPath(r), r.URL.RawQuery, timestamp, nonce, bodyHash)
	sigBytes, err := base64.RawURLEncoding.DecodeString(sigHeader)
	if err != nil {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "invalid signature"}
	}
	if !ed25519.Verify(pubkey, []byte(canonical), sigBytes) {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "invalid signature"}
	}

	v.cleanup(r.Context())

	count, err := v.Store.CountNonce(r.Context(), sqlc.CountNonceParams{BotID: botID, Nonce: nonce})
	if err != nil {
		logInternalError(r, "nonce_lookup_failed", http.StatusInternalServerError, err)
		return &HTTPError{Status: http.StatusInternalServerError, Message: "nonce lookup failed"}
	}
	if count > 0 {
		return &HTTPError{Status: http.StatusUnauthorized, Message: "nonce reuse"}
	}

	if err := v.Store.InsertNonce(r.Context(), sqlc.InsertNonceParams{BotID: botID, Nonce: nonce, CreatedAt: v.now()}); err != nil {
		logInternalError(r, "nonce_insert_failed", http.StatusInternalServerError, err)
		return &HTTPError{Status: http.StatusInternalServerError, Message: "nonce insert failed"}
	}

	return nil
}

func (v *Verifier) resolveSigningKey(r *http.Request, bodyBytes []byte) (ed25519.PublicKey, *HTTPError) {
	if r.Method == http.MethodPost && r.URL.Path == "/v0/bots" {
		var payload struct {
			SigningPubkeyEd25519 string `json:"signing_pubkey_ed25519"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return nil, &HTTPError{Status: http.StatusUnauthorized, Message: "unknown bot"}
		}
		pub, err := decodePublicKey(payload.SigningPubkeyEd25519)
		if err != nil {
			return nil, &HTTPError{Status: http.StatusUnauthorized, Message: "unknown bot"}
		}
		if !isRevokeEndpoint(r) {
			botID := strings.TrimSpace(r.Header.Get(headerBotID))
			if botID != "" {
				bot, err := v.Store.GetBot(r.Context(), botID)
				switch {
				case err == nil:
					if bot.RevokedAt.Valid {
						return nil, &HTTPError{Status: http.StatusForbidden, Message: "bot revoked"}
					}
				case errors.Is(err, sql.ErrNoRows):
					// allow new registration
				default:
					logInternalError(r, "bot_lookup_failed", http.StatusInternalServerError, err)
					return nil, &HTTPError{Status: http.StatusInternalServerError, Message: "bot lookup failed"}
				}
			}
		}
		return pub, nil
	}

	botID := strings.TrimSpace(r.Header.Get(headerBotID))
	if botID == "" {
		return nil, &HTTPError{Status: http.StatusUnauthorized, Message: "missing auth headers"}
	}
	bot, err := v.Store.GetBot(r.Context(), botID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if isRevokeEndpoint(r) {
				return nil, &HTTPError{Status: http.StatusNotFound, Message: "bot not found"}
			}
			return nil, &HTTPError{Status: http.StatusUnauthorized, Message: "unknown bot"}
		}
		logInternalError(r, "bot_lookup_failed", http.StatusInternalServerError, err)
		return nil, &HTTPError{Status: http.StatusInternalServerError, Message: "bot lookup failed"}
	}
	if bot.RevokedAt.Valid && !isRevokeEndpoint(r) {
		return nil, &HTTPError{Status: http.StatusForbidden, Message: "bot revoked"}
	}
	pub, err := decodePublicKey(bot.SigningPubkeyEd25519)
	if err != nil {
		return nil, &HTTPError{Status: http.StatusUnauthorized, Message: "unknown bot"}
	}
	return pub, nil
}

func (v *Verifier) resolveIdempotencyKey(r *http.Request, bodyBytes []byte) (string, *HTTPError) {
	if r.Method == http.MethodPost && r.URL.Path == "/v0/jobs" {
		var payload struct {
			JobID string `json:"job_id"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil || payload.JobID == "" {
			return "", &HTTPError{Status: http.StatusBadRequest, Message: "missing job_id"}
		}
		headerKey := r.Header.Get(headerIdempotency)
		if headerKey != "" && headerKey != payload.JobID {
			return "", &HTTPError{Status: http.StatusConflict, Message: "idempotency key mismatch"}
		}
		return payload.JobID, nil
	}

	key := r.Header.Get(headerIdempotency)
	if key == "" {
		return "", &HTTPError{Status: http.StatusBadRequest, Message: "missing idempotency key"}
	}
	return key, nil
}

func (v *Verifier) cleanup(ctx context.Context) {
	now := v.now()
	if v.NonceTTL > 0 {
		cutoff := now.Add(-v.NonceTTL)
		_ = v.Store.DeleteNoncesBefore(ctx, cutoff)
	}
	if v.IdempotencyTTL > 0 {
		cutoff := now.Add(-v.IdempotencyTTL)
		_ = v.Store.DeleteIdempotencyBefore(ctx, cutoff)
	}
}

func (v *Verifier) now() time.Time {
	if v.Clock == nil {
		return time.Now()
	}
	return v.Clock()
}

func readBodyAndHash(r *http.Request) ([]byte, string, error) {
	if r.Body == nil {
		return nil, emptyBodyHash(), nil
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "", err
	}
	h := sha256.Sum256(bodyBytes)
	return bodyBytes, hex.EncodeToString(h[:]), nil
}

func emptyBodyHash() string {
	h := sha256.Sum256(nil)
	return hex.EncodeToString(h[:])
}

func canonicalString(method, path, rawQuery, timestamp, nonce, bodyHash string) string {
	method = strings.ToUpper(method)
	pathAndQuery := path
	if rawQuery != "" {
		pathAndQuery += "?" + rawQuery
	}
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, pathAndQuery, timestamp, nonce, bodyHash)
}

func canonicalPath(r *http.Request) string {
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	return path
}

func isRevokeEndpoint(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	if r.Method != http.MethodPost {
		return false
	}
	path := r.URL.Path
	return strings.HasPrefix(path, "/v0/bots/") && strings.HasSuffix(path, "/revoke")
}

func withinWindow(now, ts time.Time, window time.Duration) bool {
	if window <= 0 {
		window = 5 * time.Minute
	}
	delta := now.Sub(ts)
	if delta < 0 {
		delta = -delta
	}
	return delta <= window
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
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
	if l := len(decoded); l != ed25519.PublicKeySize {
		return nil, fmt.Errorf("unexpected key length %d", l)
	}
	return ed25519.PublicKey(decoded), nil
}

type responseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header)}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *responseRecorder) statusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

func (r *responseRecorder) writeTo(w http.ResponseWriter) {
	for key, values := range r.header {
		for _, value := range values {
			if strings.EqualFold(key, "Content-Length") {
				continue
			}
			w.Header().Add(key, value)
		}
	}
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(r.body.Bytes())
}

func writeStoredResponse(w http.ResponseWriter, record sqlc.IdempotencyKey) {
	var headers http.Header
	if record.ResponseHeaders != "" {
		_ = json.Unmarshal([]byte(record.ResponseHeaders), &headers)
	}
	for key, values := range headers {
		for _, value := range values {
			if strings.EqualFold(key, "Content-Length") {
				continue
			}
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(int(record.ResponseCode))
	_, _ = io.WriteString(w, record.ResponseBody)
}

func writeError(w http.ResponseWriter, err *HTTPError) {
	if err == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err.Message)))
}

func isMutating(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func logAuthFailure(r *http.Request, reason string) {
	botID := r.Header.Get(headerBotID)
	path := canonicalPath(r)
	log.Printf("auth_failed reason=%s bot_id=%s method=%s path=%s remote_addr=%s", reason, botID, r.Method, path, r.RemoteAddr)
}

// internalErrorLogf exists for testability; do not call directly.
var internalErrorLogf = log.Printf

func logInternalError(r *http.Request, action string, status int, err error) {
	if r == nil || err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}
	botID := strings.TrimSpace(r.Header.Get(headerBotID))
	path := canonicalPath(r)
	reqID := middleware.GetReqID(r.Context())
	internalErrorLogf(
		"internal_error action=%s status=%d method=%s path=%s bot_id=%s request_id=%s remote_addr=%s err=%v",
		action,
		status,
		r.Method,
		path,
		botID,
		reqID,
		r.RemoteAddr,
		err,
	)
}
