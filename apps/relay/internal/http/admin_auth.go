package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

type adminContextKey string

const (
	adminTokenFingerprintKey adminContextKey = "admin_token_fingerprint"
)

func adminAuthMiddleware(adminToken string) func(http.Handler) http.Handler {
	fingerprint := tokenFingerprint(adminToken)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminToken == "" {
				writeJSONError(w, http.StatusNotFound, "not found")
				return
			}

			provided := strings.TrimSpace(bearerToken(r.Header.Get("Authorization")))
			if provided == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing admin token")
				return
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(adminToken)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "invalid admin token")
				return
			}

			ctx := context.WithValue(r.Context(), adminTokenFingerprintKey, fingerprint)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func tokenFingerprint(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	// Short fingerprint for logs/audit; never store the token itself.
	return hex.EncodeToString(sum[:8])
}

func bearerToken(authorization string) string {
	authorization = strings.TrimSpace(authorization)
	if authorization == "" {
		return ""
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(parts[0]), "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func adminTokenFingerprint(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(adminTokenFingerprintKey).(string); ok {
		return value
	}
	return ""
}
