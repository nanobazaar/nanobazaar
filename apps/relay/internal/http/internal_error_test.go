package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestLogInternalError_LogsForNonCanceledErrors(t *testing.T) {
	prev := internalErrorLogf
	t.Cleanup(func() { internalErrorLogf = prev })

	var got string
	internalErrorLogf = func(format string, args ...any) {
		got = fmt.Sprintf(format, args...)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.test/v0/jobs/123?x=1", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	req.Header.Set(headerBotID, "b_test")
	req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "req-123"))

	logInternalError(req, "test_action", http.StatusInternalServerError, errors.New("boom"))

	if got == "" {
		t.Fatalf("expected log line, got empty")
	}
	if !strings.Contains(got, "internal_error ") {
		t.Fatalf("expected internal_error prefix, got %q", got)
	}
	if !strings.Contains(got, "action=test_action") {
		t.Fatalf("expected action, got %q", got)
	}
	if !strings.Contains(got, "status=500") {
		t.Fatalf("expected status=500, got %q", got)
	}
	if !strings.Contains(got, "method=GET") {
		t.Fatalf("expected method, got %q", got)
	}
	if !strings.Contains(got, "path=/v0/jobs/123") {
		t.Fatalf("expected path, got %q", got)
	}
	if !strings.Contains(got, "request_id=req-123") {
		t.Fatalf("expected request_id, got %q", got)
	}
	if !strings.Contains(got, "bot_id=b_test") {
		t.Fatalf("expected bot_id, got %q", got)
	}
	if !strings.Contains(got, "remote_addr=1.2.3.4:5678") {
		t.Fatalf("expected remote_addr, got %q", got)
	}
	if !strings.Contains(got, "err=boom") {
		t.Fatalf("expected err, got %q", got)
	}
}

func TestLogInternalError_SkipsCanceledAndDeadlineErrors(t *testing.T) {
	prev := internalErrorLogf
	t.Cleanup(func() { internalErrorLogf = prev })

	called := false
	internalErrorLogf = func(string, ...any) { called = true }

	req := httptest.NewRequest(http.MethodGet, "http://example.test/v0/jobs/123", nil)

	logInternalError(req, "test_action", http.StatusInternalServerError, context.Canceled)
	if called {
		t.Fatalf("expected no log for context.Canceled")
	}

	logInternalError(req, "test_action", http.StatusInternalServerError, fmt.Errorf("wrapped: %w", context.Canceled))
	if called {
		t.Fatalf("expected no log for wrapped context.Canceled")
	}

	logInternalError(req, "test_action", http.StatusInternalServerError, context.DeadlineExceeded)
	if called {
		t.Fatalf("expected no log for context.DeadlineExceeded")
	}
}
