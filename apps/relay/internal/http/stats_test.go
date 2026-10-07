package httpapi

import (
	"context"
	"database/sql"
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

func TestStatsEndpoint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Now().UTC().Add(-time.Hour)

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
	if resp.Demand.PaidJobs != 2 || resp.Demand.DeliveredJobs != 1 || resp.Demand.UniqueBuyers != 1 || resp.Demand.RepeatBuyers != 1 {
		t.Fatalf("unexpected demand: %+v", resp.Demand)
	}
	start, err := time.Parse(time.RFC3339Nano, resp.Demand.WindowStart)
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse(time.RFC3339Nano, resp.Demand.WindowEnd)
	if err != nil || end.Sub(start) != 28*24*time.Hour || resp.Demand.WindowDays != 28 {
		t.Fatalf("invalid window: %+v, %v", resp.Demand, err)
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatal("missing cache policy")
	}
	var public map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	if len(public) != 5 {
		t.Fatalf("unexpected public fields: %v", public)
	}
	var demand map[string]json.RawMessage
	if err := json.Unmarshal(public["demand"], &demand); err != nil {
		t.Fatal(err)
	}
	if len(demand) != 7 {
		t.Fatalf("unexpected demand fields: %v", demand)
	}
	for _, key := range []string{"window_days", "window_start", "window_end", "paid_jobs", "delivered_jobs", "unique_buyers", "repeat_buyers"} {
		if _, ok := demand[key]; !ok {
			t.Fatalf("missing public field %q", key)
		}
	}
	for _, private := range []string{buyerID, sellerID, revokedID, "job_paid", "job_delivered", "offer_a", "buyer_bot_id", "payment_block_hash"} {
		if strings.Contains(rec.Body.String(), private) {
			t.Fatalf("private value leaked: %q", private)
		}
	}
	// Public callers cannot request per-buyer slices or arbitrary historical windows.
	filtered := httptestRequest(t, NewRouter(RouterConfig{Store: st}), httptest.NewRequest(http.MethodGet, "/stats?buyer_bot_id=unknown&window_days=365", nil))
	var unfiltered statsResponse
	if err := json.Unmarshal(filtered.Body.Bytes(), &unfiltered); err != nil {
		t.Fatal(err)
	}
	if filtered.Code != http.StatusOK || unfiltered.Demand.UniqueBuyers != 1 || unfiltered.Demand.WindowDays != 28 {
		t.Fatalf("query altered scope: %s", filtered.Body.String())
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
