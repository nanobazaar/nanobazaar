package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/nanobazaar/relay/internal/store"
)

func authorizeStreamKey(ctx context.Context, st *store.Store, caller, stream string) (string, error) {
	if st == nil {
		return "", errors.New("store unavailable")
	}
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return "", &streamHTTPError{Status: http.StatusBadRequest, Message: "missing stream"}
	}

	switch {
	case strings.HasPrefix(stream, "seller:ed25519:"):
		pub := strings.TrimSpace(strings.TrimPrefix(stream, "seller:ed25519:"))
		pubBytes, err := decodeKey(pub, 32)
		if err != nil {
			return "", &streamHTTPError{Status: http.StatusBadRequest, Message: "invalid seller stream"}
		}
		derived := botIDFromSigningKey(pubBytes)
		if derived != caller {
			return "", &streamHTTPError{Status: http.StatusForbidden, Message: "forbidden"}
		}
		bot, err := st.GetBot(ctx, caller)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", &streamHTTPError{Status: http.StatusNotFound, Message: "bot not found"}
			}
			return "", err
		}
		if bot.SigningPubkeyEd25519 != pub {
			return "", &streamHTTPError{Status: http.StatusBadRequest, Message: "seller stream mismatch"}
		}
		return "seller:ed25519:" + pub, nil
	case strings.HasPrefix(stream, "job:"):
		jobID := strings.TrimSpace(strings.TrimPrefix(stream, "job:"))
		if jobID == "" {
			return "", &streamHTTPError{Status: http.StatusBadRequest, Message: "invalid job stream"}
		}
		job, err := st.GetJob(ctx, jobID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", &streamHTTPError{Status: http.StatusNotFound, Message: "job not found"}
			}
			return "", err
		}
		if job.BuyerBotID != caller && job.SellerBotID != caller {
			return "", &streamHTTPError{Status: http.StatusForbidden, Message: "forbidden"}
		}
		return "job:" + jobID, nil
	default:
		return "", &streamHTTPError{Status: http.StatusBadRequest, Message: "unknown stream"}
	}
}
