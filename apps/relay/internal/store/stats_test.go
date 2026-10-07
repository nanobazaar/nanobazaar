package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/store/sqlc"
	"github.com/pressly/goose/v3"
)

func TestDemandStatsWindowAndCohort(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()
	st := New(db)
	ctx := context.Background()
	now := time.Date(2026, 10, 7, 19, 0, 0, 123456789, time.UTC)
	start := now.Add(-28 * 24 * time.Hour)
	seedBot(t, st, "seller", now)
	seedOffer(t, st, "offer", "seller", now)
	for _, id := range []string{"a", "b", "c", "boundary", "outside", "unpaid"} {
		seedBot(t, st, id, now)
	}
	for _, row := range []struct {
		id, buyer, status        string
		paid, delivered, expired time.Time
	}{
		{"a1", "a", "PAID", now.Add(-time.Hour), time.Time{}, time.Time{}},
		{"a2", "a", "DELIVERED", now.Add(-2 * time.Hour), now.Add(-time.Hour), time.Time{}},
		{"b1", "b", "EXPIRED", now.Add(-24 * time.Hour), time.Time{}, now.Add(-time.Hour)},
		{"c1", "c", "PAID", now.Add(-time.Hour), time.Time{}, time.Time{}},
		// A recent delivery of an older purchase is outside this paid cohort.
		{"c_old", "c", "DELIVERED", start.Add(-time.Hour), now.Add(-time.Hour), time.Time{}},
		{"at_start", "boundary", "PAID", start, time.Time{}, time.Time{}},
		{"before_start", "boundary", "PAID", start.Add(-time.Nanosecond), time.Time{}, time.Time{}},
		{"at_end", "outside", "PAID", now, time.Time{}, time.Time{}},
		{"future", "outside", "PAID", now.Add(time.Hour), time.Time{}, time.Time{}},
		{"unpaid", "unpaid", "REQUESTED", time.Time{}, time.Time{}, time.Time{}},
		{"charge", "unpaid", "CHARGE_CREATED", time.Time{}, time.Time{}, time.Time{}},
		{"cancelled", "unpaid", "CANCELLED", time.Time{}, time.Time{}, time.Time{}},
		// Never infer a payment time from status or job creation time.
		{"missing_time", "unpaid", "PAID", time.Time{}, time.Time{}, time.Time{}},
	} {
		seedJob(t, st, row.id, "offer", row.buyer, "seller", row.status, start.Add(-time.Hour), sql.NullTime{}, nullableTime(row.expired), nullableTime(row.delivered))
		if _, err := db.Exec(`UPDATE jobs SET paid_at = ? WHERE job_id = ?`, nullableTime(row.paid), row.id); err != nil {
			t.Fatal(err)
		}
	}
	// Revocation does not erase past demand; recipient events cannot duplicate it.
	if _, err := st.UpdateBotRevoke(ctx, sqlc.UpdateBotRevokeParams{BotID: "b", RevokedAt: nullableTime(now)}); err != nil {
		t.Fatal(err)
	}
	seedEvent(t, st, "a", "job.paid", map[string]any{"job_id": "a1"}, now)
	seedEvent(t, st, "seller", "job.paid", map[string]any{"job_id": "a1"}, now)
	stats, err := st.GetDemandStats(ctx, now.In(time.FixedZone("offset", 7200)))
	if err != nil {
		t.Fatal(err)
	}
	want := DemandStats{28, start, now, 5, 1, 4, 1}
	if stats != want {
		t.Fatalf("got %+v, want %+v", stats, want)
	}
}

