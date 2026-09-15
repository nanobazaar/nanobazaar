package store

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRecordBotActivityIsMonotonicAndThrottled(t *testing.T) {
	db := setupStoreTestDB(t)
	defer db.Close()
	st := New(db)
	ctx := context.Background()
	now := time.Date(2026, 9, 15, 6, 0, 0, 0, time.UTC)
	seedBot(t, st, "active", now)
	for _, step := range []struct{ at, want time.Time }{
		{now, now}, {now.Add(30 * time.Second), now}, {now.Add(-time.Hour), now},
		{now.Add(time.Minute), now.Add(time.Minute)}, {now.Add(10 * time.Minute), now.Add(10 * time.Minute)},
	} {
		if err := st.RecordBotActivity(ctx, "active", step.at); err != nil {
			t.Fatal(err)
		}
		bot, err := st.GetBot(ctx, "active")
		if err != nil {
			t.Fatal(err)
		}
		if !bot.LastSeenAt.Valid || !bot.LastSeenAt.Time.Equal(step.want) {
			t.Fatalf("at %s: %v, want %s", step.at, bot.LastSeenAt, step.want)
		}
	}
	// Simulate out-of-order concurrent contact recording.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := st.RecordBotActivity(ctx, "active", now.Add(time.Duration(i)*time.Minute)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	bot, err := st.GetBot(ctx, "active")
	if err != nil {
		t.Fatal(err)
	}
	if !bot.LastSeenAt.Time.Equal(now.Add(19 * time.Minute)) {
		t.Fatalf("regressed: %v", bot.LastSeenAt)
	}
	if _, err := db.Exec("UPDATE bots SET revoked_at = ? WHERE bot_id = 'active'", now); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordBotActivity(ctx, "active", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	bot, err = st.GetBot(ctx, "active")
	if err != nil {
		t.Fatal(err)
	}
	if !bot.LastSeenAt.Time.Equal(now.Add(19 * time.Minute)) {
		t.Fatal("revoked bot was refreshed")
	}
	if err := st.RecordBotActivity(ctx, "missing", now); err != nil {
		t.Fatal(err)
	}
}
