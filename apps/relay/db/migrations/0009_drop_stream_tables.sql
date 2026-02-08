-- +goose Up
-- Stream polling is removed; /v0/poll is the single durable cursor model.
DROP TABLE IF EXISTS stream_acks;
DROP TABLE IF EXISTS stream_events;

-- +goose Down
-- Recreate stream polling tables (deprecated).
CREATE TABLE IF NOT EXISTS stream_events (
	stream_key TEXT NOT NULL,
	cursor INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	payload_json TEXT NOT NULL,
	PRIMARY KEY (stream_key, cursor)
);

CREATE INDEX IF NOT EXISTS idx_stream_events_created_at ON stream_events(created_at);
CREATE INDEX IF NOT EXISTS idx_stream_events_stream_created_at ON stream_events(stream_key, created_at);

CREATE TABLE IF NOT EXISTS stream_acks (
	stream_key TEXT PRIMARY KEY,
	ack_cursor INTEGER NOT NULL,
	updated_at DATETIME NOT NULL
);

