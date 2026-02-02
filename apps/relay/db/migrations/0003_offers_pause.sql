-- +goose Up
PRAGMA foreign_keys = OFF;

DROP TRIGGER IF EXISTS offers_fts_au;
DROP TRIGGER IF EXISTS offers_fts_ad;
DROP TRIGGER IF EXISTS offers_fts_ai;
DROP TABLE IF EXISTS offers_fts;

CREATE TABLE offers_new (
	offer_id TEXT PRIMARY KEY,
	seller_bot_id TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	tags_json TEXT NOT NULL,
	price_raw TEXT NOT NULL,
	turnaround_seconds INTEGER NOT NULL,
	created_at DATETIME NOT NULL,
	expires_at DATETIME,
	status TEXT NOT NULL CHECK (status IN ('ACTIVE', 'PAUSED', 'CANCELLED', 'EXPIRED')),
	cancelled_at DATETIME,
	request_schema_hint TEXT,
	FOREIGN KEY (seller_bot_id) REFERENCES bots(bot_id)
);

INSERT INTO offers_new (
	offer_id,
	seller_bot_id,
	title,
	description,
	tags_json,
	price_raw,
	turnaround_seconds,
	created_at,
	expires_at,
	status,
	cancelled_at,
	request_schema_hint
)
SELECT
	offer_id,
	seller_bot_id,
	title,
	description,
	tags_json,
	price_raw,
	turnaround_seconds,
	created_at,
	expires_at,
	status,
	cancelled_at,
	request_schema_hint
FROM offers;

DROP TABLE offers;
ALTER TABLE offers_new RENAME TO offers;

CREATE INDEX idx_offers_seller_created_at ON offers(seller_bot_id, created_at DESC);
CREATE INDEX idx_offers_status ON offers(status);
CREATE INDEX idx_offers_created_at ON offers(created_at DESC);
CREATE INDEX idx_offers_expires_at ON offers(expires_at);

CREATE VIRTUAL TABLE offers_fts USING fts5(
	offer_id UNINDEXED,
	title,
	description,
	tags,
	tokenize = 'unicode61'
);

-- +goose StatementBegin
CREATE TRIGGER offers_fts_ai
AFTER INSERT ON offers
BEGIN
	INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
	VALUES (new.rowid, new.offer_id, new.title, new.description, new.tags_json);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER offers_fts_ad
AFTER DELETE ON offers
BEGIN
	INSERT INTO offers_fts(offers_fts, rowid, offer_id, title, description, tags)
	VALUES ('delete', old.rowid, old.offer_id, old.title, old.description, old.tags_json);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER offers_fts_au
AFTER UPDATE ON offers
BEGIN
	INSERT INTO offers_fts(offers_fts, rowid, offer_id, title, description, tags)
	VALUES ('delete', old.rowid, old.offer_id, old.title, old.description, old.tags_json);
	INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
	VALUES (new.rowid, new.offer_id, new.title, new.description, new.tags_json);
END;
-- +goose StatementEnd

INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
SELECT rowid, offer_id, title, description, tags_json
FROM offers;

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP TRIGGER IF EXISTS offers_fts_au;
DROP TRIGGER IF EXISTS offers_fts_ad;
DROP TRIGGER IF EXISTS offers_fts_ai;
DROP TABLE IF EXISTS offers_fts;

CREATE TABLE offers_new (
	offer_id TEXT PRIMARY KEY,
	seller_bot_id TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	tags_json TEXT NOT NULL,
	price_raw TEXT NOT NULL,
	turnaround_seconds INTEGER NOT NULL,
	created_at DATETIME NOT NULL,
	expires_at DATETIME,
	status TEXT NOT NULL CHECK (status IN ('ACTIVE', 'CANCELLED', 'EXPIRED')),
	cancelled_at DATETIME,
	request_schema_hint TEXT,
	FOREIGN KEY (seller_bot_id) REFERENCES bots(bot_id)
);

INSERT INTO offers_new (
	offer_id,
	seller_bot_id,
	title,
	description,
	tags_json,
	price_raw,
	turnaround_seconds,
	created_at,
	expires_at,
	status,
	cancelled_at,
	request_schema_hint
)
SELECT
	offer_id,
	seller_bot_id,
	title,
	description,
	tags_json,
	price_raw,
	turnaround_seconds,
	created_at,
	expires_at,
	CASE WHEN status = 'PAUSED' THEN 'ACTIVE' ELSE status END,
	cancelled_at,
	request_schema_hint
FROM offers;

DROP TABLE offers;
ALTER TABLE offers_new RENAME TO offers;

CREATE INDEX idx_offers_seller_created_at ON offers(seller_bot_id, created_at DESC);
CREATE INDEX idx_offers_status ON offers(status);
CREATE INDEX idx_offers_created_at ON offers(created_at DESC);
CREATE INDEX idx_offers_expires_at ON offers(expires_at);

CREATE VIRTUAL TABLE offers_fts USING fts5(
	offer_id UNINDEXED,
	title,
	description,
	tags,
	tokenize = 'unicode61'
);

-- +goose StatementBegin
CREATE TRIGGER offers_fts_ai
AFTER INSERT ON offers
BEGIN
	INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
	VALUES (new.rowid, new.offer_id, new.title, new.description, new.tags_json);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER offers_fts_ad
AFTER DELETE ON offers
BEGIN
	INSERT INTO offers_fts(offers_fts, rowid, offer_id, title, description, tags)
	VALUES ('delete', old.rowid, old.offer_id, old.title, old.description, old.tags_json);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER offers_fts_au
AFTER UPDATE ON offers
BEGIN
	INSERT INTO offers_fts(offers_fts, rowid, offer_id, title, description, tags)
	VALUES ('delete', old.rowid, old.offer_id, old.title, old.description, old.tags_json);
	INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
	VALUES (new.rowid, new.offer_id, new.title, new.description, new.tags_json);
END;
-- +goose StatementEnd

INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
SELECT rowid, offer_id, title, description, tags_json
FROM offers;

PRAGMA foreign_keys = ON;
