package httpapi

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

func TestPollBatchMultipleStreams(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	sellerPub := seedSignedBot(t, st, now)
	sellerID := botIDFromSigningKey(sellerPub)
	buyerPub := seedSignedBot(t, st, now)
	buyerID := botIDFromSigningKey(buyerPub)

	offerID := "offer_batch"
	jobID := "job_batch"
	seedOfferForStream(t, st, offerID, sellerID, now)
	seedJobForStream(t, st, jobID, offerID, buyerID, sellerID, now)

	sellerStream := "seller:ed25519:" + base64.RawURLEncoding.EncodeToString(sellerPub)
	jobStream := "job:" + jobID

	seedStreamEvent(t, st, sellerStream, "job.requested", map[string]any{"job_id": jobID}, now)
	lastSeller := seedStreamEvent(t, st, sellerStream, "job.paid", map[string]any{"job_id": jobID}, now.Add(time.Minute))
	lastJob := seedStreamEvent(t, st, jobStream, "job.requested", map[string]any{"job_id": jobID}, now)

	router := NewRouter(RouterConfig{Store: st})
	reqBody := batchPollRequest{
		Streams: []batchPollStream{
			{Stream: sellerStream, Since: 0},
			{Stream: jobStream, Since: 0},
		},
		Limit: 10,
	}
	req := newJSONRequest(t, http.MethodPost, "/v0/poll/batch", mustJSONBytes(t, reqBody))
	req.Header.Set(headerBotID, sellerID)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp batchPollResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.Results[0].Stream != sellerStream {
		t.Fatalf("expected seller stream first, got %q", resp.Results[0].Stream)
	}
	if resp.Results[0].Next != lastSeller {
		t.Fatalf("expected seller next %d, got %d", lastSeller, resp.Results[0].Next)
	}
	if resp.Results[1].Stream != jobStream {
		t.Fatalf("expected job stream second, got %q", resp.Results[1].Stream)
	}
	if resp.Results[1].Next != lastJob {
		t.Fatalf("expected job next %d, got %d", lastJob, resp.Results[1].Next)
	}
}

func TestPollBatchRejectsUnauthorizedStream(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	sellerPub := seedSignedBot(t, st, now)
	otherPub := seedSignedBot(t, st, now)
	otherID := botIDFromSigningKey(otherPub)

	streamKey := "seller:ed25519:" + base64.RawURLEncoding.EncodeToString(sellerPub)

	router := NewRouter(RouterConfig{Store: st})
	reqBody := batchPollRequest{
		Streams: []batchPollStream{{Stream: streamKey, Since: 0}},
		Limit:   10,
	}
	req := newJSONRequest(t, http.MethodPost, "/v0/poll/batch", mustJSONBytes(t, reqBody))
	req.Header.Set(headerBotID, otherID)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPollBatchRejectsJobStreamNotParticipant(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerPub := seedSignedBot(t, st, now)
	buyerID := botIDFromSigningKey(buyerPub)
	sellerPub := seedSignedBot(t, st, now)
	sellerID := botIDFromSigningKey(sellerPub)
	otherPub := seedSignedBot(t, st, now)
	otherID := botIDFromSigningKey(otherPub)

	offerID := "offer_job_participant"
	jobID := "job_participant"
	seedOfferForStream(t, st, offerID, sellerID, now)
	seedJobForStream(t, st, jobID, offerID, buyerID, sellerID, now)

	jobStream := "job:" + jobID

	router := NewRouter(RouterConfig{Store: st})
	reqBody := batchPollRequest{
		Streams: []batchPollStream{{Stream: jobStream, Since: 0}},
		Limit:   10,
	}
	req := newJSONRequest(t, http.MethodPost, "/v0/poll/batch", mustJSONBytes(t, reqBody))
	req.Header.Set(headerBotID, otherID)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPollBatchLimitHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	sellerPub := seedSignedBot(t, st, now)
	sellerID := botIDFromSigningKey(sellerPub)

	streamKey := "seller:ed25519:" + base64.RawURLEncoding.EncodeToString(sellerPub)
	first := seedStreamEvent(t, st, streamKey, "job.requested", map[string]any{"job_id": "job_1"}, now)
	_ = seedStreamEvent(t, st, streamKey, "job.paid", map[string]any{"job_id": "job_1"}, now.Add(time.Minute))
	_ = seedStreamEvent(t, st, streamKey, "job.cancelled", map[string]any{"job_id": "job_1"}, now.Add(2*time.Minute))

	router := NewRouter(RouterConfig{Store: st})
	reqBody := batchPollRequest{
		Streams: []batchPollStream{{Stream: streamKey, Since: 0}},
		Limit:   1,
	}
	req := newJSONRequest(t, http.MethodPost, "/v0/poll/batch", mustJSONBytes(t, reqBody))
	req.Header.Set(headerBotID, sellerID)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp batchPollResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if len(resp.Results[0].Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(resp.Results[0].Events))
	}
	if resp.Results[0].Next != first {
		t.Fatalf("expected next %d, got %d", first, resp.Results[0].Next)
	}
}

func TestPollBatchEmptyResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	sellerPub := seedSignedBot(t, st, now)
	sellerID := botIDFromSigningKey(sellerPub)
	streamKey := "seller:ed25519:" + base64.RawURLEncoding.EncodeToString(sellerPub)

	router := NewRouter(RouterConfig{Store: st})
	reqBody := batchPollRequest{
		Streams: []batchPollStream{{Stream: streamKey, Since: 0}},
		Limit:   10,
	}
	req := newJSONRequest(t, http.MethodPost, "/v0/poll/batch", mustJSONBytes(t, reqBody))
	req.Header.Set(headerBotID, sellerID)
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp batchPollResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if len(resp.Results[0].Events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(resp.Results[0].Events))
	}
	if resp.Results[0].Next != 0 {
		t.Fatalf("expected next 0, got %d", resp.Results[0].Next)
	}
}

func TestAckStreamMonotonic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	pub := seedSignedBot(t, st, now)
	botID := botIDFromSigningKey(pub)
	streamKey := "seller:ed25519:" + base64.RawURLEncoding.EncodeToString(pub)

	router := NewRouter(RouterConfig{Store: st})

	ackReq := newJSONRequest(t, http.MethodPost, "/v0/ack", mustJSONBytes(t, streamAckRequest{Stream: streamKey, Ack: 5}))
	ackReq.Header.Set(headerBotID, botID)
	ackRec := httptestRequest(t, router, ackReq)
	if ackRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ackRec.Code, ackRec.Body.String())
	}
	var ackResp streamAckResponse
	if err := json.Unmarshal(ackRec.Body.Bytes(), &ackResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ackResp.Ack != 5 {
		t.Fatalf("expected ack 5, got %d", ackResp.Ack)
	}

	ackReq2 := newJSONRequest(t, http.MethodPost, "/v0/ack", mustJSONBytes(t, streamAckRequest{Stream: streamKey, Ack: 3}))
	ackReq2.Header.Set(headerBotID, botID)
	ackRec2 := httptestRequest(t, router, ackReq2)
	if ackRec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ackRec2.Code, ackRec2.Body.String())
	}
	if err := json.Unmarshal(ackRec2.Body.Bytes(), &ackResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ackResp.Ack != 5 {
		t.Fatalf("expected ack to remain 5, got %d", ackResp.Ack)
	}

	ackReq3 := newJSONRequest(t, http.MethodPost, "/v0/ack", mustJSONBytes(t, streamAckRequest{Stream: streamKey, Ack: 7}))
	ackReq3.Header.Set(headerBotID, botID)
	ackRec3 := httptestRequest(t, router, ackReq3)
	if ackRec3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ackRec3.Code, ackRec3.Body.String())
	}
	if err := json.Unmarshal(ackRec3.Body.Bytes(), &ackResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ackResp.Ack != 7 {
		t.Fatalf("expected ack 7, got %d", ackResp.Ack)
	}
}

func TestAckStreamInvalidStream(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	pub := seedSignedBot(t, st, now)
	botID := botIDFromSigningKey(pub)

	router := NewRouter(RouterConfig{Store: st})
	ackReq := newJSONRequest(t, http.MethodPost, "/v0/ack", mustJSONBytes(t, streamAckRequest{Stream: "unknown:stream", Ack: 1}))
	ackReq.Header.Set(headerBotID, botID)
	ackRec := httptestRequest(t, router, ackReq)
	if ackRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", ackRec.Code, ackRec.Body.String())
	}
}

func seedSignedBot(t *testing.T, st *store.Store, now time.Time) ed25519.PublicKey {
	t.Helper()
	pub, _ := generateSigningKey(t)
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
	return pub
}

func seedOfferForStream(t *testing.T, st *store.Store, offerID, sellerID string, now time.Time) {
	t.Helper()
	if err := st.CreateOffer(context.Background(), sqlc.CreateOfferParams{
		OfferID:           offerID,
		SellerBotID:       sellerID,
		Title:             "title",
		Description:       "desc",
		TagsJson:          "[]",
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		CreatedAt:         now,
		ExpiresAt:         sql.NullTime{},
		Status:            "ACTIVE",
		CancelledAt:       sql.NullTime{},
		RequestSchemaHint: sql.NullString{},
	}); err != nil {
		t.Fatalf("create offer: %v", err)
	}
}

func seedJobForStream(t *testing.T, st *store.Store, jobID, offerID, buyerID, sellerID string, now time.Time) {
	t.Helper()
	if err := st.CreateJob(context.Background(), sqlc.CreateJobParams{
		JobID:             jobID,
		OfferID:           offerID,
		BuyerBotID:        buyerID,
		SellerBotID:       sellerID,
		Status:            "REQUESTED",
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		CreatedAt:         now,
		JobExpiresAt:      now.Add(24 * time.Hour),
		RequestPayloadID:  "payload_" + jobID,
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
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
}

func seedStreamEvent(t *testing.T, st *store.Store, streamKey, eventType string, data map[string]any, createdAt time.Time) int64 {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cursor, err := st.CreateStreamEvent(context.Background(), sqlc.CreateStreamEventParams{
		StreamKey:   streamKey,
		EventType:   eventType,
		CreatedAt:   createdAt,
		PayloadJson: string(payload),
	})
	if err != nil {
		t.Fatalf("create stream event: %v", err)
	}
	return cursor
}
