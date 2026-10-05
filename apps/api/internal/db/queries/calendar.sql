-- The night ledger, blocks and rate overrides.

-- Night ledger -----------------------------------------------------------------------------

-- Holds every night in [check_in, check_out) on each unit for a booking or a block. A night
-- someone else holds fails the primary key.
-- name: InsertUnitNights :exec
INSERT INTO unit_nights (unit_id, night, account_id, property_id, booking_id, block_id)
SELECT u.unit_id, n.night::date, @account_id::uuid, @property_id::uuid,
       sqlc.narg('booking_id')::uuid, sqlc.narg('block_id')::uuid
FROM unnest(sqlc.arg('unit_ids')::uuid[]) AS u (unit_id)
CROSS JOIN generate_series(sqlc.arg('check_in')::date, sqlc.arg('check_out')::date - 1, interval '1 day') AS n (night);

-- name: DeleteBookingNights :exec
DELETE FROM unit_nights WHERE booking_id = @booking_id AND account_id = @account_id;

-- The first night already held on these units in [check_in, check_out), with what holds it.
-- name: FirstHeldNight :one
SELECT un.night, un.unit_id, un.booking_id, un.block_id,
       b.ref AS booking_ref, b.check_in AS booking_check_in, b.check_out AS booking_check_out,
       g.name AS guest_name, bl.starts_on AS block_starts_on, bl.ends_on AS block_ends_on
FROM unit_nights un
LEFT JOIN bookings b ON b.id = un.booking_id
LEFT JOIN guests g ON g.id = b.guest_id
LEFT JOIN blocks bl ON bl.id = un.block_id
WHERE un.account_id = @account_id
  AND un.unit_id = ANY (@unit_ids::uuid[])
  AND un.night >= @check_in::date AND un.night < @check_out::date
ORDER BY un.night
LIMIT 1;

-- name: DeleteExpiredHolds :execrows
DELETE FROM unit_nights WHERE hold_expires_at < @now::timestamptz;

-- Blocks -----------------------------------------------------------------------------------

-- name: InsertBlock :exec
INSERT INTO blocks (id, account_id, property_id, starts_on, ends_on, reason, note, created_by)
VALUES (@id, @account_id, @property_id, @starts_on, @ends_on, @reason, @note, sqlc.narg('created_by'));

-- name: InsertBlockUnit :exec
INSERT INTO block_units (account_id, property_id, block_id, unit_id)
VALUES (@account_id, @property_id, @block_id, @unit_id);

-- name: GetBlock :one
SELECT id, property_id, starts_on, ends_on, reason, note
FROM blocks
WHERE id = @id AND account_id = @account_id;

-- name: DeleteBlock :exec
DELETE FROM blocks WHERE id = @id AND account_id = @account_id;

-- name: ListBlocks :many
SELECT id, property_id, starts_on, ends_on, reason, note
FROM blocks
WHERE account_id = @account_id AND property_id = @property_id
  AND ends_on > @range_from::date AND starts_on < @range_to::date
ORDER BY starts_on, id;

-- name: ListBlockUnits :many
SELECT block_id, unit_id
FROM block_units
WHERE account_id = @account_id AND block_id = ANY (@block_ids::uuid[]);

-- Rate overrides ---------------------------------------------------------------------------

-- name: DeleteRateOverrides :exec
DELETE FROM rate_overrides
WHERE account_id = @account_id AND property_id = @property_id
  AND unit_id = ANY (@unit_ids::uuid[]) AND night >= @range_from::date AND night < @range_to::date;

-- name: InsertRateOverrides :exec
INSERT INTO rate_overrides (unit_id, night, account_id, property_id, price, min_nights, closed_to_arrival)
SELECT u.unit_id, n.night::date, @account_id::uuid, @property_id::uuid, sqlc.narg('price')::bigint,
       sqlc.narg('min_nights')::smallint, @closed_to_arrival::boolean
FROM unnest(sqlc.arg('unit_ids')::uuid[]) AS u (unit_id)
CROSS JOIN generate_series(sqlc.arg('range_from')::date, sqlc.arg('range_to')::date - 1, interval '1 day') AS n (night);

-- name: ListRateOverrides :many
SELECT unit_id, night, price, min_nights, closed_to_arrival
FROM rate_overrides
WHERE account_id = @account_id AND property_id = @property_id
  AND night >= @range_from::date AND night < @range_to::date
ORDER BY unit_id, night;
