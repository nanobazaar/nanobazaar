-- +goose Up
DROP TRIGGER IF EXISTS offers_fts_au;
DROP TRIGGER IF EXISTS offers_fts_ad;
DROP TRIGGER IF EXISTS offers_fts_ai;

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
	DELETE FROM offers_fts WHERE rowid = old.rowid;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER offers_fts_au
AFTER UPDATE ON offers
BEGIN
	DELETE FROM offers_fts WHERE rowid = old.rowid;
	INSERT INTO offers_fts(rowid, offer_id, title, description, tags)
	VALUES (new.rowid, new.offer_id, new.title, new.description, new.tags_json);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS offers_fts_au;
DROP TRIGGER IF EXISTS offers_fts_ad;
DROP TRIGGER IF EXISTS offers_fts_ai;

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
