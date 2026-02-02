-- NanoBazaar Relay v0.2 queries

-- Bots
-- name: CreateBot :exec
INSERT INTO bots (
	bot_id,
	signing_pubkey_ed25519,
	encryption_pubkey_x25519,
	signing_kid,
	encryption_kid,
	created_at,
	last_seen_at
) VALUES (
	sqlc.arg(bot_id),
	sqlc.arg(signing_pubkey_ed25519),
	sqlc.arg(encryption_pubkey_x25519),
	sqlc.arg(signing_kid),
	sqlc.arg(encryption_kid),
	sqlc.arg(created_at),
	sqlc.arg(last_seen_at)
);

-- name: GetBot :one
SELECT * FROM bots WHERE bot_id = sqlc.arg(bot_id);

-- name: UpdateBotLastSeen :exec
UPDATE bots
SET last_seen_at = sqlc.arg(last_seen_at)
WHERE bot_id = sqlc.arg(bot_id);

-- Nonces (replay protection)
-- name: InsertNonce :exec
INSERT INTO nonces (
	bot_id,
	nonce,
	created_at
) VALUES (
	sqlc.arg(bot_id),
	sqlc.arg(nonce),
	sqlc.arg(created_at)
);

-- name: CountNonce :one
SELECT COUNT(1) FROM nonces
WHERE bot_id = sqlc.arg(bot_id)
	AND nonce = sqlc.arg(nonce);

-- name: DeleteNoncesBefore :exec
DELETE FROM nonces
WHERE created_at < sqlc.arg(cutoff);

-- Idempotency
-- name: GetIdempotency :one
SELECT * FROM idempotency_keys
WHERE bot_id = sqlc.arg(bot_id)
	AND endpoint = sqlc.arg(endpoint)
	AND idempotency_key = sqlc.arg(idempotency_key);

-- name: InsertIdempotency :exec
INSERT INTO idempotency_keys (
	bot_id,
	endpoint,
	idempotency_key,
	request_hash,
	response_code,
	response_body,
	response_headers,
	created_at
) VALUES (
	sqlc.arg(bot_id),
	sqlc.arg(endpoint),
	sqlc.arg(idempotency_key),
	sqlc.arg(request_hash),
	sqlc.arg(response_code),
	sqlc.arg(response_body),
	sqlc.arg(response_headers),
	sqlc.arg(created_at)
);

-- name: DeleteIdempotencyBefore :exec
DELETE FROM idempotency_keys
WHERE created_at < sqlc.arg(cutoff);

-- Offers
-- name: CreateOffer :exec
INSERT INTO offers (
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
) VALUES (
	sqlc.arg(offer_id),
	sqlc.arg(seller_bot_id),
	sqlc.arg(title),
	sqlc.arg(description),
	sqlc.arg(tags_json),
	sqlc.arg(price_raw),
	sqlc.arg(turnaround_seconds),
	sqlc.arg(created_at),
	sqlc.arg(expires_at),
	sqlc.arg(status),
	sqlc.arg(cancelled_at),
	sqlc.arg(request_schema_hint)
);

-- name: GetOffer :one
SELECT * FROM offers WHERE offer_id = sqlc.arg(offer_id);

-- name: ListOffersNewest :many
SELECT * FROM offers
WHERE (sqlc.arg(seller_bot_id) = '' OR seller_bot_id = sqlc.arg(seller_bot_id))
	AND (sqlc.arg(status) = '' OR status = sqlc.arg(status))
	AND (sqlc.arg(query) = '' OR title LIKE '%' || sqlc.arg(query) || '%' OR description LIKE '%' || sqlc.arg(query) || '%')
ORDER BY created_at DESC, offer_id DESC
LIMIT sqlc.arg(limit);

-- name: ListOffersNewestAfter :many
SELECT * FROM offers
WHERE (sqlc.arg(seller_bot_id) = '' OR seller_bot_id = sqlc.arg(seller_bot_id))
	AND (sqlc.arg(status) = '' OR status = sqlc.arg(status))
	AND (sqlc.arg(query) = '' OR title LIKE '%' || sqlc.arg(query) || '%' OR description LIKE '%' || sqlc.arg(query) || '%')
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND offer_id < sqlc.arg(cursor_offer_id))
	)
ORDER BY created_at DESC, offer_id DESC
LIMIT sqlc.arg(limit);

-- name: UpdateOfferCancel :exec
UPDATE offers
SET status = 'CANCELLED',
	cancelled_at = sqlc.arg(cancelled_at)
WHERE offer_id = sqlc.arg(offer_id)
	AND status IN ('ACTIVE', 'PAUSED');

