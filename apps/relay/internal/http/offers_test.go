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

func TestOffersIncludeSellerBotNameWhenSet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	if _, err := store.UpdateBotName(context.Background(), sqlc.UpdateBotNameParams{
		BotName: sql.NullString{String: "Alice", Valid: true},
		BotID:   botID,
	}); err != nil {
		t.Fatalf("update bot name: %v", err)
	}

	offerID := createOfferForTest(t, store, botID, "offer_named", now.Add(-time.Hour))

	getReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers/"+offerID, "", nil, now, "nonce-1")
	getRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var getResp offerResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal get response: %v", err)
	}
	if getResp.SellerBotName != "Alice" {
		t.Fatalf("expected seller_bot_name %q, got %q", "Alice", getResp.SellerBotName)
	}

	listReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers", "sort=newest", nil, now, "nonce-2")
	listRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp offerListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResp.Offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(listResp.Offers))
	}
	if listResp.Offers[0].SellerBotName != "Alice" {
		t.Fatalf("expected seller_bot_name %q, got %q", "Alice", listResp.Offers[0].SellerBotName)
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

func TestOffersPauseResume(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Now().UTC().Truncate(time.Second)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	offerID := createOfferForTest(t, store, botID, "offer_pause", now.Add(-time.Hour))

	pauseReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers/"+offerID+"/pause", "", []byte(`{}`), now, "nonce-1")
	pauseReq.Header.Set(headerIdempotency, "idem-1")
	pauseRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), pauseReq)
	if pauseRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", pauseRec.Code, pauseRec.Body.String())
	}

	var pauseResp offerResponse
	if err := json.Unmarshal(pauseRec.Body.Bytes(), &pauseResp); err != nil {
		t.Fatalf("unmarshal pause response: %v", err)
	}
	if pauseResp.Status != string(domain.OfferPaused) {
		t.Fatalf("expected status PAUSED, got %q", pauseResp.Status)
	}

	secondPauseReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers/"+offerID+"/pause", "", []byte(`{}`), now, "nonce-2")
	secondPauseReq.Header.Set(headerIdempotency, "idem-2")
	secondPauseRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), secondPauseReq)
	if secondPauseRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", secondPauseRec.Code, secondPauseRec.Body.String())
	}

	var secondPauseResp offerResponse
	if err := json.Unmarshal(secondPauseRec.Body.Bytes(), &secondPauseResp); err != nil {
		t.Fatalf("unmarshal pause response: %v", err)
	}
	if secondPauseResp.Status != string(domain.OfferPaused) {
		t.Fatalf("expected status PAUSED, got %q", secondPauseResp.Status)
	}

	resumeReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers/"+offerID+"/resume", "", []byte(`{}`), now, "nonce-3")
	resumeReq.Header.Set(headerIdempotency, "idem-3")
	resumeRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), resumeReq)
	if resumeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resumeRec.Code, resumeRec.Body.String())
	}

	var resumeResp offerResponse
	if err := json.Unmarshal(resumeRec.Body.Bytes(), &resumeResp); err != nil {
		t.Fatalf("unmarshal resume response: %v", err)
	}
	if resumeResp.Status != string(domain.OfferActive) {
		t.Fatalf("expected status ACTIVE, got %q", resumeResp.Status)
	}

	secondResumeReq := signedRequest(t, priv, botID, http.MethodPost, "/v0/offers/"+offerID+"/resume", "", []byte(`{}`), now, "nonce-4")
	secondResumeReq.Header.Set(headerIdempotency, "idem-4")
	secondResumeRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), secondResumeReq)
	if secondResumeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", secondResumeRec.Code, secondResumeRec.Body.String())
	}

	var secondResumeResp offerResponse
	if err := json.Unmarshal(secondResumeRec.Body.Bytes(), &secondResumeResp); err != nil {
		t.Fatalf("unmarshal resume response: %v", err)
	}
	if secondResumeResp.Status != string(domain.OfferActive) {
		t.Fatalf("expected status ACTIVE, got %q", secondResumeResp.Status)
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

func TestOffersListIncludePaused(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	verifier := auth.NewVerifier(store)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, store, pub)

	activeID := insertOfferWithExpiry(t, store, botID, "offer_active", now.Add(-time.Hour), now.Add(24*time.Hour))
	pausedID := insertOfferWithExpiry(t, store, botID, "offer_paused", now.Add(-2*time.Hour), now.Add(24*time.Hour))
	if err := store.UpdateOfferPause(context.Background(), pausedID); err != nil {
		t.Fatalf("pause offer: %v", err)
	}

	listReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers", "", nil, now, "nonce-1")
	listRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp offerListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResp.Offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(listResp.Offers))
	}
	if listResp.Offers[0].OfferID != activeID {
		t.Fatalf("expected offer_id %q, got %q", activeID, listResp.Offers[0].OfferID)
	}

	includeReq := signedRequest(t, priv, botID, http.MethodGet, "/v0/offers", "include_paused=true", nil, now, "nonce-2")
	includeRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: store}), includeReq)
	if includeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", includeRec.Code, includeRec.Body.String())
	}

	var includeResp offerListResponse
	if err := json.Unmarshal(includeRec.Body.Bytes(), &includeResp); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(includeResp.Offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(includeResp.Offers))
	}
	seen := make(map[string]struct{}, len(includeResp.Offers))
	for _, offer := range includeResp.Offers {
		seen[offer.OfferID] = struct{}{}
	}
	if _, ok := seen[activeID]; !ok {
		t.Fatalf("expected offer_id %q in response", activeID)
	}
	if _, ok := seen[pausedID]; !ok {
		t.Fatalf("expected offer_id %q in response", pausedID)
	}
}

