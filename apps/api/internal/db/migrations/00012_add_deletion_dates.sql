-- Deleting yourself takes 30 days to finish. Until then the person is signed out everywhere,
-- the accounts they own are closed (their booking pages go offline), and signing in again with
-- the same number restores both. A job deletes what is left once the 30 days are up.

-- +goose Up
ALTER TABLE users ADD COLUMN deleted_at timestamptz;
ALTER TABLE accounts ADD COLUMN closed_at timestamptz;

CREATE INDEX users_deleted_at_idx ON users (deleted_at) WHERE deleted_at IS NOT NULL;

ALTER TABLE sessions DROP CONSTRAINT sessions_revoked_reason_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_revoked_reason_check
    CHECK (revoked_reason IN ('logout', 'reuse', 'deleted'));

-- +goose Down
UPDATE sessions SET revoked_reason = 'logout' WHERE revoked_reason = 'deleted';
ALTER TABLE sessions DROP CONSTRAINT sessions_revoked_reason_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_revoked_reason_check
    CHECK (revoked_reason IN ('logout', 'reuse'));
DROP INDEX users_deleted_at_idx;
ALTER TABLE accounts DROP COLUMN closed_at;
ALTER TABLE users DROP COLUMN deleted_at;
