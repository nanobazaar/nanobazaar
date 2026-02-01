-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS bots (
	bot_id TEXT PRIMARY KEY,
	signing_pubkey_ed25519 TEXT NOT NULL,
	encryption_pubkey_x25519 TEXT NOT NULL,
	signing_kid TEXT NOT NULL,
	encryption_kid TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	last_seen_at DATETIME
);

CREATE TABLE IF NOT EXISTS nonces (
	bot_id TEXT NOT NULL,
	nonce TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	PRIMARY KEY (bot_id, nonce)
);

CREATE INDEX IF NOT EXISTS idx_nonces_created_at ON nonces(created_at);

CREATE TABLE IF NOT EXISTS idempotency_keys (
	bot_id TEXT NOT NULL,
	endpoint TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	request_hash TEXT NOT NULL,
	response_code INTEGER NOT NULL,
	response_body TEXT NOT NULL,
	response_headers TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	PRIMARY KEY (bot_id, endpoint, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_created_at ON idempotency_keys(created_at);

CREATE TABLE IF NOT EXISTS offers (
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

CREATE INDEX IF NOT EXISTS idx_offers_seller_created_at ON offers(seller_bot_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_offers_status ON offers(status);
CREATE INDEX IF NOT EXISTS idx_offers_created_at ON offers(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_offers_expires_at ON offers(expires_at);

CREATE TABLE IF NOT EXISTS offer_tags (
	offer_id TEXT NOT NULL,
	tag TEXT NOT NULL,
	PRIMARY KEY (offer_id, tag),
	FOREIGN KEY (offer_id) REFERENCES offers(offer_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_offer_tags_tag ON offer_tags(tag);

CREATE TABLE IF NOT EXISTS jobs (
	job_id TEXT PRIMARY KEY,
	offer_id TEXT NOT NULL,
	buyer_bot_id TEXT NOT NULL,
	seller_bot_id TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('REQUESTED', 'CHARGE_CREATED', 'PAID', 'DELIVERED', 'CANCELLED', 'EXPIRED')),
	price_raw TEXT NOT NULL,
	turnaround_seconds INTEGER NOT NULL,
	created_at DATETIME NOT NULL,
	job_expires_at DATETIME NOT NULL,
	request_payload_id TEXT NOT NULL,
	charge_id TEXT,
	charge_address TEXT,
	charge_amount_raw TEXT,
	charge_expires_at DATETIME,
	charge_sig_ed25519 TEXT,
	paid_at DATETIME,
	delivered_at DATETIME,
	cancelled_at DATETIME,
	expired_at DATETIME,
	payment_verifier TEXT,
	payment_block_hash TEXT,
	payment_observed_at DATETIME,
	amount_raw_received TEXT,
	FOREIGN KEY (offer_id) REFERENCES offers(offer_id),
	FOREIGN KEY (buyer_bot_id) REFERENCES bots(bot_id),
	FOREIGN KEY (seller_bot_id) REFERENCES bots(bot_id)
);

CREATE INDEX IF NOT EXISTS idx_jobs_buyer_created_at ON jobs(buyer_bot_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_seller_created_at ON jobs(seller_bot_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_expires_at ON jobs(job_expires_at);
CREATE INDEX IF NOT EXISTS idx_jobs_charge_address ON jobs(seller_bot_id, charge_address);
CREATE INDEX IF NOT EXISTS idx_jobs_cancelled_at ON jobs(cancelled_at);
CREATE INDEX IF NOT EXISTS idx_jobs_expired_at ON jobs(expired_at);
CREATE INDEX IF NOT EXISTS idx_jobs_delivered_at ON jobs(delivered_at);

CREATE TABLE IF NOT EXISTS payloads (
	payload_id TEXT NOT NULL,
	recipient_bot_id TEXT NOT NULL,
	job_id TEXT NOT NULL,
	sender_bot_id TEXT NOT NULL,
	payload_kind TEXT NOT NULL CHECK (payload_kind IN ('request', 'deliverable', 'message')),
	enc_alg TEXT NOT NULL,
	recipient_kid TEXT NOT NULL,
	ciphertext_b64 TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	fetched_at DATETIME,
	PRIMARY KEY (payload_id, recipient_bot_id),
	FOREIGN KEY (job_id) REFERENCES jobs(job_id),
	FOREIGN KEY (sender_bot_id) REFERENCES bots(bot_id),
	FOREIGN KEY (recipient_bot_id) REFERENCES bots(bot_id)
);

CREATE INDEX IF NOT EXISTS idx_payloads_recipient_created_at ON payloads(recipient_bot_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payloads_recipient_fetched_at ON payloads(recipient_bot_id, fetched_at);
CREATE INDEX IF NOT EXISTS idx_payloads_job ON payloads(job_id);
CREATE INDEX IF NOT EXISTS idx_payloads_created_at ON payloads(created_at);
CREATE INDEX IF NOT EXISTS idx_payloads_fetched_at ON payloads(fetched_at);

CREATE TABLE IF NOT EXISTS events (
	event_id INTEGER PRIMARY KEY AUTOINCREMENT,
	recipient_bot_id TEXT NOT NULL,
	event_type TEXT NOT NULL,
	data_json TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	FOREIGN KEY (recipient_bot_id) REFERENCES bots(bot_id)
);

CREATE INDEX IF NOT EXISTS idx_events_recipient_event_id ON events(recipient_bot_id, event_id);
CREATE INDEX IF NOT EXISTS idx_events_recipient_created_at ON events(recipient_bot_id, created_at);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_recipient_type_event_id ON events(recipient_bot_id, event_type, event_id);

CREATE TABLE IF NOT EXISTS poll_acks (
	recipient_bot_id TEXT PRIMARY KEY,
	last_acked_event_id INTEGER NOT NULL,
	updated_at DATETIME NOT NULL,
	FOREIGN KEY (recipient_bot_id) REFERENCES bots(bot_id)
);

-- +goose Down
DROP TABLE IF EXISTS poll_acks;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS payloads;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS offer_tags;
DROP TABLE IF EXISTS offers;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS nonces;
DROP TABLE IF EXISTS bots;
