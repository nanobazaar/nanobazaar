package httpapi

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/domain"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

func TestOffersCreateGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	payload := offerCreateRequest{
		Title:             "Nano summary",
		Description:       "Summarize a Nano paper",
		Tags:              []string{"nano", "summary"},
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
	}
	body := mustJSONBytes(t, payload)

	req := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", body, now, "nonce-1")
	req.Header.Set(headerIdempotency, "idem-1")

	rec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp offerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OfferID == "" {
		t.Fatalf("expected offer_id set")
	}
	if resp.SellerBotID != botID {
		t.Fatalf("expected seller_bot_id %q, got %q", botID, resp.SellerBotID)
	}
	if resp.Status != string(domain.OfferActive) {
		t.Fatalf("expected status ACTIVE, got %q", resp.Status)
	}
	if resp.ExpiresAt == nil {
		t.Fatalf("expected expires_at set")
	}

	getReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers/"+resp.OfferID, "", nil, now, "nonce-2")
	getRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var getResp offerResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal get response: %v", err)
	}
	if getResp.OfferID != resp.OfferID {
		t.Fatalf("expected offer_id %q, got %q", resp.OfferID, getResp.OfferID)
	}
}

func TestOffersCreateRejectsLargeSchemaHint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	bigHint := strings.Repeat("a", maxRequestSchemaHintBytes+1)
	payload := offerCreateRequest{
		Title:             "Nano summary",
		Description:       "Summarize a Nano paper",
		Tags:              []string{"nano", "summary"},
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		RequestSchemaHint: bigHint,
	}
	body := mustJSONBytes(t, payload)

	req := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers", "", body, now, "nonce-1")
	req.Header.Set(headerIdempotency, "idem-1")

	rec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOffersCancel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	offerID := createOfferForTest(t, store, botID, "offer_cancel", now.Add(-time.Hour))

	cancelReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers/"+offerID+"/cancel", "", []byte(`{}`), now, "nonce-1")
	cancelReq.Header.Set(headerIdempotency, "idem-1")
	cancelRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}

	var cancelResp offerResponse
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &cancelResp); err != nil {
		t.Fatalf("unmarshal cancel response: %v", err)
	}
	if cancelResp.Status != string(domain.OfferCancelled) {
		t.Fatalf("expected status CANCELLED, got %q", cancelResp.Status)
	}
	if cancelResp.CancelledAt == nil {
		t.Fatalf("expected cancelled_at set")
	}

	secondReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers/"+offerID+"/cancel", "", []byte(`{}`), now, "nonce-2")
	secondReq.Header.Set(headerIdempotency, "idem-2")
	secondRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
}

func TestOffersCancelForbidden(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pubA, _ := generateSigningKey(t)
	botA := seedBotWithKey(t, store, pubA)
	pubB, privB := generateSigningKey(t)
	botB := seedBotWithKey(t, store, pubB)

	offerID := createOfferForTest(t, store, botA, "offer_forbidden", now.Add(-time.Hour))

	cancelReq := signedRequest(t, privB, botB, http.MethodPost, "/v0/offers/"+offerID+"/cancel", "", []byte(`{}`), now, "nonce-1")
	cancelReq.Header.Set(headerIdempotency, "idem-1")

	cancelRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), cancelReq)
	if cancelRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
}

func TestOffersCancelExpired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	expiredAt := now.Add(-time.Hour)
	offerID := insertOfferWithExpiry(t, store, botID, "offer_expired", now.Add(-2*time.Hour), expiredAt)

	cancelReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers/"+offerID+"/cancel", "", []byte(`{}`), now, "nonce-1")
	cancelReq.Header.Set(headerIdempotency, "idem-1")
	cancelRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), cancelReq)
	if cancelRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
}

func TestOffersListPagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	base := now.Add(-time.Hour)
	insertOfferWithExpiry(t, store, botID, "offer_b", base, now.Add(24*time.Hour))
	insertOfferWithExpiry(t, store, botID, "offer_a", base, now.Add(24*time.Hour))
	insertOfferWithExpiry(t, store, botID, "offer_c", base.Add(-time.Hour), now.Add(24*time.Hour))

	listReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers", "limit=2", nil, now, "nonce-1")
	listRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp offerListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResp.Offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(listResp.Offers))
	}
	if listResp.Offers[0].OfferID != "offer_b" || listResp.Offers[1].OfferID != "offer_a" {
		t.Fatalf("unexpected order: %v, %v", listResp.Offers[0].OfferID, listResp.Offers[1].OfferID)
	}
	if listResp.NextCursor == "" {
		t.Fatalf("expected next_cursor set")
	}

	secondReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers", "cursor="+listResp.NextCursor, nil, now, "nonce-2")
	secondRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
	var secondResp offerListResponse
	if err := json.Unmarshal(secondRec.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("unmarshal second response: %v", err)
	}
	if len(secondResp.Offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(secondResp.Offers))
	}
	if secondResp.Offers[0].OfferID != "offer_c" {
		t.Fatalf("unexpected offer_id %q", secondResp.Offers[0].OfferID)
	}
}

func TestOffersListRelevance(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	insertOfferWithExpiry(t, store, botID, "offer_title", now.Add(-time.Hour), now.Add(24*time.Hour))
	insertOfferWithExpiry(t, store, botID, "offer_desc", now.Add(-time.Hour), now.Add(24*time.Hour))
	insertOfferWithExpiry(t, store, botID, "offer_none", now.Add(-time.Hour), now.Add(24*time.Hour))

	_, _ = store.DB.Exec(`UPDATE offers SET title = 'Nano summary' WHERE offer_id = 'offer_title'`)
	_, _ = store.DB.Exec(`UPDATE offers SET title = 'Other', description = 'nano in description' WHERE offer_id = 'offer_desc'`)
	_, _ = store.DB.Exec(`UPDATE offers SET title = 'No match', description = 'nothing', tags_json = '[]' WHERE offer_id = 'offer_none'`)

	listReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers", "q=nano&sort=relevance", nil, now, "nonce-1")
	listRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp offerListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResp.Offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(listResp.Offers))
	}
	if listResp.Offers[0].OfferID != "offer_title" {
		t.Fatalf("expected offer_title first, got %q", listResp.Offers[0].OfferID)
	}
}

func seedBotWithKey(t *testing.T, store *store.Store, pub ed25519.PublicKey) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	encryptionPub := randomKeyBytes(t)
	botID := botIDFromSigningKey(pub)
	if err := store.CreateBot(context.Background(), sqlc.CreateBotParams{
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
	return botID
}

func createOfferForTest(t *testing.T, store *store.Store, botID, offerID string, createdAt time.Time) string {
	t.Helper()
	return insertOfferWithExpiry(t, store, botID, offerID, createdAt, createdAt.Add(24*time.Hour))
}

func insertOfferWithExpiry(t *testing.T, store *store.Store, botID, offerID string, createdAt, expiresAt time.Time) string {
	t.Helper()
	if err := store.CreateOffer(context.Background(), sqlc.CreateOfferParams{
		OfferID:           offerID,
		SellerBotID:       botID,
		Title:             "Test offer",
		Description:       "Test description",
		TagsJson:          string(mustJSONBytes(t, []string{"nano"})),
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		CreatedAt:         createdAt,
		ExpiresAt:         sql.NullTime{Time: expiresAt, Valid: true},
		Status:            string(domain.OfferActive),
		CancelledAt:       sql.NullTime{},
		RequestSchemaHint: sql.NullString{},
	}); err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := store.InsertOfferTag(context.Background(), sqlc.InsertOfferTagParams{
		OfferID: offerID,
		Tag:     "nano",
	}); err != nil {
		t.Fatalf("insert offer tag: %v", err)
	}
	return offerID
}
