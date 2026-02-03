package httpapi

import (
	"bytes"
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

func TestJobLifecycleChargeDeliver(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_1"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))

	createPayload := payloadEnvelopeInput{
		PayloadID:     "pay_req_1",
		PayloadKind:   payloadKindRequest,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_seller",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("request")),
	}
	createReq := jobCreateRequest{
		JobID:          "job_1",
		OfferID:        offerID,
		RequestPayload: createPayload,
	}

	create := newJSONRequest(t, http.MethodPost, "/v0/jobs", mustJSONBytes(t, createReq))
	create.Header.Set(headerBotID, buyerID)
	createRec := httptestRequest(t, router, create)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createRec.Code, createRec.Body.String())
	}

	job, err := st.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != string(domain.JobRequested) {
		t.Fatalf("expected status REQUESTED, got %s", job.Status)
	}

	chargeReq := chargeCreateRequest{
		ChargeID:        "chg_1",
		Address:         "nano_addr",
		AmountRaw:       "1000",
		ChargeExpiresAt: now.Add(2 * time.Hour).Format(time.RFC3339),
		ChargeSig:       "sig",
	}
	charge := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_1/charge", mustJSONBytes(t, chargeReq))
	charge.Header.Set(headerBotID, sellerID)
	chargeRec := httptestRequest(t, router, charge)
	if chargeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", chargeRec.Code, chargeRec.Body.String())
	}

	cancel := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_1/cancel", nil)
	cancel.Header.Set(headerBotID, buyerID)
	cancelRec := httptestRequest(t, router, cancel)
	if cancelRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}

	markPaid := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_1/mark_paid", nil)
	markPaid.Header.Set(headerBotID, sellerID)
	markPaidRec := httptestRequest(t, router, markPaid)
	if markPaidRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", markPaidRec.Code, markPaidRec.Body.String())
	}

	deliverPayload := payloadEnvelopeInput{
		PayloadID:     "pay_deliver_1",
		PayloadKind:   payloadKindDeliver,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_buyer",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("deliver")),
	}
	deliverReq := deliverRequest{Payload: deliverPayload}
	deliver := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_1/deliver", mustJSONBytes(t, deliverReq))
	deliver.Header.Set(headerBotID, sellerID)
	deliverRec := httptestRequest(t, router, deliver)
	if deliverRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deliverRec.Code, deliverRec.Body.String())
	}

	finalJob, err := st.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if finalJob.Status != string(domain.JobDelivered) {
		t.Fatalf("expected status DELIVERED, got %s", finalJob.Status)
	}
}

func TestJobMarkPaidChargeExpired(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_2"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))

	createPayload := payloadEnvelopeInput{
		PayloadID:     "pay_req_2",
		PayloadKind:   payloadKindRequest,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_seller",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("request")),
	}
	createReq := jobCreateRequest{
		JobID:          "job_2",
		OfferID:        offerID,
		RequestPayload: createPayload,
	}
	create := newJSONRequest(t, http.MethodPost, "/v0/jobs", mustJSONBytes(t, createReq))
	create.Header.Set(headerBotID, buyerID)
	createRec := httptestRequest(t, router, create)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createRec.Code, createRec.Body.String())
	}

	chargeReq := chargeCreateRequest{
		ChargeID:        "chg_2",
		Address:         "nano_addr_2",
		AmountRaw:       "1000",
		ChargeExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339),
		ChargeSig:       "sig",
	}
	charge := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_2/charge", mustJSONBytes(t, chargeReq))
	charge.Header.Set(headerBotID, sellerID)
	chargeRec := httptestRequest(t, router, charge)
	if chargeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", chargeRec.Code, chargeRec.Body.String())
	}

	now = now.Add(2 * time.Hour)
	markPaid := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_2/mark_paid", nil)
	markPaid.Header.Set(headerBotID, sellerID)
	markPaidRec := httptestRequest(t, router, markPaid)
	if markPaidRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", markPaidRec.Code, markPaidRec.Body.String())
	}

	job, err := st.GetJob(ctx, "job_2")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != string(domain.JobExpired) {
		t.Fatalf("expected status EXPIRED, got %s", job.Status)
	}

	buyerEvents, err := st.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
		RecipientBotID: buyerID,
		SinceEventID:   0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list buyer events: %v", err)
	}
	if !containsEventType(buyerEvents, jobExpiredEventType) {
		t.Fatalf("expected job.expired event for buyer")
	}

	sellerEvents, err := st.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
		RecipientBotID: sellerID,
		SinceEventID:   0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list seller events: %v", err)
	}
	if !containsEventType(sellerEvents, jobExpiredEventType) {
		t.Fatalf("expected job.expired event for seller")
	}
}

