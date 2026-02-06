package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type AdminAuditEntry struct {
	Action           string
	TargetType       string
	TargetID         string
	Reason           string
	Note             string
	RequestID        string
	TokenFingerprint string
	RemoteAddr       string
	UserAgent        string
	Before           any
	After            any
	CreatedAt        time.Time
}

type AdminAuditRow struct {
	ID               int64
	Action           string
	TargetType       string
	TargetID         string
	Reason           string
	Note             string
	RequestID        string
	TokenFingerprint string
	RemoteAddr       string
	UserAgent        string
	BeforeJSON       string
	AfterJSON        string
	CreatedAt        time.Time
}

type AdminAuditCursor struct {
	CreatedAt time.Time
	ID        int64
}

func (s *Store) InsertAdminAudit(ctx context.Context, entry AdminAuditEntry) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("audit store unavailable")
	}
	return insertAdminAudit(ctx, s.DB, entry)
}

func (s *Store) InsertAdminAuditTx(ctx context.Context, tx *sql.Tx, entry AdminAuditEntry) (int64, error) {
	if s == nil || tx == nil {
		return 0, fmt.Errorf("audit store unavailable")
	}
	return insertAdminAudit(ctx, tx, entry)
}

type adminAuditExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertAdminAudit(ctx context.Context, execer adminAuditExecer, entry AdminAuditEntry) (int64, error) {
	if execer == nil {
		return 0, fmt.Errorf("audit store unavailable")
	}

	var beforeJSON string
	if entry.Before != nil {
		b, err := json.Marshal(entry.Before)
		if err != nil {
			return 0, err
		}
		beforeJSON = string(b)
	}
	var afterJSON string
	if entry.After != nil {
		b, err := json.Marshal(entry.After)
		if err != nil {
			return 0, err
		}
		afterJSON = string(b)
	}

	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	} else {
		createdAt = createdAt.UTC()
	}

	result, err := execer.ExecContext(ctx, `
INSERT INTO admin_audit_log (
	action,
	target_type,
	target_id,
	reason,
	note,
	request_id,
	token_fingerprint,
	remote_addr,
	user_agent,
	before_json,
	after_json,
	created_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)`,
		entry.Action,
		entry.TargetType,
		entry.TargetID,
		entry.Reason,
		nullIfEmpty(entry.Note),
		nullIfEmpty(entry.RequestID),
		nullIfEmpty(entry.TokenFingerprint),
		nullIfEmpty(entry.RemoteAddr),
		nullIfEmpty(entry.UserAgent),
		nullIfEmpty(beforeJSON),
		nullIfEmpty(afterJSON),
		createdAt,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) ListAdminAudit(ctx context.Context, limit int, cursor *AdminAuditCursor, targetType string, targetID string) ([]AdminAuditRow, *AdminAuditCursor, error) {
	if s == nil || s.DB == nil {
		return nil, nil, fmt.Errorf("audit store unavailable")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Keyset pagination.
	args := []any{targetType, targetID}
	whereCursor := ""
	if cursor != nil {
		whereCursor = "AND (created_at < ?3 OR (created_at = ?3 AND id < ?4))"
		args = append(args, cursor.CreatedAt.UTC(), cursor.ID)
	}
	args = append(args, limit+1)

	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`
SELECT id, action, target_type, target_id, reason, note, request_id, token_fingerprint, remote_addr, user_agent, before_json, after_json, created_at
FROM admin_audit_log
WHERE (?1 = '' OR target_type = ?1)
	AND (?2 = '' OR target_id = ?2)
	%s
ORDER BY created_at DESC, id DESC
LIMIT ?%d`, whereCursor, len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	out := make([]AdminAuditRow, 0, limit+1)
	for rows.Next() {
		var row AdminAuditRow
		var note sql.NullString
		var requestID sql.NullString
		var tokenFingerprint sql.NullString
		var remoteAddr sql.NullString
		var userAgent sql.NullString
		var beforeJSON sql.NullString
		var afterJSON sql.NullString
		if err := rows.Scan(
			&row.ID,
			&row.Action,
			&row.TargetType,
			&row.TargetID,
			&row.Reason,
			&note,
			&requestID,
			&tokenFingerprint,
			&remoteAddr,
			&userAgent,
			&beforeJSON,
			&afterJSON,
			&row.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		row.Note = note.String
		row.RequestID = requestID.String
		row.TokenFingerprint = tokenFingerprint.String
		row.RemoteAddr = remoteAddr.String
		row.UserAgent = userAgent.String
		row.BeforeJSON = beforeJSON.String
		row.AfterJSON = afterJSON.String
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var next *AdminAuditCursor
	if len(out) > limit {
		last := out[limit-1]
		next = &AdminAuditCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		out = out[:limit]
	}
	return out, next, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
