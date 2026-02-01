package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/store/sqlc"
)

type fakeStore struct {
	mu          sync.Mutex
	bots        map[string]sqlc.Bot
	nonces      map[string]map[string]time.Time
	idempotency map[string]sqlc.IdempotencyKey
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		bots:        make(map[string]sqlc.Bot),
		nonces:      make(map[string]map[string]time.Time),
		idempotency: make(map[string]sqlc.IdempotencyKey),
	}
}

func (f *fakeStore) GetBot(_ context.Context, botID string) (sqlc.Bot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	bot, ok := f.bots[botID]
	if !ok {
		return sqlc.Bot{}, sql.ErrNoRows
	}
	return bot, nil
}

func (f *fakeStore) CountNonce(_ context.Context, arg sqlc.CountNonceParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nonces[arg.BotID] == nil {
		return 0, nil
	}
	if _, ok := f.nonces[arg.BotID][arg.Nonce]; ok {
		return 1, nil
	}
	return 0, nil
}

func (f *fakeStore) InsertNonce(_ context.Context, arg sqlc.InsertNonceParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nonces[arg.BotID] == nil {
		f.nonces[arg.BotID] = make(map[string]time.Time)
	}
	f.nonces[arg.BotID][arg.Nonce] = arg.CreatedAt
	return nil
}

func (f *fakeStore) DeleteNoncesBefore(_ context.Context, cutoff time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for botID, entries := range f.nonces {
		for nonce, createdAt := range entries {
			if createdAt.Before(cutoff) {
				delete(entries, nonce)
			}
		}
		if len(entries) == 0 {
			delete(f.nonces, botID)
		}
	}
	return nil
}

func (f *fakeStore) GetIdempotency(_ context.Context, arg sqlc.GetIdempotencyParams) (sqlc.IdempotencyKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := arg.BotID + "|" + arg.Endpoint + "|" + arg.IdempotencyKey
	record, ok := f.idempotency[key]
	if !ok {
		return sqlc.IdempotencyKey{}, sql.ErrNoRows
	}
	return record, nil
}

func (f *fakeStore) InsertIdempotency(_ context.Context, arg sqlc.InsertIdempotencyParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := arg.BotID + "|" + arg.Endpoint + "|" + arg.IdempotencyKey
	f.idempotency[key] = sqlc.IdempotencyKey{
		BotID:           arg.BotID,
		Endpoint:        arg.Endpoint,
		IdempotencyKey:  arg.IdempotencyKey,
		RequestHash:     arg.RequestHash,
		ResponseCode:    arg.ResponseCode,
		ResponseBody:    arg.ResponseBody,
		ResponseHeaders: arg.ResponseHeaders,
		CreatedAt:       arg.CreatedAt,
	}
	return nil
}

func (f *fakeStore) DeleteIdempotencyBefore(_ context.Context, cutoff time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, record := range f.idempotency {
		if record.CreatedAt.Before(cutoff) {
			delete(f.idempotency, key)
		}
	}
	return nil
}

func TestAuthValidSignature(t *testing.T) {
	store := newFakeStore()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	botID := "bot_123"
	store.bots[botID] = sqlc.Bot{BotID: botID, SigningPubkeyEd25519: base64.RawURLEncoding.EncodeToString(pub)}

	verifier := NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	body := []byte(`{"title":"ok"}`)
	req := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", body, now, "nonce-1")
	req.Header.Set(headerIdempotency, "idem-1")

	handler := Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthInvalidSignature(t *testing.T) {
	store := newFakeStore()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	botID := "bot_123"
	store.bots[botID] = sqlc.Bot{BotID: botID, SigningPubkeyEd25519: base64.RawURLEncoding.EncodeToString(pub)}

	verifier := NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	body := []byte(`{"title":"ok"}`)
	req, err := http.NewRequest(http.MethodPost, "/v0/offers", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	bodyHash := sha256Hex(body)
	canonical := canonicalString(http.MethodPost, "/v0/offers", "", now.Format(time.RFC3339), "nonce-1", bodyHash)
	fakeSig := ed25519.Sign(ed25519.NewKeyFromSeed(make([]byte, 32)), []byte(canonical))

	setAuthHeaders(req, botID, now, "nonce-1", bodyHash, fakeSig)
	req.Header.Set(headerIdempotency, "idem-1")

	handler := Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestNonceReplay(t *testing.T) {
	store := newFakeStore()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	botID := "bot_123"
	store.bots[botID] = sqlc.Bot{BotID: botID, SigningPubkeyEd25519: base64.RawURLEncoding.EncodeToString(pub)}

	verifier := NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	body := []byte(`{}`)
	req1 := signedRequest(t, priv, botID, http.MethodGet, "/v0/bots/abc", "", body, now, "nonce-1")
	rec1 := httptest.NewRecorder()
	Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })).ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec1.Code)
	}

	req2 := signedRequest(t, priv, botID, http.MethodGet, "/v0/bots/abc", "", body, now, "nonce-1")
	rec2 := httptest.NewRecorder()
	Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })).ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec2.Code)
	}
}

