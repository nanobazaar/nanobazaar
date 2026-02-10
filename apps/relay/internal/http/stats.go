package httpapi

import (
	"net/http"

	"github.com/nanobazaar/relay/internal/store"
)

type StatsHandler struct {
	Store *store.Store
}

type statsResponse struct {
	Offers         int64  `json:"offers"`
	Jobs           int64  `json:"jobs"`
	AgentsOnline   int64  `json:"agents_online"`
	XnoTransferred string `json:"xno_transferred"`
}

func NewStatsHandler(store *store.Store) *StatsHandler {
	return &StatsHandler{Store: store}
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "stats unavailable")
		return
	}

	stats, err := h.Store.GetRelayStats(r.Context())
	if err != nil {
		writeJSONInternalError(w, r, "stats unavailable", err)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, statsResponse{
		Offers:         stats.Offers,
		Jobs:           stats.Jobs,
		AgentsOnline:   stats.AgentsOnline,
		XnoTransferred: stats.XnoTransferred,
	})
}
