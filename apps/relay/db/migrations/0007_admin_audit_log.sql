-- +goose Up
CREATE TABLE IF NOT EXISTS admin_audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	action TEXT NOT NULL,
	target_type TEXT NOT NULL,
	target_id TEXT NOT NULL,
	reason TEXT NOT NULL,
	note TEXT,
	request_id TEXT,
	token_fingerprint TEXT,
	remote_addr TEXT,
	user_agent TEXT,
	before_json TEXT,
	after_json TEXT,
	created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_audit_log_created_at ON admin_audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_target ON admin_audit_log(target_type, target_id);

-- +goose Down
DROP TABLE IF EXISTS admin_audit_log;

