-- Owner queries take the account, and the membership's property scope when it has one, so a
-- missed filter cannot leak another account's rows. A property's base rate is its lowest active
-- unit rate.

-- name: ListProperties :many
SELECT p.*,
       coalesce((SELECT min(u.rate) FROM units u
                 WHERE u.property_id = p.id AND u.archived_at IS NULL), 0)::bigint AS base_rate
FROM properties p
WHERE p.account_id = @account_id
  AND (sqlc.narg('property_ids')::uuid[] IS NULL OR p.id = ANY (sqlc.narg('property_ids')::uuid[]))
ORDER BY p.created_at DESC
LIMIT 200;

-- name: GetProperty :one
SELECT p.*,
       coalesce((SELECT min(u.rate) FROM units u
                 WHERE u.property_id = p.id AND u.archived_at IS NULL), 0)::bigint AS base_rate
FROM properties p
WHERE p.id = @id AND p.account_id = @account_id;

-- name: LockProperty :one
-- Serializes changes to one property's setup.
SELECT id FROM properties WHERE id = @id AND account_id = @account_id FOR UPDATE;

-- name: InsertProperty :exec
INSERT INTO properties (id, account_id, slug, name, booking_type, location, currency,
                        ical_export_token, display_currencies)
VALUES (@id, @account_id, @slug, @name, @booking_type, nullif(@location::text, ''), @currency,
        @ical_export_token, @display_currencies);

-- name: UpdatePropertyDetails :exec
-- Null leaves a column as it is.
UPDATE properties SET
    name             = coalesce(sqlc.narg('name')::text, name),
    booking_type     = coalesce(sqlc.narg('booking_type')::text, booking_type),
    description      = coalesce(sqlc.narg('description')::text, description),
    location         = CASE WHEN sqlc.narg('location')::text IS NULL THEN location
                            ELSE nullif(sqlc.narg('location')::text, '') END,
    check_in_time    = coalesce(sqlc.narg('check_in_time')::text, check_in_time),
    check_out_time   = coalesce(sqlc.narg('check_out_time')::text, check_out_time),
    amenities        = coalesce(sqlc.narg('amenities')::text[], amenities),
    policy           = coalesce(sqlc.narg('policy')::text, policy),
    deposit_percent  = coalesce(sqlc.narg('deposit_percent')::smallint, deposit_percent),
    balance_due_days = coalesce(sqlc.narg('balance_due_days')::smallint, balance_due_days),
    house_rules      = coalesce(sqlc.narg('house_rules')::text[], house_rules)
WHERE id = @id AND account_id = @account_id;

-- name: UpdatePropertyRules :exec
UPDATE properties SET
    min_nights      = @min_nights,
    max_nights      = @max_nights,
    same_day_cutoff = sqlc.narg('same_day_cutoff'),
    window_months   = @window_months,
    closed_arrival  = @closed_arrival
WHERE id = @id AND account_id = @account_id;

-- name: UpdatePropertyBooking :exec
UPDATE properties SET
    booking_mode       = @booking_mode,
    reply_hours        = @reply_hours,
    hold_minutes       = @hold_minutes,
    display_currencies = @display_currencies
WHERE id = @id AND account_id = @account_id;

-- name: UpdatePropertyBranding :exec
UPDATE properties SET brand_color = @brand_color, logo_key = sqlc.narg('logo_key')
WHERE id = @id AND account_id = @account_id;

-- name: UpdatePropertyExtraGuest :exec
UPDATE properties SET extra_guest_above = @extra_guest_above, extra_guest_amount = @extra_guest_amount
WHERE id = @id AND account_id = @account_id;

-- Public: booking pages look a property up by its address, across accounts.

-- name: GetPropertyBySlug :one
SELECT p.*,
       coalesce((SELECT min(u.rate) FROM units u
                 WHERE u.property_id = p.id AND u.archived_at IS NULL), 0)::bigint AS base_rate
FROM properties p
WHERE p.slug = @slug;
