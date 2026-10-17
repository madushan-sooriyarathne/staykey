-- The iCal import schedule and the export.

-- Leases up to row_limit feeds that are due, so a second run skips them, and returns what the
-- import needs. lease_until is when a feed is due again if this run never reports back.
-- name: ClaimDueFeeds :many
UPDATE ical_feeds f SET next_sync_at = @lease_until::timestamptz
FROM properties p
WHERE f.id IN (
    SELECT id FROM ical_feeds
    WHERE url <> '' AND (next_sync_at IS NULL OR next_sync_at <= @now::timestamptz)
    ORDER BY next_sync_at NULLS FIRST, id
    LIMIT @row_limit
    FOR UPDATE SKIP LOCKED
) AND p.id = f.property_id
RETURNING f.id, f.account_id, f.property_id, f.unit_id, f.channel, f.url, f.etag,
          f.consecutive_failures, f.last_synced_at, p.time_zone, p.name AS property_name;

-- name: FeedSucceeded :exec
UPDATE ical_feeds SET status = 'ok', last_synced_at = @now::timestamptz, etag = sqlc.narg('etag'),
    consecutive_failures = 0, last_error = NULL, upcoming = @upcoming, next_sync_at = @next_sync_at::timestamptz
WHERE id = @id AND account_id = @account_id;

-- The feed hasn't changed since it was last read.
-- name: FeedNotModified :exec
UPDATE ical_feeds SET status = 'ok', last_synced_at = @now::timestamptz, consecutive_failures = 0,
    last_error = NULL, next_sync_at = @next_sync_at::timestamptz
WHERE id = @id AND account_id = @account_id;

-- name: FeedFailed :one
UPDATE ical_feeds SET consecutive_failures = consecutive_failures + 1,
    status = CASE WHEN consecutive_failures + 1 >= 2 THEN 'error' ELSE status END,
    last_error = @last_error, next_sync_at = @next_sync_at::timestamptz
WHERE id = @id AND account_id = @account_id
RETURNING consecutive_failures;

-- For "Sync now": due on the next run.
-- name: MarkFeedDue :one
UPDATE ical_feeds SET next_sync_at = NULL
WHERE id = @id AND account_id = @account_id AND url <> ''
RETURNING property_id;

-- name: GetFeedProperty :one
SELECT property_id FROM ical_feeds WHERE id = @id AND account_id = @account_id;

-- The property behind an export link, for checking its token.
-- name: GetExportProperty :one
SELECT id, account_id, name, ical_export_token FROM properties WHERE slug = @slug;

-- Keeps sample feeds with made-up links from ever being read, for the development seed.
-- name: ParkFeeds :exec
UPDATE ical_feeds SET next_sync_at = @until::timestamptz, status = 'ok' WHERE account_id = @account_id;