-- name: UpdateOfferExpire :exec
UPDATE offers
SET status = 'EXPIRED'
WHERE offer_id = sqlc.arg(offer_id)
	AND status IN ('ACTIVE', 'PAUSED');

-- name: UpdateOfferPause :exec
UPDATE offers
SET status = 'PAUSED'
WHERE offer_id = sqlc.arg(offer_id)
	AND status = 'ACTIVE';

-- name: UpdateOfferResume :exec
UPDATE offers
SET status = 'ACTIVE'
WHERE offer_id = sqlc.arg(offer_id)
	AND status = 'PAUSED';

-- name: InsertOfferTag :exec
INSERT INTO offer_tags (
	offer_id,
	tag
) VALUES (
	sqlc.arg(offer_id),
	sqlc.arg(tag)
);

-- name: ListOfferTags :many
SELECT tag FROM offer_tags
WHERE offer_id = sqlc.arg(offer_id)
ORDER BY tag ASC;

-- name: ListOfferIDsByTag :many
SELECT offer_id FROM offer_tags
WHERE tag = sqlc.arg(tag);

-- Jobs
-- name: CreateJob :exec
INSERT INTO jobs (
	job_id,
	offer_id,
	buyer_bot_id,
	seller_bot_id,
	status,
	price_raw,
	turnaround_seconds,
	created_at,
	job_expires_at,
	request_payload_id,
	charge_id,
	charge_address,
	charge_amount_raw,
	charge_expires_at,
	charge_sig_ed25519,
	paid_at,
	delivered_at,
	cancelled_at,
	expired_at,
	payment_verifier,
	payment_block_hash,
	payment_observed_at,
	amount_raw_received
) VALUES (
	sqlc.arg(job_id),
	sqlc.arg(offer_id),
	sqlc.arg(buyer_bot_id),
	sqlc.arg(seller_bot_id),
	sqlc.arg(status),
	sqlc.arg(price_raw),
	sqlc.arg(turnaround_seconds),
	sqlc.arg(created_at),
	sqlc.arg(job_expires_at),
	sqlc.arg(request_payload_id),
	sqlc.arg(charge_id),
	sqlc.arg(charge_address),
	sqlc.arg(charge_amount_raw),
	sqlc.arg(charge_expires_at),
	sqlc.arg(charge_sig_ed25519),
	sqlc.arg(paid_at),
	sqlc.arg(delivered_at),
	sqlc.arg(cancelled_at),
	sqlc.arg(expired_at),
	sqlc.arg(payment_verifier),
	sqlc.arg(payment_block_hash),
	sqlc.arg(payment_observed_at),
	sqlc.arg(amount_raw_received)
);

-- name: GetJob :one
SELECT * FROM jobs WHERE job_id = sqlc.arg(job_id);

-- name: ListJobsByBuyerNewest :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsByBuyerNewestWithStatus :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
	AND status IN (sqlc.slice(statuses))
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsByBuyerNewestAfter :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsByBuyerNewestAfterWithStatus :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
	AND status IN (sqlc.slice(statuses))
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsByBuyerSince :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
	AND created_at >= sqlc.arg(created_since)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsByBuyerSinceWithStatus :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
	AND created_at >= sqlc.arg(created_since)
	AND status IN (sqlc.slice(statuses))
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsByBuyerSinceAfter :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
	AND created_at >= sqlc.arg(created_since)
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsByBuyerSinceAfterWithStatus :many
SELECT * FROM jobs
WHERE buyer_bot_id = sqlc.arg(buyer_bot_id)
	AND created_at >= sqlc.arg(created_since)
	AND status IN (sqlc.slice(statuses))
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerNewest :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerNewestWithStatus :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND status IN (sqlc.slice(statuses))
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerNewestAfter :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerNewestAfterWithStatus :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND status IN (sqlc.slice(statuses))
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerSince :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND created_at >= sqlc.arg(created_since)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerSinceWithStatus :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND created_at >= sqlc.arg(created_since)
	AND status IN (sqlc.slice(statuses))
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerSinceAfter :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND created_at >= sqlc.arg(created_since)
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: ListJobsBySellerSinceAfterWithStatus :many
SELECT * FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND created_at >= sqlc.arg(created_since)
	AND status IN (sqlc.slice(statuses))
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND job_id < sqlc.arg(cursor_job_id))
	)
ORDER BY created_at DESC, job_id DESC
LIMIT sqlc.arg(limit);

-- name: UpdateJobCancel :exec
UPDATE jobs
SET status = 'CANCELLED',
	cancelled_at = sqlc.arg(cancelled_at)
WHERE job_id = sqlc.arg(job_id)
	AND status = 'REQUESTED';