func TestJobChargeExpiresAtPreservesMilliseconds(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_ms"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))

	createPayload := payloadEnvelopeInput{
		PayloadID:     "pay_req_ms",
		PayloadKind:   payloadKindRequest,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_seller",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("request")),
	}
	createReq := jobCreateRequest{
		JobID:          "job_ms",
		OfferID:        offerID,
		RequestPayload: createPayload,
	}
	create := newJSONRequest(t, http.MethodPost, "/v0/jobs", mustJSONBytes(t, createReq))
	create.Header.Set(headerBotID, buyerID)
	createRec := httptestRequest(t, router, create)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createRec.Code, createRec.Body.String())
	}

	expiresAt := now.Add(2 * time.Hour).Add(349 * time.Millisecond)
	expiresAtStr := expiresAt.UTC().Format(time.RFC3339Nano)
	chargeReq := chargeCreateRequest{
		ChargeID:        "chg_ms",
		Address:         "nano_addr_ms",
		AmountRaw:       "1000",
		ChargeExpiresAt: expiresAtStr,
		ChargeSig:       "sig",
	}
	charge := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_ms/charge", mustJSONBytes(t, chargeReq))
	charge.Header.Set(headerBotID, sellerID)
	chargeRec := httptestRequest(t, router, charge)
	if chargeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", chargeRec.Code, chargeRec.Body.String())
	}

	var resp jobResponse
	if err := json.Unmarshal(chargeRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Charge == nil {
		t.Fatalf("expected charge in response")
	}
	if resp.Charge.ChargeExpiresAt != expiresAtStr {
		t.Fatalf("expected charge_expires_at %q, got %q", expiresAtStr, resp.Charge.ChargeExpiresAt)
	}

	events, err := st.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
		RecipientBotID: buyerID,
		SinceEventID:   0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var chargeEvent *sqlc.Event
	for i := range events {
		if events[i].EventType == jobChargeCreatedEventType {
			chargeEvent = &events[i]
			break
		}
	}
	if chargeEvent == nil {
		t.Fatalf("expected job.charge_created event")
	}

	var eventPayload map[string]any
	if err := json.Unmarshal([]byte(chargeEvent.DataJson), &eventPayload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	got, ok := eventPayload["charge_expires_at"].(string)
	if !ok {
		t.Fatalf("expected charge_expires_at string in event payload")
	}
	if got != expiresAtStr {
		t.Fatalf("expected event charge_expires_at %q, got %q", expiresAtStr, got)
	}
}

func TestJobPaymentSentEmitsEvent(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_pay"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))

	createPayload := payloadEnvelopeInput{
		PayloadID:     "pay_req_pay",
		PayloadKind:   payloadKindRequest,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_seller",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("request")),
	}
	createReq := jobCreateRequest{
		JobID:          "job_pay_1",
		OfferID:        offerID,
		RequestPayload: createPayload,
	}
	create := newJSONRequest(t, http.MethodPost, "/v0/jobs", mustJSONBytes(t, createReq))
	create.Header.Set(headerBotID, buyerID)
	createRec := httptestRequest(t, router, create)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createRec.Code, createRec.Body.String())
	}

	chargeReq := chargeCreateRequest{
		ChargeID:        "chg_pay_1",
		Address:         "nano_addr_pay",
		AmountRaw:       "1000",
		ChargeExpiresAt: now.Add(2 * time.Hour).Format(time.RFC3339),
		ChargeSig:       "sig",
	}
	charge := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_pay_1/charge", mustJSONBytes(t, chargeReq))
	charge.Header.Set(headerBotID, sellerID)
	chargeRec := httptestRequest(t, router, charge)
	if chargeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", chargeRec.Code, chargeRec.Body.String())
	}

	paymentReq := paymentSentRequest{PaymentBlockHash: "block_hash_1"}
	payment := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_pay_1/payment_sent", mustJSONBytes(t, paymentReq))
	payment.Header.Set(headerBotID, buyerID)
	paymentRec := httptestRequest(t, router, payment)
	if paymentRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", paymentRec.Code, paymentRec.Body.String())
	}

	sellerEvents, err := st.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
		RecipientBotID: sellerID,
		SinceEventID:   0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list seller events: %v", err)
	}
	if !containsEventType(sellerEvents, jobPaymentSentEventType) {
		t.Fatalf("expected job.payment_sent event for seller")
	}
}

