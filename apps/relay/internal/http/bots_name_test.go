package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/store"
)

func TestBotsSetNameAndClear(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
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
	rec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	setReqBody := mustJSONBytes(t, map[string]string{"bot_name": "Alice"})
	setReq := signedRequest(t, priv, botID, httpMethodPost, "/v0/bots/"+botID+"/name", "", setReqBody, now, "nonce-2")
	setReq.Header.Set(headerIdempotency, "idem-2")
	setRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), setReq)
	if setRec.Code != http.StatusOK {
		t.Fatalf("set name expected 200, got %d: %s", setRec.Code, setRec.Body.String())
	}

	var setResp botResponse
	if err := json.Unmarshal(setRec.Body.Bytes(), &setResp); err != nil {
		t.Fatalf("unmarshal set response: %v", err)
	}
	if setResp.BotName != "Alice" {
		t.Fatalf("expected bot_name %q, got %q", "Alice", setResp.BotName)
	}

	getReq := signedRequest(t, priv, botID, httpMethodGet, "/v0/bots/"+botID, "", nil, now, "nonce-3")
	getRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var getResp botResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal get response: %v", err)
	}
	if getResp.BotName != "Alice" {
		t.Fatalf("expected bot_name %q, got %q", "Alice", getResp.BotName)
	}

	clearReqBody := mustJSONBytes(t, map[string]string{"bot_name": "   "})
	clearReq := signedRequest(t, priv, botID, httpMethodPost, "/v0/bots/"+botID+"/name", "", clearReqBody, now, "nonce-4")
	clearReq.Header.Set(headerIdempotency, "idem-3")
	clearRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), clearReq)
	if clearRec.Code != http.StatusOK {
		t.Fatalf("clear name expected 200, got %d: %s", clearRec.Code, clearRec.Body.String())
	}

	stored, err := st.GetBot(context.Background(), botID)
	if err != nil {
		t.Fatalf("get bot: %v", err)
	}
	if stored.BotName.Valid {
		t.Fatalf("expected bot_name cleared")
	}
}