-- name: UpdateJobCharge :exec
UPDATE jobs
SET status = 'CHARGE_CREATED',
	charge_id = sqlc.arg(charge_id),
	charge_address = sqlc.arg(charge_address),
	charge_amount_raw = sqlc.arg(charge_amount_raw),
	charge_expires_at = sqlc.arg(charge_expires_at),
	charge_sig_ed25519 = sqlc.arg(charge_sig_ed25519)
WHERE job_id = sqlc.arg(job_id)
	AND status = 'REQUESTED'
	AND charge_id IS NULL;

-- name: UpdateJobMarkPaid :exec
UPDATE jobs
SET status = 'PAID',
	paid_at = sqlc.arg(paid_at),
	payment_verifier = sqlc.arg(payment_verifier),
	payment_block_hash = sqlc.arg(payment_block_hash),
	payment_observed_at = sqlc.arg(payment_observed_at),
	amount_raw_received = sqlc.arg(amount_raw_received)
WHERE job_id = sqlc.arg(job_id)
	AND status = 'CHARGE_CREATED';

-- name: UpdateJobDeliver :exec
UPDATE jobs
SET status = 'DELIVERED',
	delivered_at = sqlc.arg(delivered_at)
WHERE job_id = sqlc.arg(job_id)
	AND status = 'PAID';

-- name: UpdateJobExpire :exec
UPDATE jobs
SET status = 'EXPIRED',
	expired_at = sqlc.arg(expired_at)
WHERE job_id = sqlc.arg(job_id)
	AND status IN ('REQUESTED', 'CHARGE_CREATED', 'PAID');

-- name: CountActiveJobsByChargeAddress :one
SELECT COUNT(1) FROM jobs
WHERE seller_bot_id = sqlc.arg(seller_bot_id)
	AND charge_address = sqlc.arg(charge_address)
	AND status IN ('REQUESTED', 'CHARGE_CREATED', 'PAID');

-- Payloads
-- name: CreatePayload :exec
INSERT INTO payloads (
	payload_id,
	recipient_bot_id,
	job_id,
	sender_bot_id,
	payload_kind,
	enc_alg,
	recipient_kid,
	ciphertext_b64,
	created_at,
	fetched_at
) VALUES (
	sqlc.arg(payload_id),
	sqlc.arg(recipient_bot_id),
	sqlc.arg(job_id),
	sqlc.arg(sender_bot_id),
	sqlc.arg(payload_kind),
	sqlc.arg(enc_alg),
	sqlc.arg(recipient_kid),
	sqlc.arg(ciphertext_b64),
	sqlc.arg(created_at),
	sqlc.arg(fetched_at)
);

-- name: GetPayload :one
SELECT * FROM payloads
WHERE payload_id = sqlc.arg(payload_id)
	AND recipient_bot_id = sqlc.arg(recipient_bot_id);

-- name: GetPayloadRecipient :one
SELECT recipient_bot_id FROM payloads
WHERE payload_id = sqlc.arg(payload_id)
LIMIT 1;

-- name: ListPayloadMetadataUnfetched :many
SELECT payload_id,
	job_id,
	sender_bot_id,
	recipient_bot_id,
	payload_kind,
	enc_alg,
	recipient_kid,
	created_at,
	fetched_at
FROM payloads
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND (sqlc.arg(job_id) = '' OR job_id = sqlc.arg(job_id))
	AND fetched_at IS NULL
ORDER BY created_at DESC, payload_id DESC
LIMIT sqlc.arg(limit);

-- name: ListPayloadMetadataUnfetchedAfter :many
SELECT payload_id,
	job_id,
	sender_bot_id,
	recipient_bot_id,
	payload_kind,
	enc_alg,
	recipient_kid,
	created_at,
	fetched_at
FROM payloads
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND (sqlc.arg(job_id) = '' OR job_id = sqlc.arg(job_id))
	AND fetched_at IS NULL
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND payload_id < sqlc.arg(cursor_payload_id))
	)
ORDER BY created_at DESC, payload_id DESC
LIMIT sqlc.arg(limit);

-- name: ListPayloadMetadataFetched :many
SELECT payload_id,
	job_id,
	sender_bot_id,
	recipient_bot_id,
	payload_kind,
	enc_alg,
	recipient_kid,
	created_at,
	fetched_at
FROM payloads
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND (sqlc.arg(job_id) = '' OR job_id = sqlc.arg(job_id))
	AND fetched_at IS NOT NULL
ORDER BY created_at DESC, payload_id DESC
LIMIT sqlc.arg(limit);

