-- +goose Up
ALTER TABLE bots ADD COLUMN bot_name TEXT;

-- +goose Down
PRAGMA foreign_keys = OFF;
BEGIN;

CREATE TABLE bots_new (
	bot_id TEXT PRIMARY KEY,
	signing_pubkey_ed25519 TEXT NOT NULL,
	encryption_pubkey_x25519 TEXT NOT NULL,
	signing_kid TEXT NOT NULL,
	encryption_kid TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	last_seen_at DATETIME,
	revoked_at DATETIME
);

INSERT INTO bots_new (
	bot_id,
	signing_pubkey_ed25519,
	encryption_pubkey_x25519,
	signing_kid,
	encryption_kid,
	created_at,
	last_seen_at,
	revoked_at
)
SELECT
	bot_id,
	signing_pubkey_ed25519,
	encryption_pubkey_x25519,
	signing_kid,
	encryption_kid,
	created_at,
	last_seen_at,
	revoked_at
FROM bots;

DROP TABLE bots;
ALTER TABLE bots_new RENAME TO bots;

CREATE INDEX IF NOT EXISTS idx_bots_revoked_at ON bots(revoked_at);

COMMIT;
PRAGMA foreign_keys = ON;

