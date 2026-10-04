-- Owner queries take the account, and the membership's property scope when it has one, so a
-- missed filter cannot leak another account's rows.

-- name: ListProperties :many
SELECT id, account_id, slug, name, booking_type, location, currency, base_rate_minor, created_at
FROM properties
WHERE account_id = @account_id
  AND (sqlc.narg('property_ids')::uuid[] IS NULL OR id = ANY (sqlc.narg('property_ids')::uuid[]))
ORDER BY created_at DESC
LIMIT 200;

-- name: CreateProperty :one
INSERT INTO properties (id, account_id, slug, name, booking_type, location, currency, base_rate_minor)
VALUES (@id, @account_id, @slug, @name, @booking_type, nullif(@location::text, ''), @currency, @base_rate_minor)
RETURNING id, account_id, slug, name, booking_type, location, currency, base_rate_minor, created_at;

-- Public: booking pages look a property up by its address, across accounts.

-- name: GetPropertyBySlug :one
SELECT id, account_id, slug, name, booking_type, location, currency, base_rate_minor, created_at
FROM properties
WHERE slug = @slug;
