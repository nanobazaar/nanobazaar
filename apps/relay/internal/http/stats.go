package httpapi

import (
	"net/http"
	"time"

	"github.com/nanobazaar/relay/internal/store"
)

type StatsHandler struct {
	Store *store.Store
}

type statsResponse struct {
	Offers         int64               `json:"offers"`
	Jobs           int64               `json:"jobs"`
	AgentsOnline   int64               `json:"agents_online"`
	XnoTransferred string              `json:"xno_transferred"`
	Demand         demandStatsResponse `json:"demand"`
}

type demandStatsResponse struct {
	WindowDays    int    `json:"window_days"`
	WindowStart   string `json:"window_start"`
	WindowEnd     string `json:"window_end"`
	PaidJobs      int64  `json:"paid_jobs"`
	DeliveredJobs int64  `json:"delivered_jobs"`
	UniqueBuyers  int64  `json:"unique_buyers"`
	RepeatBuyers  int64  `json:"repeat_buyers"`
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
		Demand: demandStatsResponse{
			WindowDays:    stats.Demand.WindowDays,
			WindowStart:   stats.Demand.WindowStart.Format(time.RFC3339Nano),
			WindowEnd:     stats.Demand.WindowEnd.Format(time.RFC3339Nano),
			PaidJobs:      stats.Demand.PaidJobs,
			DeliveredJobs: stats.Demand.DeliveredJobs,
			UniqueBuyers:  stats.Demand.UniqueBuyers,
			RepeatBuyers:  stats.Demand.RepeatBuyers,
		},
	})
}
