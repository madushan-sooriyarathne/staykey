-- Sweeps the job workers run across every account.

-- name: DeclineExpiredRequests :many
UPDATE bookings SET status = 'declined', request_expires_at = NULL, version = version + 1
WHERE status = 'requested' AND request_expires_at < @now::timestamptz
RETURNING id, account_id;

-- A stay waiting for payment is cancelled once its deadline passes with nothing paid. The
-- deadline is only set when the property cancels unpaid stays.
-- name: CancelUnpaidBookings :many
UPDATE bookings b SET status = 'cancelled', payment_due_at = NULL, cancelled_at = @now::timestamptz,
                      cancel_reason = 'Not paid in time', version = version + 1
WHERE b.status = 'awaiting_payment' AND b.payment_due_at < @now::timestamptz
  AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.booking_id = b.id AND p.kind = 'payment')
RETURNING b.id, b.account_id;

-- name: FileInUse :one
SELECT EXISTS (SELECT 1 FROM property_photos WHERE key = @key::text)
    OR EXISTS (SELECT 1 FROM properties WHERE logo_key = @key::text)
    OR EXISTS (SELECT 1 FROM payment_slips WHERE file_key = @key::text) AS in_use;
