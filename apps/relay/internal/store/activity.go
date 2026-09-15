package store

import (
	"context"
	"time"
)

// RecordBotActivity records server-verified contact at most once per minute.
// The predicate is atomic so delayed requests cannot move the timestamp backwards.
func (s *Store) RecordBotActivity(ctx context.Context, botID string, at time.Time) error {
	at = at.UTC()
	_, err := s.DB.ExecContext(ctx, `UPDATE bots SET last_seen_at = ?
 WHERE bot_id = ? AND revoked_at IS NULL
 AND (last_seen_at IS NULL OR julianday(last_seen_at) <= julianday(?))`,
		at, botID, at.Add(-time.Minute))
	return err
}