func TestJobChargeExpiresAtRejectsNonCanonical(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_noncanon"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))

	createPayload := payloadEnvelopeInput{
		PayloadID:     "pay_req_noncanon",
		PayloadKind:   payloadKindRequest,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_seller",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("request")),
	}
	createReq := jobCreateRequest{
		JobID:          "job_noncanon",
		OfferID:        offerID,
		RequestPayload: createPayload,
	}
	create := newJSONRequest(t, http.MethodPost, "/v0/jobs", mustJSONBytes(t, createReq))
	create.Header.Set(headerBotID, buyerID)
	createRec := httptestRequest(t, router, create)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createRec.Code, createRec.Body.String())
	}

	expiresAt := now.Add(2 * time.Hour).Add(340 * time.Millisecond)
	canonical := expiresAt.UTC().Format(time.RFC3339Nano)
	nonCanonical := strings.TrimSuffix(canonical, "Z") + "0Z"
	chargeReq := chargeCreateRequest{
		ChargeID:        "chg_noncanon",
		Address:         "nano_addr_noncanon",
		AmountRaw:       "1000",
		ChargeExpiresAt: nonCanonical,
		ChargeSig:       "sig",
	}
	charge := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_noncanon/charge", mustJSONBytes(t, chargeReq))
	charge.Header.Set(headerBotID, sellerID)
	chargeRec := httptestRequest(t, router, charge)
	if chargeRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", chargeRec.Code, chargeRec.Body.String())
	}
}

func containsEventType(events []sqlc.Event, eventType string) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

func TestJobDeliverRequiresPaid(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_3"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))

	createPayload := payloadEnvelopeInput{
		PayloadID:     "pay_req_3",
		PayloadKind:   payloadKindRequest,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_seller",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("request")),
	}
	createReq := jobCreateRequest{
		JobID:          "job_3",
		OfferID:        offerID,
		RequestPayload: createPayload,
	}
	create := newJSONRequest(t, http.MethodPost, "/v0/jobs", mustJSONBytes(t, createReq))
	create.Header.Set(headerBotID, buyerID)
	createRec := httptestRequest(t, router, create)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", createRec.Code, createRec.Body.String())
	}

	deliverPayload := payloadEnvelopeInput{
		PayloadID:     "pay_deliver_3",
		PayloadKind:   payloadKindDeliver,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_buyer",
		CiphertextB64: base64.RawURLEncoding.EncodeToString([]byte("deliver")),
	}
	deliverReq := deliverRequest{Payload: deliverPayload}
	deliver := newJSONRequest(t, http.MethodPost, "/v0/jobs/job_3/deliver", mustJSONBytes(t, deliverReq))
	deliver.Header.Set(headerBotID, sellerID)
	deliverRec := httptestRequest(t, router, deliver)
	if deliverRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", deliverRec.Code, deliverRec.Body.String())
	}
}

func TestJobCreateRejectsLargePayload(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_big"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	oversized := make([]byte, maxPayloadBytes+1)
	ciphertext := base64.RawURLEncoding.EncodeToString(oversized)
	createPayload := payloadEnvelopeInput{
		PayloadID:     "pay_req_big",
		PayloadKind:   payloadKindRequest,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_seller",
		CiphertextB64: ciphertext,
	}
	createReq := jobCreateRequest{
		JobID:          "job_big",
		OfferID:        offerID,
		RequestPayload: createPayload,
	}

	clock := func() time.Time { return now }
	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))
	create := newJSONRequest(t, http.MethodPost, "/v0/jobs", mustJSONBytes(t, createReq))
	create.Header.Set(headerBotID, buyerID)
	createRec := httptestRequest(t, router, create)
	if createRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", createRec.Code, createRec.Body.String())
	}
}

