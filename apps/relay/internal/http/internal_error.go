package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// internalErrorLogf exists for testability; do not call directly.
var internalErrorLogf = log.Printf

func logInternalError(r *http.Request, action string, status int, err error) {
	if r == nil || err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}

	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}

	route := routePattern(r)
	reqID := middleware.GetReqID(r.Context())
	botID := strings.TrimSpace(r.Header.Get(headerBotID))

	internalErrorLogf(
		"internal_error action=%s status=%d method=%s path=%s route=%s bot_id=%s request_id=%s remote_addr=%s err=%v",
		action,
		status,
		r.Method,
		path,
		route,
		botID,
		reqID,
		r.RemoteAddr,
		err,
	)
}

func internalErrorActionFromMessage(message string) string {
	action := strings.ToLower(strings.TrimSpace(message))
	action = strings.ReplaceAll(action, " ", "_")
	action = strings.ReplaceAll(action, "-", "_")
	action = strings.Trim(action, "_")
	if action == "" {
		return "internal_error"
	}
	return action
}

func writeJSONInternalError(w http.ResponseWriter, r *http.Request, message string, err error) {
	logInternalError(r, internalErrorActionFromMessage(message), http.StatusInternalServerError, err)
	writeJSONError(w, http.StatusInternalServerError, message)
}

func writeJSONInternalErrorAction(w http.ResponseWriter, r *http.Request, action string, message string, err error) {
	logInternalError(r, action, http.StatusInternalServerError, err)
	writeJSONError(w, http.StatusInternalServerError, message)
}