-- name: ListPayloadMetadataFetchedAfter :many
SELECT payload_id,
	job_id,
	sender_bot_id,
	recipient_bot_id,
	payload_kind,
	enc_alg,
	recipient_kid,
	created_at,
	fetched_at
FROM payloads
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND (sqlc.arg(job_id) = '' OR job_id = sqlc.arg(job_id))
	AND fetched_at IS NOT NULL
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND payload_id < sqlc.arg(cursor_payload_id))
	)
ORDER BY created_at DESC, payload_id DESC
LIMIT sqlc.arg(limit);

-- name: ListPayloadMetadataAll :many
SELECT payload_id,
	job_id,
	sender_bot_id,
	recipient_bot_id,
	payload_kind,
	enc_alg,
	recipient_kid,
	created_at,
	fetched_at
FROM payloads
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND (sqlc.arg(job_id) = '' OR job_id = sqlc.arg(job_id))
ORDER BY created_at DESC, payload_id DESC
LIMIT sqlc.arg(limit);

-- name: ListPayloadMetadataAllAfter :many
SELECT payload_id,
	job_id,
	sender_bot_id,
	recipient_bot_id,
	payload_kind,
	enc_alg,
	recipient_kid,
	created_at,
	fetched_at
FROM payloads
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND (sqlc.arg(job_id) = '' OR job_id = sqlc.arg(job_id))
	AND (
		created_at < sqlc.arg(cursor_created_at)
		OR (created_at = sqlc.arg(cursor_created_at) AND payload_id < sqlc.arg(cursor_payload_id))
	)
ORDER BY created_at DESC, payload_id DESC
LIMIT sqlc.arg(limit);

-- name: MarkPayloadFetched :exec
UPDATE payloads
SET fetched_at = sqlc.arg(fetched_at)
WHERE payload_id = sqlc.arg(payload_id)
	AND recipient_bot_id = sqlc.arg(recipient_bot_id);

-- name: DeletePayloadsBefore :exec
DELETE FROM payloads
WHERE created_at < sqlc.arg(cutoff);

-- name: DeletePayloadsFetchedBefore :exec
DELETE FROM payloads
WHERE fetched_at IS NOT NULL
	AND fetched_at < sqlc.arg(cutoff);

-- name: DeleteOffersBefore :exec
DELETE FROM offers
WHERE created_at < sqlc.arg(cutoff);

-- name: DeleteJobsTerminalBefore :exec
DELETE FROM jobs
WHERE status IN ('CANCELLED', 'EXPIRED', 'DELIVERED')
	AND (
		(status = 'CANCELLED' AND cancelled_at < sqlc.arg(cutoff))
		OR (status = 'EXPIRED' AND expired_at < sqlc.arg(cutoff))
		OR (status = 'DELIVERED' AND delivered_at < sqlc.arg(cutoff))
	);

-- Events
-- name: CreateEvent :exec
INSERT INTO events (
	recipient_bot_id,
	event_type,
	data_json,
	created_at
) VALUES (
	sqlc.arg(recipient_bot_id),
	sqlc.arg(event_type),
	sqlc.arg(data_json),
	sqlc.arg(created_at)
);

-- name: ListEventsAfterID :many
SELECT event_id,
	recipient_bot_id,
	event_type,
	data_json,
	created_at
FROM events
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND event_id > sqlc.arg(since_event_id)
ORDER BY event_id ASC
LIMIT sqlc.arg(limit);

-- name: ListEventsAfterIDByTypes :many
SELECT event_id,
	recipient_bot_id,
	event_type,
	data_json,
	created_at
FROM events
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND event_id > sqlc.arg(since_event_id)
	AND event_type IN (sqlc.slice(event_types))
ORDER BY event_id ASC
LIMIT sqlc.arg(limit);

-- name: GetEventCreatedAt :one
SELECT created_at FROM events
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id)
	AND event_id = sqlc.arg(event_id);

-- name: GetMinEventID :one
SELECT MIN(event_id) FROM events
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id);

-- name: DeleteEventsBefore :exec
DELETE FROM events
WHERE created_at < sqlc.arg(cutoff);

-- Poll acks
-- name: GetPollAck :one
SELECT * FROM poll_acks
WHERE recipient_bot_id = sqlc.arg(recipient_bot_id);

-- name: UpsertPollAck :exec
INSERT INTO poll_acks (
	recipient_bot_id,
	last_acked_event_id,
	updated_at
) VALUES (
	sqlc.arg(recipient_bot_id),
	sqlc.arg(last_acked_event_id),
	sqlc.arg(updated_at)
)
ON CONFLICT(recipient_bot_id) DO UPDATE SET
	last_acked_event_id = excluded.last_acked_event_id,
	updated_at = excluded.updated_at;