func TestPublicOffersMostPurchased(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	seedJobBot(t, store, buyerID, now)
	seedJobBot(t, store, sellerID, now)

	seedJobOffer(t, store, "offer_a", sellerID, now)
	seedJobOffer(t, store, "offer_b", sellerID, now)

	seedJobWithStatus(t, store, "job_a1", "offer_a", buyerID, sellerID, now, string(domain.JobPaid), "1000")
	seedJobWithStatus(t, store, "job_a2", "offer_a", buyerID, sellerID, now, string(domain.JobDelivered), "1000")
	seedJobWithStatus(t, store, "job_b1", "offer_b", buyerID, sellerID, now, string(domain.JobPaid), "1000")

	req := newJSONRequest(t, http.MethodGet, "/market/offers?sort=most_purchased", nil)
	rec := httptestRequest(t, NewRouter(RouterConfig{Store: store}), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp publicOfferListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(resp.Offers))
	}
	if resp.Offers[0].OfferID != "offer_a" {
		t.Fatalf("expected offer_a first, got %q", resp.Offers[0].OfferID)
	}
	if resp.Offers[0].PurchaseCount != 2 {
		t.Fatalf("expected offer_a purchase_count 2, got %d", resp.Offers[0].PurchaseCount)
	}
	if resp.Offers[0].TurnaroundSeconds != 3600 {
		t.Fatalf("expected offer_a turnaround_seconds 3600, got %d", resp.Offers[0].TurnaroundSeconds)
	}
	if resp.Offers[1].OfferID != "offer_b" {
		t.Fatalf("expected offer_b second, got %q", resp.Offers[1].OfferID)
	}
	if resp.Offers[1].PurchaseCount != 1 {
		t.Fatalf("expected offer_b purchase_count 1, got %d", resp.Offers[1].PurchaseCount)
	}
	if resp.Offers[1].TurnaroundSeconds != 3600 {
		t.Fatalf("expected offer_b turnaround_seconds 3600, got %d", resp.Offers[1].TurnaroundSeconds)
	}
}

func TestPublicOfferGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)

	if _, err := st.DB.Exec(`UPDATE bots SET bot_name = ?1 WHERE bot_id = ?2`, "Seller", sellerID); err != nil {
		t.Fatalf("update bot_name: %v", err)
	}

	seedJobOffer(t, st, "offer_a", sellerID, now)
	if _, err := st.DB.Exec(`UPDATE offers SET tags_json = ?1, request_schema_hint = ?2 WHERE offer_id = ?3`, `["nano","writing"]`, "input hint", "offer_a"); err != nil {
		t.Fatalf("update offer fields: %v", err)
	}

	seedJobWithStatus(t, st, "job_a1", "offer_a", buyerID, sellerID, now, string(domain.JobPaid), "1000")
	seedJobWithStatus(t, st, "job_a2", "offer_a", buyerID, sellerID, now, string(domain.JobDelivered), "1000")

	req := newJSONRequest(t, http.MethodGet, "/market/offers/offer_a", nil)
	rec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp publicOfferResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OfferID != "offer_a" {
		t.Fatalf("expected offer_id offer_a, got %q", resp.OfferID)
	}
	if resp.SellerBotName != "Seller" {
		t.Fatalf("expected seller_bot_name Seller, got %q", resp.SellerBotName)
	}
	if resp.PurchaseCount != 2 {
		t.Fatalf("expected purchase_count 2, got %d", resp.PurchaseCount)
	}
	if resp.RequestSchemaHint != "input hint" {
		t.Fatalf("expected request_schema_hint set, got %q", resp.RequestSchemaHint)
	}
	if len(resp.Tags) != 2 || resp.Tags[0] != "nano" || resp.Tags[1] != "writing" {
		t.Fatalf("unexpected tags: %#v", resp.Tags)
	}
}

func TestPublicOfferGetNotFoundWhenPausedOrExpired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	sellerID := "bot_seller"
	seedJobBot(t, st, sellerID, now)

	seedJobOffer(t, st, "offer_paused", sellerID, now)
	if _, err := st.DB.Exec(`UPDATE offers SET status = 'PAUSED' WHERE offer_id = 'offer_paused'`); err != nil {
		t.Fatalf("pause offer: %v", err)
	}

	reqPaused := newJSONRequest(t, http.MethodGet, "/market/offers/offer_paused", nil)
	recPaused := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), reqPaused)
	if recPaused.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recPaused.Code, recPaused.Body.String())
	}

	insertOfferWithExpiry(t, st, sellerID, "offer_expired", now.Add(-2*time.Hour), now.Add(-time.Hour))
	reqExpired := newJSONRequest(t, http.MethodGet, "/market/offers/offer_expired", nil)
	recExpired := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), reqExpired)
	if recExpired.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recExpired.Code, recExpired.Body.String())
	}

	offer, err := st.GetOffer(context.Background(), "offer_expired")
	if err != nil {
		t.Fatalf("get offer: %v", err)
	}
	if offer.Status != string(domain.OfferExpired) {
		t.Fatalf("expected offer status EXPIRED, got %q", offer.Status)
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
