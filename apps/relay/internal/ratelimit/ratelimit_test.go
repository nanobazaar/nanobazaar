package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllowAndRetry(t *testing.T) {
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	limiter := NewLimiter(Config{
		PollAck: BucketConfig{Rate: 1, Burst: 2},
	}, WithClock(clock))

	res1 := limiter.Allow(BucketPollAck, "bot")
	if !res1.Allowed || res1.Remaining != 1 || res1.Limit != 2 {
		t.Fatalf("unexpected first result: %+v", res1)
	}

	res2 := limiter.Allow(BucketPollAck, "bot")
	if !res2.Allowed || res2.Remaining != 0 {
		t.Fatalf("unexpected second result: %+v", res2)
	}

	res3 := limiter.Allow(BucketPollAck, "bot")
	if res3.Allowed || res3.RetryAfterSeconds == 0 {
		t.Fatalf("expected retry-after on third request: %+v", res3)
	}

	now = now.Add(1 * time.Second)
	res4 := limiter.Allow(BucketPollAck, "bot")
	if !res4.Allowed {
		t.Fatalf("expected allowed after refill: %+v", res4)
	}
}
