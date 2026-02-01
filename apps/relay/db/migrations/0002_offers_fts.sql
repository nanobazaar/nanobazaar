-- +goose Up
CREATE VIRTUAL TABLE IF NOT EXISTS offers_fts USING fts5(
	offer_id UNINDEXED,
	title,
	description,
	tags,
	tokenize = 'unicode61'
);

CREATE TRIGGER IF NOT EXISTS offers_fts_ai
AFTER INSERT ON offers
BEGIN
	INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
	VALUES (new.rowid, new.offer_id, new.title, new.description, new.tags_json);
END;

CREATE TRIGGER IF NOT EXISTS offers_fts_ad
AFTER DELETE ON offers
BEGIN
	INSERT INTO offers_fts(offers_fts, rowid, offer_id, title, description, tags)
	VALUES ('delete', old.rowid, old.offer_id, old.title, old.description, old.tags_json);
END;

CREATE TRIGGER IF NOT EXISTS offers_fts_au
AFTER UPDATE ON offers
BEGIN
	INSERT INTO offers_fts(offers_fts, rowid, offer_id, title, description, tags)
	VALUES ('delete', old.rowid, old.offer_id, old.title, old.description, old.tags_json);
	INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
	VALUES (new.rowid, new.offer_id, new.title, new.description, new.tags_json);
END;

INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
SELECT rowid, offer_id, title, description, tags_json
FROM offers;

-- +goose Down
DROP TRIGGER IF EXISTS offers_fts_au;
DROP TRIGGER IF EXISTS offers_fts_ad;
DROP TRIGGER IF EXISTS offers_fts_ai;
DROP TABLE IF EXISTS offers_fts;
