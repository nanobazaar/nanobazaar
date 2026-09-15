package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/store/sqlc"
	"github.com/pressly/goose/v3"
)

func TestEventRetentionMigrationPreservesCommerce(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrations := filepath.Join("..", "..", "db", "migrations")
	if err := goose.UpTo(db, migrations, 9); err != nil {
		t.Fatal(err)
	}
	st := New(db)
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	for _, bot := range []string{"buyer", "seller", "empty"} {
		seedBot(t, st, bot, now)
	}
	seedOffer(t, st, "offer", "seller", now)
	seedJob(t, st, "job", "offer", "buyer", "seller", "PAID", now, sql.NullTime{}, sql.NullTime{}, sql.NullTime{})
	if _, err := db.Exec(`UPDATE jobs SET charge_id = 'charge', charge_address = 'nano_test',
		charge_amount_raw = '1000', charge_expires_at = ?, charge_sig_ed25519 = 'signature',
		paid_at = ?, payment_verifier = 'seller', payment_block_hash = 'block',
		payment_observed_at = ?, amount_raw_received = '1000' WHERE job_id = 'job'`, now.Add(time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	seedPayload(t, st, "payload_job", "job", "buyer", "seller", "request", now, sql.NullTime{})
	for _, bot := range []string{"buyer", "seller", "buyer", "empty", "seller"} {
		seedEvent(t, st, bot, "job.paid", map[string]any{"job_id": "job"}, now)
	}
	if _, err := db.Exec("DELETE FROM events WHERE event_id IN (1, 2, 4)"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertPollAck(context.Background(), sqlc.UpsertPollAckParams{RecipientBotID: "buyer", LastAckedEventID: 3, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	before := commerceRows(t, db)
	if err := goose.UpTo(db, migrations, 10); err != nil {
		t.Fatal(err)
	}
	for recipient, want := range map[string]int64{"buyer": 2, "seller": 4, "empty": 5} {
		assertDeletedThrough(t, db, recipient, want)
	}
	if after := commerceRows(t, db); !reflect.DeepEqual(before, after) {
		t.Fatalf("migration changed commerce rows\nbefore: %#v\nafter: %#v", before, after)
	}
	seedBot(t, st, "new", now)
	seedEvent(t, st, "new", "job.requested", map[string]any{"job_id": "new_job"}, now)
	retention, err := ReadEventRetention(context.Background(), db, "new")
	if err != nil || retention.DeletedThroughEventID != 0 || retention.MinEventIDRetained != 6 {
		t.Fatalf("new migrated bot must be pollable from zero: retention=%+v err=%v", retention, err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM event_retention WHERE recipient_bot_id = 'new'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("new bot should have no legacy watermark: count=%d err=%v", count, err)
	}
	if _, err := db.Exec("DELETE FROM events WHERE recipient_bot_id = 'new'"); err != nil {
		t.Fatal(err)
	}
	assertDeletedThrough(t, db, "new", 6)
	beforeRollback := commerceRows(t, db)
	if err := goose.Down(db, migrations); err != nil {
		t.Fatal(err)
	}
	if after := commerceRows(t, db); !reflect.DeepEqual(beforeRollback, after) {
		t.Fatalf("rollback changed commerce rows\nbefore: %#v\nafter: %#v", beforeRollback, after)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name IN ('event_retention', 'events_retention_ad')").Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback must remove retention table and trigger: count=%d err=%v", count, err)
	}
	// Deleting events must still work after rollback removes the trigger's table.
	if _, err := db.Exec("DELETE FROM events"); err != nil {
		t.Fatalf("delete after rollback: %v", err)
	}
}

func TestEventRetentionMigrationEmptySequence(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrations := filepath.Join("..", "..", "db", "migrations")
	if err := goose.UpTo(db, migrations, 9); err != nil {
		t.Fatal(err)
	}
	seedBot(t, New(db), "empty", time.Now().UTC())
	if err := goose.UpTo(db, migrations, 10); err != nil {
		t.Fatal(err)
	}
	assertDeletedThrough(t, db, "empty", 0)
}

func TestRetentionDeletesTrackHighestRecipientEventAtomically(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()
	st := New(db)
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	seedBot(t, st, "recipient", now)
	seedBot(t, st, "other", now)
	seedEvent(t, st, "recipient", "job.requested", nil, now)
	seedEvent(t, st, "other", "job.requested", nil, now.Add(-3*time.Hour))
	seedEvent(t, st, "recipient", "job.paid", nil, now.Add(-2*time.Hour))
	seedEvent(t, st, "recipient", "job.delivered", nil, now.Add(-time.Hour))
	if err := st.DeleteEventsBefore(context.Background(), now.Add(-90*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertDeletedThrough(t, db, "recipient", 3)
	assertDeletedThrough(t, db, "other", 2)
	// Repeated cleanup and deleting an older ID cannot lower the remembered loss.
	if err := st.DeleteEventsBefore(context.Background(), now.Add(-90*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM events WHERE event_id = 1"); err != nil {
		t.Fatal(err)
	}
	assertDeletedThrough(t, db, "recipient", 3)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("DELETE FROM events WHERE event_id = 4"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertDeletedThrough(t, db, "recipient", 3)
	if _, err := st.GetEventCreatedAt(context.Background(), sqlc.GetEventCreatedAtParams{RecipientBotID: "recipient", EventID: 4}); err != nil {
		t.Fatalf("rollback should restore event and watermark together: %v", err)
	}
}

func TestEventRetentionAndEventsUseOneReadSnapshot(t *testing.T) {
	// WAL permits retention to commit while a poll is reading an earlier
	// snapshot. Without a shared transaction, the page can silently lose work.
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "relay.db")+"?_journal_mode=WAL&_busy_timeout=1000")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	if _, err := db.Exec(`CREATE TABLE bots (bot_id TEXT PRIMARY KEY);
		CREATE TABLE poll_acks (recipient_bot_id TEXT PRIMARY KEY, last_acked_event_id INTEGER, updated_at DATETIME);
		CREATE TABLE events (event_id INTEGER PRIMARY KEY AUTOINCREMENT, recipient_bot_id TEXT, event_type TEXT, data_json TEXT, created_at DATETIME);
		CREATE TABLE event_retention (recipient_bot_id TEXT PRIMARY KEY, deleted_through_event_id INTEGER);
		INSERT INTO bots VALUES ('recipient');
		INSERT INTO poll_acks VALUES ('recipient', 0, '2026-09-15 08:00:00');
		INSERT INTO events VALUES (1, 'recipient', 'job.requested', '{}', '2026-09-15 08:00:00');`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	queries := New(db).Queries.WithTx(tx)
	if _, err := queries.GetPollAck(context.Background(), "recipient"); err != nil {
		t.Fatal(err)
	}
	writer, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec("DELETE FROM events; INSERT INTO event_retention VALUES ('recipient', 1)"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	retention, err := ReadEventRetention(context.Background(), tx, "recipient")
	if err != nil || retention.DeletedThroughEventID != 0 || retention.MinEventIDRetained != 1 {
		t.Fatalf("snapshot retention=%+v err=%v", retention, err)
	}
	events, err := queries.ListEventsAfterID(context.Background(), sqlc.ListEventsAfterIDParams{RecipientBotID: "recipient", SinceEventID: 0, Limit: 10})
	if err != nil || len(events) != 1 || events[0].EventID != 1 {
		t.Fatalf("snapshot lost event: events=%v err=%v", events, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	retention, err = ReadEventRetention(context.Background(), db, "recipient")
	if err != nil || retention.DeletedThroughEventID != 1 || retention.MinEventIDRetained != 2 {
		t.Fatalf("next poll retention=%+v err=%v", retention, err)
	}
}

func assertDeletedThrough(t *testing.T, db *sql.DB, recipient string, want int64) {
	t.Helper()
	var got int64
	if err := db.QueryRow("SELECT deleted_through_event_id FROM event_retention WHERE recipient_bot_id = ?", recipient).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s deleted through %d, want %d", recipient, got, want)
	}
}

// Compare all persisted commerce columns, including keys, charge/payment proof,
// ciphertext and retained events, across migration and rollback.
func commerceRows(t *testing.T, db *sql.DB) map[string][][]any {
	t.Helper()
	result := make(map[string][][]any)
	for _, table := range []string{"bots", "offers", "offer_tags", "jobs", "payloads", "events", "poll_acks"} {
		rows, err := db.Query("SELECT * FROM " + table + " ORDER BY rowid")
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				t.Fatal(err)
			}
			result[table] = append(result[table], values)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return result
}
