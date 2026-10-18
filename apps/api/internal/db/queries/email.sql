-- The email outbox, suppressions, address confirmations and timed guest emails.

-- name: QueueEmail :execrows
INSERT INTO emails (id, account_id, booking_id, user_id, kind, to_address, payload, dedupe_key, scheduled_for)
VALUES (@id, sqlc.narg('account_id'), sqlc.narg('booking_id'), sqlc.narg('user_id'), @kind, @to_address,
        @payload, sqlc.narg('dedupe_key'), @scheduled_for)
ON CONFLICT (dedupe_key) DO NOTHING;

-- Takes due emails for one run and counts the attempt. A claim from before stale_before is from
-- a run that stopped mid-send, so it is taken again; attempts bound how often that can happen.
-- name: ClaimDueEmails :many
UPDATE emails SET claimed_at = @now::timestamptz, attempts = attempts + 1
WHERE id IN (
    SELECT id FROM emails
    WHERE status = 'pending' AND scheduled_for <= @now::timestamptz
      AND (claimed_at IS NULL OR claimed_at < @stale_before::timestamptz)
    ORDER BY scheduled_for, id
    LIMIT @row_limit
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkEmail :exec
UPDATE emails SET status = @status, sent_at = sqlc.narg('sent_at'),
                  provider_message_id = sqlc.narg('provider_message_id'), last_error = sqlc.narg('last_error')
WHERE id = @id;

-- Puts an email back after a failed send, to try again at retry_at.
-- name: ReleaseEmail :exec
UPDATE emails SET claimed_at = NULL, scheduled_for = @retry_at::timestamptz, last_error = @last_error
WHERE id = @id;

-- name: IsSuppressed :one
SELECT EXISTS (SELECT 1 FROM email_suppressions WHERE address = lower(@address::text));

-- A suppressed address no longer counts as confirmed, so its owner is asked for a working one.
-- name: Suppress :exec
WITH added AS (
    INSERT INTO email_suppressions (address, reason) VALUES (lower(@address::text), @reason)
    ON CONFLICT (address) DO NOTHING
)
UPDATE users SET email_confirmed_at = NULL WHERE email = lower(@address::text);

-- name: AccountClosed :one
SELECT status = 'closed' AS closed FROM accounts WHERE id = @id;

-- Everything a guest email is written from. owner_email is the first owner's confirmed address,
-- the Reply-To. template_body and template_channels are empty for system kinds.
-- name: GuestEmailData :one
SELECT b.id, b.account_id, b.status, b.source, b.check_in, b.check_out, b.payment_due_at, b.currency,
       b.total, b.payment_method, b.guest_token_sealed,
       coalesce((SELECT sum(CASE WHEN pm.kind = 'payment' THEN pm.amount ELSE -pm.amount END)
                 FROM payments pm WHERE pm.booking_id = b.id), 0)::bigint AS paid,
       g.name AS guest_name, g.email AS guest_email,
       p.name AS property_name, p.slug AS property_slug, p.location AS property_location,
       p.lat AS property_lat, p.lng AS property_lng, p.check_in_time AS property_check_in,
       p.time_zone AS property_time_zone,
       a.status AS account_status,
       coalesce((SELECT u.email FROM memberships m JOIN users u ON u.id = m.user_id
                 WHERE m.account_id = b.account_id AND m.role = 'owner' AND m.status = 'active'
                   AND u.email IS NOT NULL AND u.email_confirmed_at IS NOT NULL AND u.deleted_at IS NULL
                 ORDER BY m.created_at, m.id LIMIT 1), '')::text AS owner_email,
       coalesce(t.body, '')::text AS template_body,
       coalesce(t.channels, '{}')::text[] AS template_channels
FROM bookings b
JOIN guests g ON g.id = b.guest_id
JOIN properties p ON p.id = b.property_id
JOIN accounts a ON a.id = b.account_id
LEFT JOIN message_templates t ON t.account_id = b.account_id AND t.key = sqlc.arg('template_key')::text
WHERE b.id = @booking_id AND b.account_id = @account_id;

-- Gives a stay a guest token, unless it already has one that emails can use.
-- name: SetGuestToken :execrows
UPDATE bookings SET guest_token_hash = @token_hash, guest_token_sealed = @token_sealed
WHERE id = @id AND account_id = @account_id AND guest_token_sealed IS NULL;

-- Stays whose timed emails may be due soon. The caller works out the exact time in the
-- property's time zone and passes bounds that only keep the candidates few. Imported stays, stays with
-- no guest email, and closed accounts are left out.
-- name: TimedEmailCandidates :many
SELECT b.id, b.account_id, b.status, b.check_in, b.check_out, b.payment_due_at,
       g.email AS guest_email, p.time_zone
FROM bookings b
JOIN guests g ON g.id = b.guest_id AND length(g.email) > 0
JOIN properties p ON p.id = b.property_id
JOIN accounts a ON a.id = b.account_id AND a.status <> 'closed'
WHERE b.source NOT IN ('airbnb', 'booking', 'agoda', 'expedia')
  AND (
      (b.status = 'awaiting_payment' AND b.payment_due_at >= @due_after::timestamptz AND b.payment_due_at <= @due_before::timestamptz)
   OR (b.status IN ('awaiting_payment', 'confirmed') AND b.check_in >= @check_in_from::date AND b.check_in <= @check_in_to::date)
   OR (b.status IN ('confirmed', 'checked_in', 'checked_out') AND b.check_out >= @check_out_from::date AND b.check_out <= @check_out_to::date)
  )
ORDER BY b.id;

-- name: CreateEmailConfirmation :exec
INSERT INTO email_confirmations (id, user_id, email, token_hash, expires_at, created_at)
VALUES (@id, @user_id, @email, @token_hash, @expires_at, @now::timestamptz);

-- name: UseEmailConfirmation :one
UPDATE email_confirmations SET used_at = @now::timestamptz
WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > @now::timestamptz
RETURNING user_id, email;

-- Confirms the address only while it is still the person's email.
-- name: ConfirmUserEmail :execrows
UPDATE users SET email_confirmed_at = @now::timestamptz
WHERE id = @id AND email = @email AND deleted_at IS NULL;

-- Whether this is the person's first email address, so it gets the welcome email.
-- name: HadEmailConfirmation :one
SELECT EXISTS (SELECT 1 FROM email_confirmations WHERE user_id = @user_id);

-- name: CountSessions :one
SELECT count(*) FROM sessions WHERE user_id = @user_id;

-- name: HasSignedInOnDevice :one
SELECT EXISTS (SELECT 1 FROM sessions WHERE user_id = @user_id AND device_name = @device_name);

-- The address account emails go to: the person's email once it is confirmed.
-- name: ConfirmedEmail :one
SELECT coalesce(email, '')::text AS email, name FROM users
WHERE id = @id AND email IS NOT NULL AND email_confirmed_at IS NOT NULL;

-- The first owner's confirmed address and name, for receipts.
-- name: OwnerConfirmedEmail :one
SELECT u.id, coalesce(u.email, '')::text AS email, u.name FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.account_id = @account_id AND m.role = 'owner' AND m.status = 'active'
  AND u.email IS NOT NULL AND u.email_confirmed_at IS NOT NULL AND u.deleted_at IS NULL
ORDER BY m.created_at, m.id LIMIT 1;

-- name: SkipAccountEmails :exec
UPDATE emails SET status = 'skipped' WHERE account_id = @account_id AND status = 'pending';

-- name: CountConfirmationsSince :one
SELECT count(*) FROM email_confirmations WHERE user_id = @user_id AND created_at > @since::timestamptz;
