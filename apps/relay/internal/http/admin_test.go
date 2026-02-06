package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/store"
)

func TestAdminAuth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	router := NewAdminRouter(AdminRouterConfig{
		Store:      st,
		AdminToken: "secret-token",
	})

	req := newJSONRequest(t, http.MethodGet, "/admin/overview", nil)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}

	req = newJSONRequest(t, http.MethodGet, "/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec = httptestRequest(t, router, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}

	req = newJSONRequest(t, http.MethodGet, "/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	req = newJSONRequest(t, http.MethodGet, "/admin/meta", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var meta struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.Mode != "separate_listener" {
		t.Fatalf("expected separate_listener, got %q", meta.Mode)
	}
}

func TestAdminRevokeBotCreatesAudit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC)
	seedJobBot(t, st, "bot_test", now)

	router := NewAdminRouter(AdminRouterConfig{
		Store:      st,
		AdminToken: "secret-token",
		StreamHub:  NewStreamHub(st),
	})

	body := mustJSONBytes(t, map[string]any{"reason": "moderation"})
	req := newJSONRequest(t, http.MethodPost, "/admin/bots/bot_test/revoke", body)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		AuditID int64 `json:"audit_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.AuditID <= 0 {
		t.Fatalf("expected audit_id > 0, got %d", resp.AuditID)
	}

	var revokedAt sql.NullTime
	if err := st.DB.QueryRow(`SELECT revoked_at FROM bots WHERE bot_id = ?1`, "bot_test").Scan(&revokedAt); err != nil {
		t.Fatalf("query bot revoked_at: %v", err)
	}
	if !revokedAt.Valid {
		t.Fatalf("expected bot revoked_at set")
	}

	var count int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM admin_audit_log WHERE id = ?1 AND action = 'bot.revoke' AND target_type = 'bot' AND target_id = 'bot_test'`, resp.AuditID).Scan(&count); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected audit row, got %d", count)
	}
}

func TestAdminPublicMountOnMainRouter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)

	router := NewRouter(RouterConfig{
		Verifier:    verifier,
		Store:       st,
		AdminPublic: true,
		AdminToken:  "secret-token",
	})

	req := newJSONRequest(t, http.MethodGet, "/admin/overview", nil)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}

	req = newJSONRequest(t, http.MethodGet, "/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	req = newJSONRequest(t, http.MethodGet, "/admin/meta", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var meta struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.Mode != "public_mount" {
		t.Fatalf("expected public_mount, got %q", meta.Mode)
	}
}
