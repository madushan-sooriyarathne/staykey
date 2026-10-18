-- Message templates, notification preferences, push tokens and the morning summary.

-- name: InsertTemplate :exec
INSERT INTO message_templates (id, account_id, key, name, channels, timing, body)
VALUES (@id, @account_id, sqlc.arg('key')::text, @name, @channels, @timing, @body)
ON CONFLICT (account_id, key) DO NOTHING;

-- name: ListTemplates :many
SELECT * FROM message_templates WHERE account_id = @account_id
ORDER BY array_position(ARRAY['confirmed', 'reminder', 'prearrival', 'thanks'], key);

-- name: UpdateTemplateBody :one
UPDATE message_templates SET body = @body WHERE id = @id AND account_id = @account_id
RETURNING *;

-- name: ListNotificationPrefs :many
SELECT event, push, whatsapp FROM notification_prefs
WHERE account_id = @account_id AND user_id = @user_id
ORDER BY array_position(ARRAY['booking', 'request', 'payment', 'cancellation', 'sync', 'summary'], event);

-- name: InsertNotificationPref :exec
INSERT INTO notification_prefs (id, account_id, user_id, event, push, whatsapp)
VALUES (@id, @account_id, @user_id, @event, @push, @whatsapp)
ON CONFLICT (user_id, account_id, event) DO NOTHING;

-- name: SetNotificationPref :exec
INSERT INTO notification_prefs (id, account_id, user_id, event, push, whatsapp)
VALUES (@id, @account_id, @user_id, @event, @push, @whatsapp)
ON CONFLICT (user_id, account_id, event) DO UPDATE SET push = excluded.push, whatsapp = excluded.whatsapp;

-- A token moves to whoever signs in on that device last.
-- name: UpsertPushToken :exec
INSERT INTO push_tokens (id, user_id, session_id, token, platform)
VALUES (@id, @user_id, @session_id, @token, @platform)
ON CONFLICT (token) DO UPDATE SET user_id = excluded.user_id, session_id = excluded.session_id,
                                  platform = excluded.platform;

-- Members whose summary is due: past 07:00 and before 11:00 in the account's time zone (its
-- first property's), and not yet sent today there.
-- name: DueSummaries :many
SELECT p.account_id, a.name AS account_name, p.user_id, m.id AS membership_id, m.role, u.phone, p.push, p.whatsapp,
       l.local_now::date AS local_day,
       coalesce((SELECT array_agg(mp.property_id ORDER BY mp.property_id) FROM membership_properties mp
                 WHERE mp.membership_id = m.id), '{}')::uuid[] AS property_ids
FROM notification_prefs p
JOIN memberships m ON m.account_id = p.account_id AND m.user_id = p.user_id AND m.status = 'active'
JOIN accounts a ON a.id = p.account_id AND a.status <> 'closed'
JOIN users u ON u.id = p.user_id AND u.deleted_at IS NULL
CROSS JOIN LATERAL (
    SELECT timezone(coalesce((SELECT pr.time_zone FROM properties pr WHERE pr.account_id = p.account_id
                              ORDER BY pr.created_at, pr.id LIMIT 1), 'Asia/Colombo'),
                    sqlc.arg('now')::timestamptz) AS local_now
) l
WHERE p.event = 'summary' AND (p.push OR p.whatsapp)
  AND l.local_now::time >= '07:00' AND l.local_now::time < '11:00'
  AND (p.last_sent_on IS NULL OR p.last_sent_on < l.local_now::date)
ORDER BY p.account_id, p.user_id;

-- name: SummaryCounts :one
SELECT
    count(*) FILTER (WHERE check_in = @day::date AND status IN ('awaiting_payment', 'confirmed')) AS arrivals,
    count(*) FILTER (WHERE check_out = @day::date AND status IN ('confirmed', 'checked_in')) AS departures,
    count(*) FILTER (WHERE status = 'requested') AS requests
FROM bookings
WHERE account_id = @account_id
  AND (sqlc.narg('property_ids')::uuid[] IS NULL OR property_id = ANY (sqlc.narg('property_ids')::uuid[]));

-- name: MarkSummarySent :exec
UPDATE notification_prefs SET last_sent_on = @day::date
WHERE account_id = @account_id AND user_id = @user_id AND event = 'summary';
