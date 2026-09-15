-- +goose Up
CREATE TABLE event_retention (
	recipient_bot_id TEXT PRIMARY KEY,
	deleted_through_event_id INTEGER NOT NULL CHECK (deleted_through_event_id >= 0),
	FOREIGN KEY (recipient_bot_id) REFERENCES bots(bot_id)
);

-- Old deletions have no recipient history. Preserve the previous conservative
-- boundary for bots with retained events; an empty legacy log uses the last
-- allocated event ID. Existing bots may need one resync. Bots registered after
-- this migration start without a watermark and are not affected by global gaps.
INSERT INTO event_retention (recipient_bot_id, deleted_through_event_id)
SELECT bots.bot_id, COALESCE(
	(SELECT MIN(event_id) - 1 FROM events WHERE recipient_bot_id = bots.bot_id),
	(SELECT seq FROM sqlite_sequence WHERE name = 'events'),
	0
)
FROM bots;

-- Record deletion in the same transaction, including direct SQL cleanup.
-- +goose StatementBegin
CREATE TRIGGER events_retention_ad
AFTER DELETE ON events
BEGIN
	INSERT INTO event_retention (recipient_bot_id, deleted_through_event_id)
	VALUES (old.recipient_bot_id, old.event_id)
	ON CONFLICT (recipient_bot_id) DO UPDATE SET
		deleted_through_event_id = MAX(deleted_through_event_id, excluded.deleted_through_event_id);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER events_retention_ad;
DROP TABLE event_retention;
