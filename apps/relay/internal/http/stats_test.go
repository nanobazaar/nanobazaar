package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/domain"
	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

func TestStatsEndpoint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	revokedID := "bot_revoked"
	seedJobBot(t, st, buyerID, now)
	seedJobBot(t, st, sellerID, now)
	seedJobBot(t, st, revokedID, now)
	if _, err := st.UpdateBotRevoke(context.Background(), sqlc.UpdateBotRevokeParams{
		RevokedAt: sql.NullTime{Time: now, Valid: true},
		BotID:     revokedID,
	}); err != nil {
		t.Fatalf("revoke bot: %v", err)
	}

	seedJobOffer(t, st, "offer_a", sellerID, now)
	seedJobOffer(t, st, "offer_b", sellerID, now)

	seedJobWithStatus(t, st, "job_paid", "offer_a", buyerID, sellerID, now, string(domain.JobPaid), "1000000000000000000000000000000")
	seedJobWithStatus(t, st, "job_delivered", "offer_b", buyerID, sellerID, now, string(domain.JobDelivered), "500000000000000000000000000000")
	seedJobWithStatus(t, st, "job_requested", "offer_b", buyerID, sellerID, now, string(domain.JobRequested), "")

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptestRequest(t, NewRouter(RouterConfig{Store: st}), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Offers != 2 {
		t.Fatalf("expected offers 2, got %d", resp.Offers)
	}
	if resp.Jobs != 2 {
		t.Fatalf("expected jobs 2, got %d", resp.Jobs)
	}
	if resp.AgentsOnline != 2 {
		t.Fatalf("expected agents_online 2, got %d", resp.AgentsOnline)
	}
	if resp.XnoTransferred != "1.5" {
		t.Fatalf("expected xno_transferred 1.5, got %q", resp.XnoTransferred)
	}
}

func seedJobWithStatus(t *testing.T, st *store.Store, jobID, offerID, buyerID, sellerID string, now time.Time, status, amountRaw string) {
	t.Helper()
	amount := sql.NullString{}
	if amountRaw != "" {
		amount = sql.NullString{String: amountRaw, Valid: true}
	}
	paidAt := sql.NullTime{}
	deliveredAt := sql.NullTime{}
	if status == string(domain.JobPaid) || status == string(domain.JobDelivered) {
		paidAt = sql.NullTime{Time: now, Valid: true}
	}
	if status == string(domain.JobDelivered) {
		deliveredAt = sql.NullTime{Time: now, Valid: true}
	}

	err := st.CreateJob(context.Background(), sqlc.CreateJobParams{
		JobID:             jobID,
		OfferID:           offerID,
		BuyerBotID:        buyerID,
		SellerBotID:       sellerID,
		Status:            status,
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
		PaidAt:            paidAt,
		DeliveredAt:       deliveredAt,
		CancelledAt:       sql.NullTime{},
		ExpiredAt:         sql.NullTime{},
		PaymentVerifier:   sql.NullString{},
		PaymentBlockHash:  sql.NullString{},
		PaymentObservedAt: sql.NullTime{},
		AmountRawReceived: amount,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
}
