package metrics

import (
	"context"
	"encoding/json"
	"net/http"
)

type PayloadStatsFunc func(ctx context.Context) (pending int64, bytes int64, err error)

func NewHandler(registry *Registry, payloadStats PayloadStatsFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if payloadStats != nil {
			if pending, bytes, err := payloadStats(r.Context()); err == nil {
				registry.SetPayloadStats(pending, bytes)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(registry.Snapshot())
	})
}
