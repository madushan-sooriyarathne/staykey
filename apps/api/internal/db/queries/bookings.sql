-- Guests, bookings and what hangs off them. Child rows load for a page of bookings at once.

-- Guests -----------------------------------------------------------------------------------

-- name: FindGuest :one
SELECT id FROM guests
WHERE account_id = @account_id
  AND ((sqlc.narg('phone')::text IS NOT NULL AND phone = sqlc.narg('phone')::text)
    OR (sqlc.narg('email')::text IS NOT NULL AND lower(email) = lower(sqlc.narg('email')::text)))
ORDER BY created_at
LIMIT 1;

-- name: InsertGuest :exec
INSERT INTO guests (id, account_id, name, phone, email, country)
VALUES (@id, @account_id, @name, sqlc.narg('phone'), sqlc.narg('email'), sqlc.narg('country'));

-- name: UpdateGuest :exec
UPDATE guests SET name = @name, phone = sqlc.narg('phone'), email = sqlc.narg('email'),
                  country = sqlc.narg('country')
WHERE id = @id AND account_id = @account_id;

-- Bookings ---------------------------------------------------------------------------------

-- name: NextBookingNumber :one
UPDATE accounts SET booking_seq = booking_seq + 1 WHERE id = @account_id RETURNING booking_seq;

-- name: InsertBooking :exec
INSERT INTO bookings (id, account_id, property_id, unit_id, ref, source, status, guest_id, adults,
                      children, check_in, check_out, currency, total, guest_note, owner_note,
                      request_expires_at, payment_due_at, promo_id, created_by)
VALUES (@id, @account_id, @property_id, @unit_id, @ref, @source, @status, @guest_id, @adults,
        @children, @check_in, @check_out, @currency, @total, @guest_note, @owner_note,
        sqlc.narg('request_expires_at'), sqlc.narg('payment_due_at'), sqlc.narg('promo_id'),
        sqlc.narg('created_by'));

-- name: UpdateBooking :exec
UPDATE bookings SET unit_id = @unit_id, source = @source, status = @status, adults = @adults,
                    children = @children, check_in = @check_in, check_out = @check_out,
                    total = @total, guest_note = @guest_note, owner_note = @owner_note,
                    request_expires_at = sqlc.narg('request_expires_at'),
                    payment_due_at = sqlc.narg('payment_due_at'),
                    cancel_reason = sqlc.narg('cancel_reason'),
                    cancelled_at = sqlc.narg('cancelled_at'),
                    version = version + 1
WHERE id = @id AND account_id = @account_id;

-- name: GetBooking :one
SELECT sqlc.embed(b), g.name AS guest_name, g.phone AS guest_phone, g.email AS guest_email,
       g.country AS guest_country
FROM bookings b
JOIN guests g ON g.id = b.guest_id
WHERE b.id = @id AND b.account_id = @account_id;

-- name: GetBookingForUpdate :one
SELECT sqlc.embed(b), g.name AS guest_name, g.phone AS guest_phone, g.email AS guest_email,
       g.country AS guest_country
FROM bookings b
JOIN guests g ON g.id = b.guest_id
WHERE b.id = @id AND b.account_id = @account_id
FOR UPDATE OF b;

-- Stays that overlap [from, to), ordered by arrival. The cursor is the last row's arrival and id.
-- name: ListBookings :many
SELECT sqlc.embed(b), g.name AS guest_name, g.phone AS guest_phone, g.email AS guest_email,
       g.country AS guest_country
FROM bookings b
JOIN guests g ON g.id = b.guest_id
WHERE b.account_id = @account_id
  AND (sqlc.narg('property_ids')::uuid[] IS NULL OR b.property_id = ANY (sqlc.narg('property_ids')::uuid[]))
  AND (sqlc.narg('range_from')::date IS NULL OR b.check_out > sqlc.narg('range_from')::date)
  AND (sqlc.narg('range_to')::date IS NULL OR b.check_in < sqlc.narg('range_to')::date)
  AND (sqlc.narg('after_check_in')::date IS NULL
    OR (b.check_in, b.id) > (sqlc.narg('after_check_in')::date, sqlc.narg('after_id')::uuid))
ORDER BY b.check_in, b.id
LIMIT @row_limit;

-- name: ListBookingLines :many
SELECT booking_id, label, amount, kind
FROM booking_lines
WHERE account_id = @account_id AND booking_id = ANY (@booking_ids::uuid[])
ORDER BY booking_id, position;

-- name: DeleteBookingLines :exec
DELETE FROM booking_lines WHERE booking_id = @booking_id AND account_id = @account_id;

-- name: InsertBookingLine :exec
INSERT INTO booking_lines (account_id, booking_id, position, label, amount, kind)
VALUES (@account_id, @booking_id, @position, @label, @amount, sqlc.narg('kind'));

-- name: ListBookingExtras :many
SELECT booking_id, extra_id, name, amount
FROM booking_extras
WHERE account_id = @account_id AND booking_id = ANY (@booking_ids::uuid[])
ORDER BY booking_id, name;

-- name: DeleteBookingExtras :exec
DELETE FROM booking_extras WHERE booking_id = @booking_id AND account_id = @account_id;

-- name: InsertBookingExtra :exec
INSERT INTO booking_extras (account_id, booking_id, extra_id, name, amount)
VALUES (@account_id, @booking_id, @extra_id, @name, @amount);

-- name: InsertBookingEvent :exec
INSERT INTO booking_events (id, account_id, booking_id, from_status, to_status, actor_user_id, reason)
VALUES (@id, @account_id, @booking_id, sqlc.narg('from_status'), @to_status,
        sqlc.narg('actor_user_id'), @reason);

-- name: ListBookingEvents :many
SELECT from_status, to_status, actor_user_id, reason, created_at
FROM booking_events
WHERE booking_id = @booking_id AND account_id = @account_id
ORDER BY created_at, id;

-- Payments and slips -----------------------------------------------------------------------

-- name: ListPayments :many
SELECT id, booking_id, kind, method, amount, note, received_at, slip_id
FROM payments
WHERE account_id = @account_id AND booking_id = ANY (@booking_ids::uuid[])
ORDER BY booking_id, received_at, id;

-- name: InsertPayment :exec
INSERT INTO payments (id, account_id, booking_id, kind, method, amount, currency, note,
                      received_at, recorded_by, slip_id)
VALUES (@id, @account_id, @booking_id, @kind, @method, @amount, @currency, @note, @received_at,
        sqlc.narg('recorded_by'), sqlc.narg('slip_id'));

-- name: ListSlips :many
SELECT id, booking_id, file_key, amount, status, uploaded_at
FROM payment_slips
WHERE account_id = @account_id AND booking_id = ANY (@booking_ids::uuid[])
ORDER BY booking_id, uploaded_at, id;

-- name: GetSlipForUpdate :one
SELECT id, booking_id, amount, status
FROM payment_slips
WHERE id = @id AND account_id = @account_id
FOR UPDATE;

-- name: InsertSlip :exec
INSERT INTO payment_slips (id, account_id, booking_id, file_key, amount, uploaded_at)
VALUES (@id, @account_id, @booking_id, @file_key, @amount, @uploaded_at);

-- name: ReviewSlip :exec
UPDATE payment_slips SET status = @status, reviewed_by = sqlc.narg('reviewed_by'),
                         reviewed_at = @reviewed_at
WHERE id = @id AND account_id = @account_id;
