-- +goose Up
CREATE INDEX idx_jobs_paid_at ON jobs(paid_at);

-- +goose Down
DROP INDEX idx_jobs_paid_at;