func TestIdempotencyCollision(t *testing.T) {
	store := newFakeStore()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	botID := "bot_123"
	store.bots[botID] = sqlc.Bot{BotID: botID, SigningPubkeyEd25519: base64.RawURLEncoding.EncodeToString(pub)}

	verifier := NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	body1 := []byte(`{"title":"one"}`)
	req1 := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", body1, now, "nonce-1")
	req1.Header.Set(headerIdempotency, "idem-1")

	rec1 := httptest.NewRecorder()
	Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})).ServeHTTP(rec1, req1)

	body2 := []byte(`{"title":"two"}`)
	req2 := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", body2, now, "nonce-2")
	req2.Header.Set(headerIdempotency, "idem-1")

	rec2 := httptest.NewRecorder()
	Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec2.Code)
	}
}

func TestIdempotencyReplay(t *testing.T) {
	store := newFakeStore()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	botID := "bot_123"
	store.bots[botID] = sqlc.Bot{BotID: botID, SigningPubkeyEd25519: base64.RawURLEncoding.EncodeToString(pub)}

	verifier := NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	body := []byte(`{"title":"one"}`)
	req1 := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", body, now, "nonce-1")
	req1.Header.Set(headerIdempotency, "idem-1")

	count := 0
	h := Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec1.Code)
	}

	req2 := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", body, now, "nonce-2")
	req2.Header.Set(headerIdempotency, "idem-1")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec2.Code)
	}
	if rec2.Body.String() != "created" {
		t.Fatalf("expected stored body, got %q", rec2.Body.String())
	}
	if count != 1 {
		t.Fatalf("expected handler called once, got %d", count)
	}
}

func TestJobsIdempotencyUsesJobID(t *testing.T) {
	store := newFakeStore()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	botID := "bot_123"
	store.bots[botID] = sqlc.Bot{BotID: botID, SigningPubkeyEd25519: base64.RawURLEncoding.EncodeToString(pub)}

	verifier := NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	body := []byte(`{"job_id":"job_123"}`)
	req := signedRequest(t, priv, botID, http.MethodPost, "/v0/jobs", "", body, now, "nonce-1")

	rec := httptest.NewRecorder()
	Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	key := botID + "|" + "POST /v0/jobs" + "|" + "job_123"
	if _, ok := store.idempotency[key]; !ok {
		t.Fatalf("expected idempotency record stored")
	}
}

func signedRequest(t *testing.T, priv ed25519.PrivateKey, botID, method, path, rawQuery string, body []byte, ts time.Time, nonce string) *http.Request {
	url := path
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	bodyHash := sha256Hex(body)
	canonical := canonicalString(method, path, rawQuery, ts.Format(time.RFC3339), nonce, bodyHash)
	sig := ed25519.Sign(priv, []byte(canonical))
	setAuthHeaders(req, botID, ts, nonce, bodyHash, sig)
	return req
}

func setAuthHeaders(req *http.Request, botID string, ts time.Time, nonce, bodyHash string, sig []byte) {
	req.Header.Set(headerBotID, botID)
	req.Header.Set(headerTimestamp, ts.Format(time.RFC3339))
	req.Header.Set(headerNonce, nonce)
	req.Header.Set(headerBodySHA256, bodyHash)
	req.Header.Set(headerSignature, base64.RawURLEncoding.EncodeToString(sig))
}

func sha256Hex(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func TestBotsRegistrationUsesBodyKey(t *testing.T) {
	store := newFakeStore()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	botID := "bot_123"

	verifier := NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	payload := map[string]string{
		"signing_pubkey_ed25519": base64.RawURLEncoding.EncodeToString(pub),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := signedRequest(t, priv, botID, http.MethodPost, "/v0/bots", "", body, now, "nonce-1")
	req.Header.Set(headerIdempotency, "idem-1")

	rec := httptest.NewRecorder()
	Middleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
