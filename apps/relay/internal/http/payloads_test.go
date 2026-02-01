package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/domain"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

func TestPayloadFetchMarksFetched(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_1"
	jobID := "job_1"
	payloadID := "payload_1"

	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)
	seedJob(t, st, jobID, offerID, buyerID, sellerID, now)
	seedPayload(t, st, payloadID, jobID, sellerID, buyerID, payloadKindMessage, now, false)

	router := NewRouter(nil, st)
	req := httptest.NewRequest(http.MethodGet, "/v0/payloads/"+payloadID, nil)
	req.Header.Set(headerBotID, buyerID)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp payloadEnvelopeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PayloadID != payloadID || resp.JobID != jobID {
		t.Fatalf("unexpected payload response")
	}
	if resp.CiphertextB64 == "" {
		t.Fatalf("expected ciphertext")
	}

	stored, err := st.GetPayload(ctx, sqlc.GetPayloadParams{PayloadID: payloadID, RecipientBotID: buyerID})
	if err != nil {
		t.Fatalf("get payload: %v", err)
	}
	if !stored.FetchedAt.Valid {
		t.Fatalf("expected fetched_at to be set")
	}
}

func TestPayloadFetchForbidden(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_1"
	jobID := "job_1"
	payloadID := "payload_1"

	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)
	seedJob(t, st, jobID, offerID, buyerID, sellerID, now)
	seedPayload(t, st, payloadID, jobID, sellerID, buyerID, payloadKindMessage, now, false)

	router := NewRouter(nil, st)
	req := httptest.NewRequest(http.MethodGet, "/v0/payloads/"+payloadID, nil)
	req.Header.Set(headerBotID, sellerID)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	stored, err := st.GetPayload(ctx, sqlc.GetPayloadParams{PayloadID: payloadID, RecipientBotID: buyerID})
	if err != nil {
		t.Fatalf("get payload: %v", err)
	}
	if stored.FetchedAt.Valid {
		t.Fatalf("expected fetched_at unchanged")
	}
}

func TestPayloadListStatusAndCursor(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_1"
	jobID := "job_1"

	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)
	seedJob(t, st, jobID, offerID, buyerID, sellerID, now)

	seedPayload(t, st, "payload_old", jobID, sellerID, buyerID, payloadKindMessage, now.Add(-time.Minute), false)
	seedPayload(t, st, "payload_new", jobID, sellerID, buyerID, payloadKindMessage, now, true)

	router := NewRouter(nil, st)

	unfetchedReq := httptest.NewRequest(http.MethodGet, "/v0/payloads?status=unfetched", nil)
	unfetchedReq.Header.Set(headerBotID, buyerID)
	unfetchedRec := httptestRequest(t, router, unfetchedReq)
	if unfetchedRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", unfetchedRec.Code)
	}
	if strings.Contains(unfetchedRec.Body.String(), "ciphertext_b64") {
		t.Fatalf("metadata should not include ciphertext")
	}

	fetchedReq := httptest.NewRequest(http.MethodGet, "/v0/payloads?status=fetched", nil)
	fetchedReq.Header.Set(headerBotID, buyerID)
	fetchedRec := httptestRequest(t, router, fetchedReq)
	if fetchedRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", fetchedRec.Code)
	}

	allReq := httptest.NewRequest(http.MethodGet, "/v0/payloads?status=all&limit=1", nil)
	allReq.Header.Set(headerBotID, buyerID)
	allRec := httptestRequest(t, router, allReq)
	if allRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", allRec.Code)
	}

	var allResp payloadListResponse
	if err := json.Unmarshal(allRec.Body.Bytes(), &allResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(allResp.Payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(allResp.Payloads))
	}
	if allResp.NextCursor == "" {
		t.Fatalf("expected next_cursor")
	}

	cursorReq := httptest.NewRequest(http.MethodGet, "/v0/payloads?status=all&limit=1&cursor="+allResp.NextCursor, nil)
	cursorReq.Header.Set(headerBotID, buyerID)
	cursorRec := httptestRequest(t, router, cursorReq)
	if cursorRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", cursorRec.Code)
	}
	var cursorResp payloadListResponse
	if err := json.Unmarshal(cursorRec.Body.Bytes(), &cursorResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cursorResp.Payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(cursorResp.Payloads))
	}
}

func seedJob(t *testing.T, st *store.Store, jobID, offerID, buyerID, sellerID string, now time.Time) {
	t.Helper()
	err := st.CreateJob(context.Background(), sqlc.CreateJobParams{
		JobID:             jobID,
		OfferID:           offerID,
		BuyerBotID:        buyerID,
		SellerBotID:       sellerID,
		Status:            string(domain.JobRequested),
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		CreatedAt:         now,
		JobExpiresAt:      now.Add(48 * time.Hour),
		RequestPayloadID:  "request_payload",
		ChargeID:          sql.NullString{},
		ChargeAddress:     sql.NullString{},
		ChargeAmountRaw:   sql.NullString{},
		ChargeExpiresAt:   sql.NullTime{},
		ChargeSigEd25519:  sql.NullString{},
		PaidAt:            sql.NullTime{},
		DeliveredAt:       sql.NullTime{},
		CancelledAt:       sql.NullTime{},
		ExpiredAt:         sql.NullTime{},
		PaymentVerifier:   sql.NullString{},
		PaymentBlockHash:  sql.NullString{},
		PaymentObservedAt: sql.NullTime{},
		AmountRawReceived: sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
}

func seedPayload(t *testing.T, st *store.Store, payloadID, jobID, senderID, recipientID, kind string, createdAt time.Time, fetched bool) {
	t.Helper()
	fetchedAt := sql.NullTime{}
	if fetched {
		fetchedAt = sql.NullTime{Time: createdAt.Add(time.Minute), Valid: true}
	}
	ciphertext := base64.RawURLEncoding.EncodeToString([]byte("ciphertext"))
	err := st.CreatePayload(context.Background(), sqlc.CreatePayloadParams{
		PayloadID:      payloadID,
		RecipientBotID: recipientID,
		JobID:          jobID,
		SenderBotID:    senderID,
		PayloadKind:    kind,
		EncAlg:         encAlgSealBox,
		RecipientKid:   "kid",
		CiphertextB64:  ciphertext,
		CreatedAt:      createdAt,
		FetchedAt:      fetchedAt,
	})
	if err != nil {
		t.Fatalf("create payload: %v", err)
	}
}
