package httpapi

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

func TestStreamWakeup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	encryptionPub := randomKeyBytes(t)
	botID := botIDFromSigningKey(pub)

	if err := st.CreateBot(context.Background(), sqlc.CreateBotParams{
		BotID:                  botID,
		SigningPubkeyEd25519:   base64.RawURLEncoding.EncodeToString(pub),
		EncryptionPubkeyX25519: base64.RawURLEncoding.EncodeToString(encryptionPub),
		SigningKid:             kidFromKey(pub),
		EncryptionKid:          kidFromKey(encryptionPub),
		CreatedAt:              now,
		LastSeenAt:             sql.NullTime{Time: now, Valid: true},
	}); err != nil {
		t.Fatalf("create bot: %v", err)
	}

	hub := NewStreamHub()
	router := NewRouter(RouterConfig{Verifier: verifier, Store: st, StreamHub: hub})
	srv := httptest.NewServer(router)
	defer srv.Close()

	rawQuery := ""
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v0/stream", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	bodyHash := sha256Hex(nil)
	canonical := canonicalString(http.MethodGet, "/v0/stream", rawQuery, now.Format(time.RFC3339), "nonce-1", bodyHash)
	sig := ed25519.Sign(priv, []byte(canonical))
	setAuthHeaders(req, botID, now, "nonce-1", bodyHash, sig)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}

	reader := bufio.NewReader(resp.Body)

	// Wait for the initial connected frame so we know the connection is registered.
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(line, "connected") {
		t.Fatalf("expected connected frame, got %q", line)
	}

	// Trigger a wakeup.
	hub.NotifyEvent(context.Background(), botID, "test.wakeup", nil)

	var gotWake bool
	var gotData string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		l, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		l = strings.TrimSpace(l)
		if l == "event: wake" {
			gotWake = true
			continue
		}
		if gotWake && strings.HasPrefix(l, "data: ") {
			gotData = strings.TrimPrefix(l, "data: ")
			break
		}
	}
	if !gotWake || gotData == "" {
		t.Fatalf("expected wake event with data, gotWake=%v gotData=%q", gotWake, gotData)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(gotData), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["hint"] != "poll" {
		t.Fatalf("expected hint=poll, got %v", payload["hint"])
	}
	if _, ok := payload["streams"]; ok {
		t.Fatalf("expected no streams in wake payload, got %v", payload["streams"])
	}
}

func TestStreamRejectsStreamsParam(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	encryptionPub := randomKeyBytes(t)
	botID := botIDFromSigningKey(pub)

	if err := st.CreateBot(context.Background(), sqlc.CreateBotParams{
		BotID:                  botID,
		SigningPubkeyEd25519:   base64.RawURLEncoding.EncodeToString(pub),
		EncryptionPubkeyX25519: base64.RawURLEncoding.EncodeToString(encryptionPub),
		SigningKid:             kidFromKey(pub),
		EncryptionKid:          kidFromKey(encryptionPub),
		CreatedAt:              now,
		LastSeenAt:             sql.NullTime{Time: now, Valid: true},
	}); err != nil {
		t.Fatalf("create bot: %v", err)
	}

	hub := NewStreamHub()
	router := NewRouter(RouterConfig{Verifier: verifier, Store: st, StreamHub: hub})
	srv := httptest.NewServer(router)
	defer srv.Close()

	rawQuery := "streams=job:ignored"
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v0/stream?"+rawQuery, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	bodyHash := sha256Hex(nil)
	canonical := canonicalString(http.MethodGet, "/v0/stream", rawQuery, now.Format(time.RFC3339), "nonce-1", bodyHash)
	sig := ed25519.Sign(priv, []byte(canonical))
	setAuthHeaders(req, botID, now, "nonce-1", bodyHash, sig)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
