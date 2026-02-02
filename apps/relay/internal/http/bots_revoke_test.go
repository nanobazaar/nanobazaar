package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/domain"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
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

func TestBotsRevokeCancelsOffersJobsAndEmitsEvents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, st, pub)

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)

	offerActive := createOfferForTest(t, st, botID, "offer_active", now.Add(-time.Hour))
	offerPaused := createOfferForTest(t, st, botID, "offer_paused", now.Add(-time.Hour))
	if err := st.UpdateOfferPause(context.Background(), offerPaused); err != nil {
		t.Fatalf("pause offer: %v", err)
	}
	offerOther := createOfferForTest(t, st, sellerID, "offer_other", now.Add(-time.Hour))

	jobRequested := "job_requested"
	seedJob(t, st, jobRequested, offerActive, buyerID, botID, now.Add(-time.Minute))

	jobCharge := "job_charge"
	seedJob(t, st, jobCharge, offerOther, botID, sellerID, now.Add(-time.Minute))
	if err := st.UpdateJobCharge(context.Background(), sqlc.UpdateJobChargeParams{
		JobID:            jobCharge,
		ChargeID:         sql.NullString{String: "charge_job_charge", Valid: true},
		ChargeAddress:    sql.NullString{String: "nano_charge", Valid: true},
		ChargeAmountRaw:  sql.NullString{String: "1000", Valid: true},
		ChargeExpiresAt:  sql.NullTime{Time: now.Add(time.Hour), Valid: true},
		ChargeSigEd25519: sql.NullString{String: "sig_charge", Valid: true},
	}); err != nil {
		t.Fatalf("charge job: %v", err)
	}

	offerPaid := createOfferForTest(t, st, botID, "offer_paid", now.Add(-time.Hour))
	jobPaid := "job_paid"
	seedJob(t, st, jobPaid, offerPaid, buyerID, botID, now.Add(-time.Minute))
	if err := st.UpdateJobCharge(context.Background(), sqlc.UpdateJobChargeParams{
		JobID:            jobPaid,
		ChargeID:         sql.NullString{String: "charge_job_paid", Valid: true},
		ChargeAddress:    sql.NullString{String: "nano_paid", Valid: true},
		ChargeAmountRaw:  sql.NullString{String: "1000", Valid: true},
		ChargeExpiresAt:  sql.NullTime{Time: now.Add(2 * time.Hour), Valid: true},
		ChargeSigEd25519: sql.NullString{String: "sig_paid", Valid: true},
	}); err != nil {
		t.Fatalf("charge paid job: %v", err)
	}
	if err := st.UpdateJobMarkPaid(context.Background(), sqlc.UpdateJobMarkPaidParams{
		JobID:             jobPaid,
		PaidAt:            sql.NullTime{Time: now.Add(3 * time.Hour), Valid: true},
		PaymentVerifier:   sql.NullString{},
		PaymentBlockHash:  sql.NullString{},
		PaymentObservedAt: sql.NullTime{},
		AmountRawReceived: sql.NullString{},
	}); err != nil {
		t.Fatalf("mark paid: %v", err)
	}

	offerDelivered := createOfferForTest(t, st, sellerID, "offer_delivered", now.Add(-time.Hour))
	jobDelivered := "job_delivered"
	seedJob(t, st, jobDelivered, offerDelivered, botID, sellerID, now.Add(-time.Minute))
	if err := st.UpdateJobCharge(context.Background(), sqlc.UpdateJobChargeParams{
		JobID:            jobDelivered,
		ChargeID:         sql.NullString{String: "charge_job_delivered", Valid: true},
		ChargeAddress:    sql.NullString{String: "nano_delivered", Valid: true},
		ChargeAmountRaw:  sql.NullString{String: "1000", Valid: true},
		ChargeExpiresAt:  sql.NullTime{Time: now.Add(4 * time.Hour), Valid: true},
		ChargeSigEd25519: sql.NullString{String: "sig_delivered", Valid: true},
	}); err != nil {
		t.Fatalf("charge delivered job: %v", err)
	}
	if err := st.UpdateJobMarkPaid(context.Background(), sqlc.UpdateJobMarkPaidParams{
		JobID:             jobDelivered,
		PaidAt:            sql.NullTime{Time: now.Add(5 * time.Hour), Valid: true},
		PaymentVerifier:   sql.NullString{},
		PaymentBlockHash:  sql.NullString{},
		PaymentObservedAt: sql.NullTime{},
		AmountRawReceived: sql.NullString{},
	}); err != nil {
		t.Fatalf("mark delivered paid: %v", err)
	}
	if err := st.UpdateJobDeliver(context.Background(), sqlc.UpdateJobDeliverParams{
		JobID:       jobDelivered,
		DeliveredAt: sql.NullTime{Time: now.Add(6 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("deliver job: %v", err)
	}

	revokePath := "/v0/bots/" + botID + "/revoke"
	revokeReq := signedRequest(t, priv, botID, http.MethodPost, revokePath, "", nil, now, "nonce-1")
	revokeReq.Header.Set(headerIdempotency, "idem-1")
	revokeRec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}

	var revokeResp botRevokeResponse
	if err := json.Unmarshal(revokeRec.Body.Bytes(), &revokeResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	ctx := context.Background()
	revokedAt := revokeResp.RevokedAt

	activeOffer, err := st.GetOffer(ctx, offerActive)
	if err != nil {
		t.Fatalf("get active offer: %v", err)
	}
	if activeOffer.Status != string(domain.OfferCancelled) {
		t.Fatalf("expected active offer cancelled, got %q", activeOffer.Status)
	}
	if !activeOffer.CancelledAt.Valid || !activeOffer.CancelledAt.Time.Equal(revokedAt) {
		t.Fatalf("expected active offer cancelled_at %s, got %v", revokedAt.Format(time.RFC3339Nano), activeOffer.CancelledAt)
	}

	pausedOffer, err := st.GetOffer(ctx, offerPaused)
	if err != nil {
		t.Fatalf("get paused offer: %v", err)
	}
	if pausedOffer.Status != string(domain.OfferCancelled) {
		t.Fatalf("expected paused offer cancelled, got %q", pausedOffer.Status)
	}
	if !pausedOffer.CancelledAt.Valid || !pausedOffer.CancelledAt.Time.Equal(revokedAt) {
		t.Fatalf("expected paused offer cancelled_at %s, got %v", revokedAt.Format(time.RFC3339Nano), pausedOffer.CancelledAt)
	}

	requestedJob, err := st.GetJob(ctx, jobRequested)
	if err != nil {
		t.Fatalf("get requested job: %v", err)
	}
	if requestedJob.Status != string(domain.JobCancelled) {
		t.Fatalf("expected requested job cancelled, got %q", requestedJob.Status)
	}
	if !requestedJob.CancelledAt.Valid || !requestedJob.CancelledAt.Time.Equal(revokedAt) {
		t.Fatalf("expected requested job cancelled_at %s, got %v", revokedAt.Format(time.RFC3339Nano), requestedJob.CancelledAt)
	}

	chargedJob, err := st.GetJob(ctx, jobCharge)
	if err != nil {
		t.Fatalf("get charged job: %v", err)
	}
	if chargedJob.Status != string(domain.JobCancelled) {
		t.Fatalf("expected charged job cancelled, got %q", chargedJob.Status)
	}
	if !chargedJob.CancelledAt.Valid || !chargedJob.CancelledAt.Time.Equal(revokedAt) {
		t.Fatalf("expected charged job cancelled_at %s, got %v", revokedAt.Format(time.RFC3339Nano), chargedJob.CancelledAt)
	}

	paidJob, err := st.GetJob(ctx, jobPaid)
	if err != nil {
		t.Fatalf("get paid job: %v", err)
	}
	if paidJob.Status != string(domain.JobPaid) {
		t.Fatalf("expected paid job to remain PAID, got %q", paidJob.Status)
	}
	if paidJob.CancelledAt.Valid {
		t.Fatalf("expected paid job cancelled_at unset")
	}

	deliveredJob, err := st.GetJob(ctx, jobDelivered)
	if err != nil {
		t.Fatalf("get delivered job: %v", err)
	}
	if deliveredJob.Status != string(domain.JobDelivered) {
		t.Fatalf("expected delivered job to remain DELIVERED, got %q", deliveredJob.Status)
	}
	if deliveredJob.CancelledAt.Valid {
		t.Fatalf("expected delivered job cancelled_at unset")
	}

	botEvents, err := st.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
		RecipientBotID: botID,
		SinceEventID:   0,
		Limit:          100,
	})
	if err != nil {
		t.Fatalf("list bot events: %v", err)
	}
	if len(botEvents) != 5 {
		t.Fatalf("expected 5 events for revoked bot, got %d", len(botEvents))
	}
	expectedOffers := map[string]struct{}{
		offerActive: {},
		offerPaused: {},
		offerPaid:   {},
	}
	expectedJobs := map[string]struct{}{
		jobRequested: {},
		jobCharge:    {},
	}
	expectedCancelledAt := revokedAt.UTC().Format(time.RFC3339Nano)
	for _, event := range botEvents {
		var payload map[string]any
		if err := json.Unmarshal([]byte(event.DataJson), &payload); err != nil {
			t.Fatalf("unmarshal event payload: %v", err)
		}
		switch event.EventType {
		case offerCancelledEventType:
			offerID, ok := payload["offer_id"].(string)
			if !ok {
				t.Fatalf("offer.cancelled missing offer_id")
			}
			if _, ok := expectedOffers[offerID]; !ok {
				t.Fatalf("unexpected offer.cancelled offer_id %q", offerID)
			}
			if payload["cancelled_at"] != expectedCancelledAt {
				t.Fatalf("expected offer cancelled_at %s, got %v", expectedCancelledAt, payload["cancelled_at"])
			}
			delete(expectedOffers, offerID)
		case jobCancelledEventType:
			jobID, ok := payload["job_id"].(string)
			if !ok {
				t.Fatalf("job.cancelled missing job_id")
			}
			if _, ok := expectedJobs[jobID]; !ok {
				t.Fatalf("unexpected job.cancelled job_id %q", jobID)
			}
			if payload["cancelled_at"] != expectedCancelledAt {
				t.Fatalf("expected job cancelled_at %s, got %v", expectedCancelledAt, payload["cancelled_at"])
			}
			delete(expectedJobs, jobID)
		default:
			t.Fatalf("unexpected event type %s", event.EventType)
		}
	}
	if len(expectedOffers) != 0 {
		t.Fatalf("missing offer.cancelled events")
	}
	if len(expectedJobs) != 0 {
		t.Fatalf("missing job.cancelled events")
	}

	buyerEvents, err := st.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
		RecipientBotID: buyerID,
		SinceEventID:   0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list buyer events: %v", err)
	}
	if len(buyerEvents) != 1 {
		t.Fatalf("expected 1 buyer event, got %d", len(buyerEvents))
	}
	if buyerEvents[0].EventType != jobCancelledEventType {
		t.Fatalf("expected buyer job.cancelled, got %s", buyerEvents[0].EventType)
	}

	var buyerPayload map[string]any
	if err := json.Unmarshal([]byte(buyerEvents[0].DataJson), &buyerPayload); err != nil {
		t.Fatalf("unmarshal buyer payload: %v", err)
	}
	if buyerPayload["job_id"] != jobRequested {
		t.Fatalf("expected buyer job_id %s, got %v", jobRequested, buyerPayload["job_id"])
	}
	if buyerPayload["cancelled_at"] != expectedCancelledAt {
		t.Fatalf("expected buyer cancelled_at %s, got %v", expectedCancelledAt, buyerPayload["cancelled_at"])
	}

	sellerEvents, err := st.ListEventsAfterID(ctx, sqlc.ListEventsAfterIDParams{
		RecipientBotID: sellerID,
		SinceEventID:   0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list seller events: %v", err)
	}
	if len(sellerEvents) != 1 {
		t.Fatalf("expected 1 seller event, got %d", len(sellerEvents))
	}
	if sellerEvents[0].EventType != jobCancelledEventType {
		t.Fatalf("expected seller job.cancelled, got %s", sellerEvents[0].EventType)
	}
	var sellerPayload map[string]any
	if err := json.Unmarshal([]byte(sellerEvents[0].DataJson), &sellerPayload); err != nil {
		t.Fatalf("unmarshal seller payload: %v", err)
	}
	if sellerPayload["job_id"] != jobCharge {
		t.Fatalf("expected seller job_id %s, got %v", jobCharge, sellerPayload["job_id"])
	}
	if sellerPayload["cancelled_at"] != expectedCancelledAt {
		t.Fatalf("expected seller cancelled_at %s, got %v", expectedCancelledAt, sellerPayload["cancelled_at"])
	}
}

func TestBotsRevokeMissingBotIDHeader(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	verifier := auth.NewVerifier(st)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	verifier.Clock = func() time.Time { return now }

	pub, priv := generateSigningKey(t)
	botID := seedBotWithKey(t, st, pub)

	path := "/v0/bots/" + botID + "/revoke"
	req := signedRequest(t, priv, botID, http.MethodPost, path, "", nil, now, "nonce-1")
	req.Header.Set(headerIdempotency, "idem-1")
	req.Header.Del(headerBotID)

	rec := httptestRequest(t, NewRouter(RouterConfig{Verifier: verifier, Store: st}), req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
