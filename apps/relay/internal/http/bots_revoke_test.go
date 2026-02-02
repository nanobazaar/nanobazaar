package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/store"
)

func TestBotsRevokeIdempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, st, pub)

	path := "/v0/bots/" + botID + "/revoke"
	req1 := signedRequest(t, priv, botID, http.MethodPost, path, "", nil, now, "nonce-1")
	req1.Header.Set(headerIdempotency, "idem-1")
	rec1 := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	var resp1 botRevokeResponse
	if err := json.Unmarshal(rec1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp1.BotID != botID {
		t.Fatalf("expected bot_id %q, got %q", botID, resp1.BotID)
	}
	if !resp1.Revoked {
		t.Fatalf("expected revoked true")
	}
	if resp1.RevokedAt.IsZero() {
		t.Fatalf("expected revoked_at set")
	}

	req2 := signedRequest(t, priv, botID, http.MethodPost, path, "", nil, now, "nonce-2")
	req2.Header.Set(headerIdempotency, "idem-2")
	rec2 := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp2 botRevokeResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp2.RevokedAt.Equal(resp1.RevokedAt) {
		t.Fatalf("expected revoked_at to remain %s, got %s", resp1.RevokedAt.Format(time.RFC3339Nano), resp2.RevokedAt.Format(time.RFC3339Nano))
	}
}

func TestRevokedBotForbidden(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, st, pub)

	revokePath := "/v0/bots/" + botID + "/revoke"
	revokeReq := signedRequest(t, priv, botID, http.MethodPost, revokePath, "", nil, now, "nonce-1")
	revokeReq.Header.Set(headerIdempotency, "idem-1")
	revokeRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}

	offerReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", mustJSONBytes(t, offerCreateRequest{
		Title:             "Revoked offer",
		Description:       "Should be blocked",
		Tags:              []string{"nano"},
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
	}), now, "nonce-2")
	offerReq.Header.Set(headerIdempotency, "idem-2")
	offerRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), offerReq)
	if offerRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", offerRec.Code, offerRec.Body.String())
	}

	pollReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/poll", "", nil, now, "nonce-3")
	pollRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), pollReq)
	if pollRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", pollRec.Code, pollRec.Body.String())
	}
}
