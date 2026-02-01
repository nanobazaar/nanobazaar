package httpapi

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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/store"
)

const (
	headerTimestamp   = "X-NBR-Timestamp"
	headerNonce       = "X-NBR-Nonce"
	headerBodySHA256  = "X-NBR-Body-SHA256"
	headerSignature   = "X-NBR-Signature"
	headerIdempotency = "X-Idempotency-Key"
)

func TestBotsRegister(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	encryptionPub := randomKeyBytes(t)
	botID := botIDFromSigningKey(pub)

	reqBody := botRegistrationRequest{
		SigningPubkeyEd25519:   base64.RawURLEncoding.EncodeToString(pub),
		EncryptionPubkeyX25519: base64.RawURLEncoding.EncodeToString(encryptionPub),
		SigningKid:             kidFromKey(pub),
		EncryptionKid:          kidFromKey(encryptionPub),
	}
	body := mustJSONBytes(t, reqBody)

	req := signedRequest(t, priv, botID, httpMethodPost, "/v0/bots", "", body, now, "nonce-1")
	req.Header.Set(headerIdempotency, "idem-1")

	rec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp botResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.BotID != botID {
		t.Fatalf("expected bot_id %q, got %q", botID, resp.BotID)
	}
	if resp.LastSeenAt == nil {
		t.Fatalf("expected last_seen_at set")
	}

	stored, err := store.GetBot(context.Background(), botID)
	if err != nil {
		t.Fatalf("get bot: %v", err)
	}
	if stored.SigningKid != reqBody.SigningKid || stored.EncryptionKid != reqBody.EncryptionKid {
		t.Fatalf("unexpected kids stored")
	}
	if !stored.LastSeenAt.Valid {
		t.Fatalf("expected last_seen_at stored")
	}
}

func TestBotsRegisterConflict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := botIDFromSigningKey(pub)

	firstEncPub := randomKeyBytes(t)
	first := botRegistrationRequest{
		SigningPubkeyEd25519:   base64.RawURLEncoding.EncodeToString(pub),
		EncryptionPubkeyX25519: base64.RawURLEncoding.EncodeToString(firstEncPub),
		SigningKid:             kidFromKey(pub),
		EncryptionKid:          kidFromKey(firstEncPub),
	}

	req1 := signedRequest(t, priv, botID, httpMethodPost, "/v0/bots", "", mustJSONBytes(t, first), now, "nonce-1")
	req1.Header.Set(headerIdempotency, "idem-1")
	rec1 := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	secondEncryption := randomKeyBytes(t)
	second := botRegistrationRequest{
		SigningPubkeyEd25519:   base64.RawURLEncoding.EncodeToString(pub),
		EncryptionPubkeyX25519: base64.RawURLEncoding.EncodeToString(secondEncryption),
		SigningKid:             kidFromKey(pub),
		EncryptionKid:          kidFromKey(secondEncryption),
	}

	req2 := signedRequest(t, priv, botID, httpMethodPost, "/v0/bots", "", mustJSONBytes(t, second), now, "nonce-2")
	req2.Header.Set(headerIdempotency, "idem-2")
	rec2 := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestBotsGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	encryptionPub := randomKeyBytes(t)
	botID := botIDFromSigningKey(pub)

	register := botRegistrationRequest{
		SigningPubkeyEd25519:   base64.RawURLEncoding.EncodeToString(pub),
		EncryptionPubkeyX25519: base64.RawURLEncoding.EncodeToString(encryptionPub),
		SigningKid:             kidFromKey(pub),
		EncryptionKid:          kidFromKey(encryptionPub),
	}
	req := signedRequest(t, priv, botID, httpMethodPost, "/v0/bots", "", mustJSONBytes(t, register), now, "nonce-1")
	req.Header.Set(headerIdempotency, "idem-1")
	rec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	getReq := signedRequest(t, priv, botID, httpMethodGet, "/v0/bots/"+botID, "", nil, now, "nonce-2")
	getRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var resp botResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.BotID != botID {
		t.Fatalf("expected bot_id %q, got %q", botID, resp.BotID)
	}
	if resp.LastSeenAt == nil {
		t.Fatalf("expected last_seen_at set")
	}
}

const (
	httpMethodPost = "POST"
	httpMethodGet  = "GET"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	schemaPath := filepath.Join("..", "..", "db", "schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if err := execSchema(db, string(schema)); err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}

func execSchema(db *sql.DB, schema string) error {
	if _, err := db.Exec(schema); err == nil {
		return nil
	} else if !strings.Contains(err.Error(), "fts5") {
		return err
	}
	stripped := stripFTS(schema)
	if _, err := db.Exec(stripped); err != nil {
		return err
	}
	return nil
}

func stripFTS(schema string) string {
	const startMarker = "-- FTS BEGIN"
	const endMarker = "-- FTS END"
	for {
		start := strings.Index(schema, startMarker)
		if start == -1 {
			break
		}
		end := strings.Index(schema[start:], endMarker)
		if end == -1 {
			break
		}
		end = start + end + len(endMarker)
		schema = schema[:start] + schema[end:]
	}
	return schema
}

func generateSigningKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func randomKeyBytes(t *testing.T) []byte {
	t.Helper()
	buf := make([]byte, ed25519.PublicKeySize)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return buf
}

func mustJSONBytes(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func signedRequest(t *testing.T, priv ed25519.PrivateKey, botID, method, path, rawQuery string, body []byte, ts time.Time, nonce string) *http.Request {
	t.Helper()
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

func canonicalString(method, path, rawQuery, timestamp, nonce, bodyHash string) string {
	pathAndQuery := path
	if rawQuery != "" {
		pathAndQuery += "?" + rawQuery
	}
	return method + "\n" + pathAndQuery + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash
}

func httptestRequest(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