func TestJobDeliverRejectsLargePayload(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_big_deliver"
	jobID := "job_big_deliver"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)
	seedJob(t, st, jobID, offerID, buyerID, sellerID, now)

	oversized := make([]byte, maxPayloadBytes+1)
	ciphertext := base64.RawURLEncoding.EncodeToString(oversized)
	deliverPayload := payloadEnvelopeInput{
		PayloadID:     "pay_msg_big",
		PayloadKind:   payloadKindMessage,
		EncAlg:        encAlgSealBox,
		RecipientKid:  "kid_buyer",
		CiphertextB64: ciphertext,
	}
	deliverReq := deliverRequest{Payload: deliverPayload}

	clock := func() time.Time { return now }
	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))
	deliver := newJSONRequest(t, http.MethodPost, "/v0/jobs/"+jobID+"/deliver", mustJSONBytes(t, deliverReq))
	deliver.Header.Set(headerBotID, sellerID)
	deliverRec := httptestRequest(t, router, deliver)
	if deliverRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", deliverRec.Code, deliverRec.Body.String())
	}
}

func TestJobListStatusFilterWithCursor(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	offerID := "offer_list"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobOffer(t, st, offerID, sellerID, now)

	seedJob(t, st, "job_a", offerID, buyerID, sellerID, now.Add(-2*time.Hour))
	seedJob(t, st, "job_b", offerID, buyerID, sellerID, now.Add(-time.Hour))
	seedJob(t, st, "job_c", offerID, buyerID, sellerID, now.Add(-30*time.Minute))

	if err := st.UpdateJobCancel(context.Background(), sqlc.UpdateJobCancelParams{
		JobID:       "job_b",
		CancelledAt: sql.NullTime{Time: now, Valid: true},
	}); err != nil {
		t.Fatalf("cancel job: %v", err)
	}

	clock := func() time.Time { return now }
	router := NewRouter(RouterConfig{Store: st}, WithClock(clock))
	listReq := httptest.NewRequest(http.MethodGet, "/v0/jobs?role=buyer&status=REQUESTED&limit=1", nil)
	listReq.Header.Set(headerBotID, buyerID)
	listRec := httptestRequest(t, router, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp jobListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(listResp.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(listResp.Jobs))
	}
	if listResp.Jobs[0].JobID != "job_c" {
		t.Fatalf("expected job_c first, got %q", listResp.Jobs[0].JobID)
	}
	if listResp.NextCursor == "" {
		t.Fatalf("expected next_cursor set")
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v0/jobs?role=buyer&status=REQUESTED&limit=1&cursor="+listResp.NextCursor, nil)
	secondReq.Header.Set(headerBotID, buyerID)
	secondRec := httptestRequest(t, router, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", secondRec.Code, secondRec.Body.String())
	}

	var secondResp jobListResponse
	if err := json.Unmarshal(secondRec.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(secondResp.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(secondResp.Jobs))
	}
	if secondResp.Jobs[0].JobID != "job_a" {
		t.Fatalf("expected job_a second, got %q", secondResp.Jobs[0].JobID)
	}
}

func seedJobBot(t *testing.T, st *store.Store, botID string, now time.Time) {
	t.Helper()
	err := st.CreateBot(context.Background(), sqlc.CreateBotParams{
		BotID:                  botID,
		SigningPubkeyEd25519:   "signing",
		EncryptionPubkeyX25519: "encryption",
		SigningKid:             "signing_kid",
		EncryptionKid:          "encryption_kid",
		CreatedAt:              now,
		LastSeenAt:             sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}
}

func seedJobOffer(t *testing.T, st *store.Store, offerID, sellerID string, now time.Time) {
	t.Helper()
	err := st.CreateOffer(context.Background(), sqlc.CreateOfferParams{
		OfferID:           offerID,
		SellerBotID:       sellerID,
		Title:             "title",
		Description:       "desc",
		TagsJson:          "[]",
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		CreatedAt:         now,
		ExpiresAt:         sql.NullTime{},
		Status:            string(domain.OfferActive),
		CancelledAt:       sql.NullTime{},
		RequestSchemaHint: sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
}

func newJSONRequest(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	var buf *bytes.Reader
	if body == nil {
		buf = bytes.NewReader(nil)
	} else {
		buf = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
