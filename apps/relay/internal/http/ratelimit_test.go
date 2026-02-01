package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanobazaar/relay/internal/ratelimit"
	"github.com/nanobazaar/relay/internal/store"
)

func TestRateLimitHeadersAnd429(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		OfferSearch: ratelimit.BucketConfig{Rate: 1, Burst: 1},
	})

	router := NewRouter(RouterConfig{Store: st, Limiter: limiter})

	req := httptest.NewRequest(http.MethodGet, "/v0/offers", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-RateLimit-Limit") == "" {
		t.Fatalf("expected X-RateLimit-Limit header")
	}
	if rec.Header().Get("X-RateLimit-Remaining") == "" {
		t.Fatalf("expected X-RateLimit-Remaining header")
	}
	if rec.Header().Get("X-RateLimit-Reset") == "" {
		t.Fatalf("expected X-RateLimit-Reset header")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v0/offers", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	rec2 := httptestRequest(t, router, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if rec2.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header")
	}
}
