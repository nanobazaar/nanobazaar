package store

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/nanobazaar/relay/internal/domain"
)

const nanoRawDecimals = 30

var nanoRawUnit = new(big.Int).Exp(big.NewInt(10), big.NewInt(nanoRawDecimals), nil)

type RelayStats struct {
	Offers         int64
	Jobs           int64
	AgentsOnline   int64
	XnoTransferred string
	Demand         DemandStats
}

// Stay inside the 30-day terminal-job retention horizon, including cleanup races.
const demandWindowDays = 28

type DemandStats struct {
	WindowDays    int
	WindowStart   time.Time
	WindowEnd     time.Time
	PaidJobs      int64
	DeliveredJobs int64
	UniqueBuyers  int64
	RepeatBuyers  int64
}

// GetDemandStats counts distinct jobs, not payment attempts or recipient events.
// paid_at survives expiry/reissue; its latest confirmation determines the window.
// Delivery is a subset of this paid cohort, not all deliveries in the window.
func (s *Store) GetDemandStats(ctx context.Context, now time.Time) (DemandStats, error) {
	stats := DemandStats{
		WindowDays:  demandWindowDays,
		WindowStart: now.UTC().Add(-demandWindowDays * 24 * time.Hour),
		WindowEnd:   now.UTC(),
	}
	if s == nil || s.DB == nil {
		return stats, fmt.Errorf("stats store unavailable")
	}
	err := s.DB.QueryRowContext(ctx, `
		WITH buyers AS (
			SELECT buyer_bot_id, COUNT(*) AS paid_jobs,
				SUM(CASE WHEN delivered_at >= paid_at AND delivered_at < ?2 THEN 1 ELSE 0 END) AS delivered_jobs
			FROM jobs
			WHERE paid_at >= ?1 AND paid_at < ?2
			GROUP BY buyer_bot_id
		)
		SELECT COALESCE(SUM(paid_jobs), 0), COALESCE(SUM(delivered_jobs), 0),
			COUNT(*), COALESCE(SUM(CASE WHEN paid_jobs >= 2 THEN 1 ELSE 0 END), 0)
		FROM buyers`, stats.WindowStart, stats.WindowEnd).Scan(
		&stats.PaidJobs, &stats.DeliveredJobs, &stats.UniqueBuyers, &stats.RepeatBuyers,
	)
	return stats, err
}

func (s *Store) GetRelayStats(ctx context.Context) (RelayStats, error) {
	var stats RelayStats
	if s == nil || s.DB == nil {
		return stats, fmt.Errorf("stats store unavailable")
	}

	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM offers WHERE status IN (?1, ?2)`, domain.OfferActive, domain.OfferPaused).Scan(&stats.Offers); err != nil {
		return stats, err
	}

	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM jobs WHERE status IN ('PAID', 'DELIVERED')`).Scan(&stats.Jobs); err != nil {
		return stats, err
	}

	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM bots WHERE revoked_at IS NULL`).Scan(&stats.AgentsOnline); err != nil {
		return stats, err
	}

	rows, err := s.DB.QueryContext(ctx, `SELECT amount_raw_received FROM jobs WHERE status IN ('PAID', 'DELIVERED') AND amount_raw_received IS NOT NULL`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	totalRaw := big.NewInt(0)
	var rawInt big.Int
	for rows.Next() {
		var rawStr string
		if err := rows.Scan(&rawStr); err != nil {
			return stats, err
		}
		rawStr = strings.TrimSpace(rawStr)
		if rawStr == "" {
			continue
		}
		if _, ok := rawInt.SetString(rawStr, 10); !ok {
			return stats, fmt.Errorf("invalid amount_raw_received %q", rawStr)
		}
		totalRaw.Add(totalRaw, &rawInt)
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	stats.XnoTransferred = formatRawAsNano(totalRaw)
	// Release rows before the next query, including on single-connection stores.
	if err := rows.Close(); err != nil {
		return stats, err
	}
	stats.Demand, err = s.GetDemandStats(ctx, time.Now())
	if err != nil {
		return stats, err
	}
	return stats, nil
}

func formatRawAsNano(raw *big.Int) string {
	if raw == nil || raw.Sign() == 0 {
		return "0"
	}
	if raw.Sign() < 0 {
		abs := new(big.Int).Abs(raw)
		return "-" + formatRawAsNano(abs)
	}

	intPart := new(big.Int).Quo(raw, nanoRawUnit)
	rem := new(big.Int).Mod(raw, nanoRawUnit)
	if rem.Sign() == 0 {
		return intPart.String()
	}

	frac := rem.String()
	if len(frac) < nanoRawDecimals {
		frac = strings.Repeat("0", nanoRawDecimals-len(frac)) + frac
	}
	frac = strings.TrimRight(frac, "0")
	return intPart.String() + "." + frac
}
