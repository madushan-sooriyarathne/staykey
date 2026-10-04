-- Codes and refresh tokens are stored as hashes. Times come from the API's clock so the rules
-- can be tested without waiting.

-- name: LockPhone :exec
-- Serializes code requests for one number inside a transaction, so rate limits hold under load.
SELECT pg_advisory_xact_lock(hashtextextended(@phone::text, 0));

-- name: OTPsForPhoneSince :one
-- How many codes went to a number in the window, and when the oldest of them was sent.
SELECT count(*)::int AS sent, coalesce(min(created_at), @since::timestamptz)::timestamptz AS oldest
FROM otp_challenges
WHERE phone = @phone AND created_at > @since::timestamptz;

-- name: OTPsForIPSince :one
SELECT count(*)::int AS sent, coalesce(min(created_at), @since::timestamptz)::timestamptz AS oldest
FROM otp_challenges
WHERE ip = @ip AND created_at > @since::timestamptz;

-- name: LastOTPSentAt :one
SELECT created_at FROM otp_challenges WHERE phone = @phone ORDER BY created_at DESC LIMIT 1;

-- name: CreateOTPChallenge :exec
INSERT INTO otp_challenges (id, phone, code_hash, ip, expires_at, created_at)
VALUES (@id, @phone, @code_hash, sqlc.narg('ip'), @expires_at, @created_at);

-- name: GetOpenOTPForUpdate :one
-- The latest unused, unexpired code for a number. Older codes stop counting once a new one is sent.
SELECT * FROM otp_challenges
WHERE phone = @phone AND consumed_at IS NULL AND expires_at > @now
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: RecordOTPAttempt :exec
UPDATE otp_challenges SET attempts = attempts + 1 WHERE id = @id;

-- name: ConsumeOTP :exec
UPDATE otp_challenges SET consumed_at = @now::timestamptz WHERE id = @id;

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, refresh_hash, device_name, rotated_at, last_used_at, expires_at, created_at)
VALUES (@id, @user_id, @refresh_hash, @device_name, @now, @now, @expires_at, @now);

-- name: GetSessionForUpdate :one
SELECT * FROM sessions WHERE id = @id FOR UPDATE;

-- name: RotateSession :exec
UPDATE sessions
SET previous_refresh_hash = refresh_hash,
    refresh_hash          = @refresh_hash,
    rotated_at            = @now,
    last_used_at          = @now,
    expires_at            = @expires_at
WHERE id = @id;

-- name: ReissueSession :exec
-- A retry inside the grace period: replace the token the client never received, and keep the
-- previous hash and rotation time so the grace period cannot be stretched.
UPDATE sessions
SET refresh_hash = @refresh_hash,
    last_used_at = @now,
    expires_at   = @expires_at
WHERE id = @id;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = @now::timestamptz, revoked_reason = @reason::text
WHERE id = @id AND revoked_at IS NULL;

-- name: GetActiveSessionUser :one
SELECT user_id FROM sessions WHERE id = @id AND revoked_at IS NULL AND expires_at > @now;
