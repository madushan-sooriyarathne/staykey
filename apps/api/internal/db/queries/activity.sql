-- Activity: what happened in an account, newest first. Rows also act as the alert outbox.

-- name: InsertActivity :exec
INSERT INTO activity (id, account_id, property_id, booking_id, kind, title, subtitle, actor_user_id, created_at)
VALUES (@id, @account_id, sqlc.narg('property_id'), sqlc.narg('booking_id'), @kind, @title, @subtitle,
        sqlc.narg('actor_user_id'), @created_at);

-- Rows without a property (account-wide) are visible to everyone in the account.
-- name: ListActivity :many
SELECT * FROM activity a
WHERE a.account_id = @account_id
  AND (sqlc.narg('property_ids')::uuid[] IS NULL OR a.property_id IS NULL
       OR a.property_id = ANY (sqlc.narg('property_ids')::uuid[]))
  AND NOT (a.kind = ANY (@hidden_kinds::text[]))
  AND (sqlc.narg('before_at')::timestamptz IS NULL
       OR (a.created_at, a.id) < (sqlc.narg('before_at')::timestamptz, sqlc.narg('before_id')::uuid))
ORDER BY a.created_at DESC, a.id DESC
LIMIT @row_limit;

-- What the person did themselves never counts as unread.
-- name: CountUnreadActivity :one
SELECT count(*) FROM activity a
WHERE a.account_id = @account_id
  AND (sqlc.narg('property_ids')::uuid[] IS NULL OR a.property_id IS NULL
       OR a.property_id = ANY (sqlc.narg('property_ids')::uuid[]))
  AND NOT (a.kind = ANY (@hidden_kinds::text[]))
  AND a.actor_user_id IS DISTINCT FROM @user_id::uuid
  AND a.created_at > coalesce(
      (SELECT r.last_read_at FROM activity_reads r WHERE r.account_id = @account_id AND r.user_id = @user_id::uuid),
      '-infinity'::timestamptz);

-- name: GetLastRead :one
SELECT last_read_at FROM activity_reads WHERE account_id = @account_id AND user_id = @user_id;

-- name: SetLastRead :exec
INSERT INTO activity_reads (account_id, user_id, last_read_at) VALUES (@account_id, @user_id, @last_read_at)
ON CONFLICT (account_id, user_id) DO UPDATE SET last_read_at = greatest(activity_reads.last_read_at, excluded.last_read_at);

-- name: ClaimUnnotifiedActivity :many
SELECT * FROM activity WHERE notified_at IS NULL
ORDER BY created_at, id
LIMIT @row_limit
FOR UPDATE SKIP LOCKED;

-- name: MarkActivityNotified :exec
UPDATE activity SET notified_at = @now::timestamptz WHERE id = ANY (@ids::uuid[]);

-- Active members who want this event on some channel and can see the property, except the
-- person who acted.
-- name: ActivityRecipients :many
SELECT m.user_id, m.role, u.phone, p.push, p.whatsapp
FROM memberships m
JOIN users u ON u.id = m.user_id
JOIN notification_prefs p ON p.account_id = m.account_id AND p.user_id = m.user_id AND p.event = @event
WHERE m.account_id = @account_id AND m.status = 'active'
  AND (p.push OR p.whatsapp)
  AND m.user_id IS DISTINCT FROM sqlc.narg('actor_user_id')::uuid
  AND (sqlc.narg('property_id')::uuid IS NULL
       OR NOT EXISTS (SELECT 1 FROM membership_properties mp WHERE mp.membership_id = m.id)
       OR EXISTS (SELECT 1 FROM membership_properties mp
                  WHERE mp.membership_id = m.id AND mp.property_id = sqlc.narg('property_id')::uuid))
ORDER BY m.created_at, m.id;

-- Push tokens on sessions that are still signed in.
-- name: LivePushTokens :many
SELECT pt.user_id, pt.token FROM push_tokens pt
JOIN sessions s ON s.id = pt.session_id
WHERE pt.user_id = ANY (@user_ids::uuid[]) AND s.revoked_at IS NULL AND s.expires_at > @now::timestamptz
ORDER BY pt.created_at, pt.id;

-- name: DeletePushTokens :exec
DELETE FROM push_tokens WHERE token = ANY (@tokens::text[]);
