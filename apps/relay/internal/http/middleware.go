package httpapi

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/ratelimit"
)

func metricsMiddleware(registry *metrics.Registry) func(http.Handler) http.Handler {
	if registry == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			status := ww.Status()
			route := routePattern(r)
			registry.RecordRequest(r.Method, route, status, time.Since(start))
		})
	}
}

func errorLogMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			status := ww.Status()
			if status >= 500 {
				reqID := middleware.GetReqID(r.Context())
				path := ""
				if r.URL != nil {
					path = r.URL.Path
				}
				log.Printf("http_5xx status=%d method=%s path=%s bot_id=%s request_id=%s remote_addr=%s", status, r.Method, path, r.Header.Get(headerBotID), reqID, r.RemoteAddr)
			}
		})
	}
}

func rateLimitMiddleware(limiter *ratelimit.Limiter, bucket string, registry *metrics.Registry) func(http.Handler) http.Handler {
	if limiter == nil || !limiter.Enabled(bucket) {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r)
			result := limiter.Allow(bucket, key)
			if result.Limit > 0 {
				resetAt := time.Now().Add(time.Duration(result.ResetSeconds) * time.Second).Unix()
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
			}
			if !result.Allowed {
				if result.RetryAfterSeconds > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(result.RetryAfterSeconds))
				}
				if registry != nil {
					registry.RecordRateLimit(bucket)
				}
				logRateLimit(bucket, key, r)
				writeJSONError(w, http.StatusTooManyRequests, "rate limited")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func healthMiddleware(public bool) func(http.Handler) http.Handler {
	if public {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isLoopbackRequest(r) {
				writeJSONError(w, http.StatusUnauthorized, "health endpoint restricted")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func routePattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	if ctx := chi.RouteContext(r.Context()); ctx != nil {
		if pattern := ctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	if r.URL != nil {
		return r.URL.Path
	}
	return ""
}

func rateLimitKey(r *http.Request) string {
	if botID := strings.TrimSpace(r.Header.Get(headerBotID)); botID != "" {
		return botID
	}
	host := r.RemoteAddr
	if host == "" {
		return "unknown"
	}
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return parsed
	}
	return host
}

func isLoopbackRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	addr := r.RemoteAddr
	if addr == "" {
		return false
	}
	host := addr
	if parsed, _, err := net.SplitHostPort(addr); err == nil {
		host = parsed
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func logRateLimit(bucket, key string, r *http.Request) {
	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}
	log.Printf("rate_limited bucket=%s key=%s method=%s path=%s remote_addr=%s", bucket, key, r.Method, path, r.RemoteAddr)
}
