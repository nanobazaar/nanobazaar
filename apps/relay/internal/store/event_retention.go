package store

import (
	"context"

	"github.com/nanobazaar/relay/internal/store/sqlc"
)

type EventRetention struct {
	DeletedThroughEventID int64
	MinEventIDRetained    int64
}

// ReadEventRetention accepts the poll's transaction so its deletion boundary,
// ACK and returned events all come from the same SQLite snapshot.
func ReadEventRetention(ctx context.Context, db sqlc.DBTX, recipient string) (EventRetention, error) {
	var result EventRetention
	err := db.QueryRowContext(ctx, `SELECT
		COALESCE((SELECT deleted_through_event_id FROM event_retention WHERE recipient_bot_id = ?), 0),
		COALESCE((SELECT MIN(event_id) FROM events WHERE recipient_bot_id = ?), 0)`, recipient, recipient).
		Scan(&result.DeletedThroughEventID, &result.MinEventIDRetained)
	if err != nil {
		return EventRetention{}, err
	}
	// min-1 must be a safe resync ACK even if every event was removed or a
	// newer event was deleted while an older event is still retained.
	if result.DeletedThroughEventID > 0 && result.MinEventIDRetained <= result.DeletedThroughEventID {
		result.MinEventIDRetained = result.DeletedThroughEventID + 1
	}
	return result, nil
}
