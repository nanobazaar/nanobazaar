package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nanobazaar/relay/internal/store/sqlc"
)

func TestDeleteEventsBefore(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()

	st := New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	seedBot(t, st, "bot_events", now)

	seedEvent(t, st, "bot_events", "job.requested", map[string]any{"job_id": "job_1"}, now.Add(-2*time.Hour))
	seedEvent(t, st, "bot_events", "job.paid", map[string]any{"job_id": "job_1"}, now)

	if err := st.DeleteEventsBefore(context.Background(), now.Add(-time.Hour)); err != nil {
		t.Fatalf("delete events: %v", err)
	}

	remaining, err := st.ListEventsAfterID(context.Background(), sqlc.ListEventsAfterIDParams{
		RecipientBotID: "bot_events",
		SinceEventID:   0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 event, got %d", len(remaining))
	}
	if remaining[0].EventType != "job.paid" {
		t.Fatalf("unexpected event type %q", remaining[0].EventType)
	}
}

func TestDeletePayloadsFetchedBefore(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()

	st := New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerID := "bot_buyer"
	sellerID := "bot_seller"
	seedBot(t, st, buyerID, now)
	seedBot(t, st, sellerID, now)

	offerID := "offer_payloads"
	seedOffer(t, st, offerID, sellerID, now)
	seedJob(t, st, "job_payloads", offerID, buyerID, sellerID, "REQUESTED", now, sql.NullTime{}, sql.NullTime{}, sql.NullTime{})

	seedPayload(t, st, "payload_old", "job_payloads", sellerID, buyerID, "message", now.Add(-2*time.Hour), sql.NullTime{Time: now.Add(-90 * time.Minute), Valid: true})
	seedPayload(t, st, "payload_new", "job_payloads", sellerID, buyerID, "message", now.Add(-time.Hour), sql.NullTime{Time: now.Add(-30 * time.Minute), Valid: true})
	seedPayload(t, st, "payload_unfetched", "job_payloads", sellerID, buyerID, "message", now, sql.NullTime{})

	if err := st.DeletePayloadsFetchedBefore(context.Background(), sql.NullTime{Time: now.Add(-time.Hour), Valid: true}); err != nil {
		t.Fatalf("delete payloads fetched: %v", err)
	}

	_, err := st.GetPayload(context.Background(), sqlc.GetPayloadParams{PayloadID: "payload_old", RecipientBotID: buyerID})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected payload_old deleted, got %v", err)
	}
	if _, err := st.GetPayload(context.Background(), sqlc.GetPayloadParams{PayloadID: "payload_new", RecipientBotID: buyerID}); err != nil {
		t.Fatalf("expected payload_new retained, got %v", err)
	}
	if _, err := st.GetPayload(context.Background(), sqlc.GetPayloadParams{PayloadID: "payload_unfetched", RecipientBotID: buyerID}); err != nil {
		t.Fatalf("expected payload_unfetched retained, got %v", err)
	}
}

func TestDeleteJobsTerminalBefore(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()

	st := New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	buyerID := "bot_job_buyer"
	sellerID := "bot_job_seller"
	seedBot(t, st, buyerID, now)
	seedBot(t, st, sellerID, now)

	offerID := "offer_jobs"
	seedOffer(t, st, offerID, sellerID, now)

	seedJob(t, st, "job_cancel_old", offerID, buyerID, sellerID, "CANCELLED", now.Add(-2*time.Hour), sql.NullTime{Time: now.Add(-2 * time.Hour), Valid: true}, sql.NullTime{}, sql.NullTime{})
	seedJob(t, st, "job_cancel_new", offerID, buyerID, sellerID, "CANCELLED", now, sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, sql.NullTime{})
	seedJob(t, st, "job_expired_old", offerID, buyerID, sellerID, "EXPIRED", now.Add(-2*time.Hour), sql.NullTime{}, sql.NullTime{Time: now.Add(-2 * time.Hour), Valid: true}, sql.NullTime{})
	seedJob(t, st, "job_delivered_old", offerID, buyerID, sellerID, "DELIVERED", now.Add(-2*time.Hour), sql.NullTime{}, sql.NullTime{}, sql.NullTime{Time: now.Add(-2 * time.Hour), Valid: true})
	seedJob(t, st, "job_requested", offerID, buyerID, sellerID, "REQUESTED", now.Add(-2*time.Hour), sql.NullTime{}, sql.NullTime{}, sql.NullTime{})

	if err := st.DeleteJobsTerminalBefore(context.Background(), sql.NullTime{Time: now.Add(-time.Hour), Valid: true}); err != nil {
		t.Fatalf("delete jobs: %v", err)
	}

	assertJobDeleted(t, st, "job_cancel_old")
	assertJobDeleted(t, st, "job_expired_old")
	assertJobDeleted(t, st, "job_delivered_old")
	assertJobPresent(t, st, "job_cancel_new")
	assertJobPresent(t, st, "job_requested")
}

func setupStoreTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)

	schemaPath := filepath.Join("..", "..", "db", "schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}

func seedBot(t *testing.T, st *Store, botID string, createdAt time.Time) {
	t.Helper()
	err := st.CreateBot(context.Background(), sqlc.CreateBotParams{
		BotID:                  botID,
		SigningPubkeyEd25519:   "sig-" + botID,
		EncryptionPubkeyX25519: "enc-" + botID,
		SigningKid:             "sign-" + botID,
		EncryptionKid:          "enc-" + botID,
		CreatedAt:              createdAt,
		LastSeenAt:             sql.NullTime{},
	})
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}
}

func seedOffer(t *testing.T, st *Store, offerID, sellerID string, createdAt time.Time) {
	t.Helper()
	err := st.CreateOffer(context.Background(), sqlc.CreateOfferParams{
		OfferID:           offerID,
		SellerBotID:       sellerID,
		Title:             "Test",
		Description:       "Test offer",
		TagsJson:          `["test"]`,
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		CreatedAt:         createdAt,
		ExpiresAt:         sql.NullTime{Time: createdAt.Add(24 * time.Hour), Valid: true},
		Status:            "ACTIVE",
		CancelledAt:       sql.NullTime{},
		RequestSchemaHint: sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
}

func seedJob(t *testing.T, st *Store, jobID, offerID, buyerID, sellerID, status string, createdAt time.Time, cancelledAt, expiredAt, deliveredAt sql.NullTime) {
	t.Helper()
	err := st.CreateJob(context.Background(), sqlc.CreateJobParams{
		JobID:             jobID,
		OfferID:           offerID,
		BuyerBotID:        buyerID,
		SellerBotID:       sellerID,
		Status:            status,
		PriceRaw:          "1000",
		TurnaroundSeconds: 3600,
		CreatedAt:         createdAt,
		JobExpiresAt:      createdAt.Add(24 * time.Hour),
		RequestPayloadID:  "payload_" + jobID,
		ChargeID:          sql.NullString{},
		ChargeAddress:     sql.NullString{},
		ChargeAmountRaw:   sql.NullString{},
		ChargeExpiresAt:   sql.NullTime{},
		ChargeSigEd25519:  sql.NullString{},
		PaidAt:            sql.NullTime{},
		DeliveredAt:       deliveredAt,
		CancelledAt:       cancelledAt,
		ExpiredAt:         expiredAt,
		PaymentVerifier:   sql.NullString{},
		PaymentBlockHash:  sql.NullString{},
		PaymentObservedAt: sql.NullTime{},
		AmountRawReceived: sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
}

func seedEvent(t *testing.T, st *Store, recipient, eventType string, data map[string]any, createdAt time.Time) {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if err := st.CreateEvent(context.Background(), sqlc.CreateEventParams{
		RecipientBotID: recipient,
		EventType:      eventType,
		DataJson:       string(payload),
		CreatedAt:      createdAt,
	}); err != nil {
		t.Fatalf("create event: %v", err)
	}
}

func seedPayload(t *testing.T, st *Store, payloadID, jobID, senderID, recipientID, kind string, createdAt time.Time, fetchedAt sql.NullTime) {
	t.Helper()
	if err := st.CreatePayload(context.Background(), sqlc.CreatePayloadParams{
		PayloadID:      payloadID,
		RecipientBotID: recipientID,
		JobID:          jobID,
		SenderBotID:    senderID,
		PayloadKind:    kind,
		EncAlg:         "enc",
		RecipientKid:   "kid",
		CiphertextB64:  "cipher",
		CreatedAt:      createdAt,
		FetchedAt:      fetchedAt,
	}); err != nil {
		t.Fatalf("create payload: %v", err)
	}
}

func assertJobDeleted(t *testing.T, st *Store, jobID string) {
	t.Helper()
	_, err := st.GetJob(context.Background(), jobID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected %s deleted, got %v", jobID, err)
	}
}

func assertJobPresent(t *testing.T, st *Store, jobID string) {
	t.Helper()
	if _, err := st.GetJob(context.Background(), jobID); err != nil {
		t.Fatalf("expected %s present, got %v", jobID, err)
	}
}