func TestDemandStatsDeliveryEvidence(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()
	st := New(db)
	now := time.Date(2026, 10, 7, 19, 0, 0, 0, time.UTC)
	seedBot(t, st, "buyer", now)
	seedBot(t, st, "seller", now)
	seedOffer(t, st, "offer", "seller", now)
	paid := now.Add(-time.Hour)
	for i, delivered := range []time.Time{time.Time{}, now, now.Add(time.Hour), paid.Add(-time.Second), now.Add(-time.Second)} {
		id := string(rune('a' + i))
		seedJob(t, st, id, "offer", "buyer", "seller", "DELIVERED", paid, sql.NullTime{}, sql.NullTime{}, nullableTime(delivered))
		if _, err := db.Exec(`UPDATE jobs SET paid_at = ? WHERE job_id = ?`, paid, id); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := st.GetDemandStats(context.Background(), now)
	if err != nil || stats.PaidJobs != 5 || stats.DeliveredJobs != 1 || stats.UniqueBuyers != 1 || stats.RepeatBuyers != 1 {
		t.Fatalf("stats=%+v, err=%v", stats, err)
	}
}

func TestDemandStatsSurvivesRetention(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()
	st := New(db)
	ctx := context.Background()
	now := time.Date(2026, 10, 7, 19, 0, 0, 0, time.UTC)
	seedBot(t, st, "buyer", now)
	seedBot(t, st, "seller", now)
	seedOffer(t, st, "offer", "seller", now)
	for _, row := range []struct {
		id string
		at time.Time
	}{
		{"old", now.Add(-31 * 24 * time.Hour)},
		{"recent", now.Add(-27 * 24 * time.Hour)},
	} {
		seedJob(t, st, row.id, "offer", "buyer", "seller", "DELIVERED", row.at, sql.NullTime{}, sql.NullTime{}, nullableTime(row.at))
		if _, err := db.Exec(`UPDATE jobs SET paid_at = ? WHERE job_id = ?`, row.at, row.id); err != nil {
			t.Fatal(err)
		}
	}
	before, err := st.GetDemandStats(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteJobsTerminalBefore(ctx, nullableTime(now.Add(-30*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	assertJobDeleted(t, st, "old")
	after, err := st.GetDemandStats(ctx, now)
	if err != nil || before != after || after.PaidJobs != 1 || after.RepeatBuyers != 0 {
		t.Fatalf("before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestDemandStatsReissueIsOneJob(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()
	st := New(db)
	ctx := context.Background()
	now := time.Now().UTC()
	seedBot(t, st, "buyer", now)
	seedBot(t, st, "seller", now)
	seedOffer(t, st, "offer", "seller", now)
	seedJob(t, st, "job", "offer", "buyer", "seller", "CHARGE_CREATED", now.Add(-time.Hour), sql.NullTime{}, sql.NullTime{}, sql.NullTime{})
	params := sqlc.UpdateJobMarkPaidParams{JobID: "job", PaidAt: nullableTime(now.Add(-time.Hour))}
	for i := 0; i < 2; i++ {
		if err := st.UpdateJobMarkPaid(ctx, params); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.UpdateJobExpire(ctx, sqlc.UpdateJobExpireParams{JobID: "job", ExpiredAt: nullableTime(now.Add(-time.Minute))}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateJobChargeReissue(ctx, sqlc.UpdateJobChargeReissueParams{JobID: "job"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		stats, err := st.GetDemandStats(ctx, now)
		if err != nil || stats.PaidJobs != 1 || stats.UniqueBuyers != 1 || stats.RepeatBuyers != 0 {
			t.Fatalf("stats=%+v err=%v", stats, err)
		}
		params.PaidAt = nullableTime(now.Add(-time.Second))
		if err := st.UpdateJobMarkPaid(ctx, params); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDemandStatsEmptyAndUnavailable(t *testing.T) {
	db := setupStoreTestDB(t)
	st := New(db)
	stats, err := st.GetDemandStats(context.Background(), time.Now())
	if err != nil || stats.PaidJobs != 0 || stats.DeliveredJobs != 0 || stats.UniqueBuyers != 0 || stats.RepeatBuyers != 0 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	db.Close()
	for _, unavailable := range []*Store{nil, {}, st} {
		if _, err := unavailable.GetDemandStats(context.Background(), time.Now()); err == nil {
			t.Fatal("expected unavailable error")
		}
	}
}

func TestDemandMetricsMigration(t *testing.T) {
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
	if err := goose.UpTo(db, migrations, 10); err != nil {
		t.Fatal(err)
	}
	st := New(db)
	now := time.Now().UTC()
	seedBot(t, st, "buyer", now)
	seedBot(t, st, "seller", now)
	seedOffer(t, st, "offer", "seller", now)
	seedJob(t, st, "job", "offer", "buyer", "seller", "CHARGE_CREATED", now.Add(-time.Hour), sql.NullTime{}, sql.NullTime{}, sql.NullTime{})
	if err := st.UpdateJobMarkPaid(context.Background(), sqlc.UpdateJobMarkPaidParams{JobID: "job", PaidAt: nullableTime(now.Add(-time.Hour))}); err != nil {
		t.Fatal(err)
	}
	before, err := st.GetJob(context.Background(), "job")
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db, migrations, 11); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'idx_jobs_paid_at'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("index count=%d err=%v", count, err)
	}
	if err := goose.Down(db, migrations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'idx_jobs_paid_at'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("index count=%d err=%v", count, err)
	}
	after, err := st.GetJob(context.Background(), "job")
	if err != nil || before != after {
		t.Fatalf("migration changed commerce data: %v", err)
	}
}

func nullableTime(at time.Time) sql.NullTime {
	return sql.NullTime{Time: at, Valid: !at.IsZero()}
}
