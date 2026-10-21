-- Guest card payments through Payments.lk.

-- Card mode and connections -----------------------------------------------------------------

-- name: GetCardState :one
SELECT a.card_mode, c.key_last4, c.mode, c.status, c.checked_at
FROM accounts a
LEFT JOIN card_connections c ON c.account_id = a.id
WHERE a.id = @account_id;

-- name: SetCardMode :exec
UPDATE accounts SET card_mode = @card_mode WHERE id = @account_id;

-- name: GetCardConnection :one
SELECT account_id, secret_key_sealed, key_last4, mode, status, checked_at, events_since
FROM card_connections
WHERE account_id = @account_id;

-- name: UpsertCardConnection :exec
INSERT INTO card_connections (account_id, secret_key_sealed, key_last4, mode, status, checked_at,
                              events_since)
VALUES (@account_id, @secret_key_sealed, @key_last4, @mode, 'connected', @checked_at, @events_since)
ON CONFLICT (account_id) DO UPDATE SET
    secret_key_sealed = excluded.secret_key_sealed,
    key_last4         = excluded.key_last4,
    mode              = excluded.mode,
    status            = 'connected',
    checked_at        = excluded.checked_at,
    events_since      = excluded.events_since;

-- name: DeleteCardConnection :exec
DELETE FROM card_connections WHERE account_id = @account_id;

-- name: MarkCardConnectionFailed :execrows
UPDATE card_connections SET status = 'failed', checked_at = @checked_at
WHERE account_id = @account_id AND status = 'connected';

-- name: AdvanceCardEvents :exec
UPDATE card_connections SET events_since = @events_since
WHERE account_id = @account_id AND events_since < @events_since;

-- name: GetCardCursor :one
SELECT events_since FROM card_cursors WHERE source = @source;

-- name: SetCardCursor :exec
INSERT INTO card_cursors (source, events_since) VALUES (@source, @events_since)
ON CONFLICT (source) DO UPDATE SET events_since = greatest(card_cursors.events_since, excluded.events_since);

-- Connections the events job reads: working, on accounts that use their own account and are
-- still open.
-- name: ListLiveCardConnections :many
SELECT c.account_id, c.secret_key_sealed, c.mode, c.events_since
FROM card_connections c
JOIN accounts a ON a.id = c.account_id
WHERE c.status = 'connected' AND a.card_mode = 'own' AND a.status <> 'closed'
ORDER BY c.account_id;

-- Checkouts ---------------------------------------------------------------------------------

-- name: InsertCardCheckout :exec
INSERT INTO card_checkouts (id, account_id, booking_id, collected_by, purpose, amount, currency,
                            charged_cents, rate, mode, expires_at, created_at)
VALUES (@id, @account_id, @booking_id, @collected_by, @purpose, @amount, @currency, @charged_cents,
        sqlc.narg('rate'), @mode, @expires_at, @created_at);

-- name: SetCardCheckoutProvider :exec
UPDATE card_checkouts SET provider_checkout_id = @provider_checkout_id
WHERE id = @id AND provider_checkout_id IS NULL;

-- An open checkout for the same part of the stay and the same amount can be sent to the guest
-- again, as long as it has a while to run.
-- name: ReusableCardCheckout :one
SELECT id, provider_checkout_id, collected_by
FROM card_checkouts
WHERE booking_id = @booking_id AND account_id = @account_id AND purpose = @purpose
  AND charged_cents = @charged_cents AND collected_by = @collected_by AND mode = @mode
  AND status = 'open' AND provider_checkout_id IS NOT NULL AND expires_at > @fresh_until
ORDER BY created_at DESC
LIMIT 1;

-- name: OpenCardCheckouts :many
SELECT id, provider_checkout_id, collected_by
FROM card_checkouts
WHERE booking_id = @booking_id AND status = 'open' AND provider_checkout_id IS NOT NULL
ORDER BY created_at DESC
LIMIT 5;

-- name: GetCardCheckoutForUpdate :one
SELECT *
FROM card_checkouts
WHERE id = @id
FOR UPDATE;

-- name: GetCardCheckoutByProviderForUpdate :one
SELECT *
FROM card_checkouts
WHERE provider_checkout_id = @provider_checkout_id
FOR UPDATE;

-- name: MarkCardCheckoutPaid :exec
UPDATE card_checkouts SET status = 'paid', provider_payment_id = @provider_payment_id, paid_at = @paid_at
WHERE id = @id;

-- name: MarkCardCheckoutExpired :exec
UPDATE card_checkouts SET status = 'expired' WHERE id = @id AND status = 'open';

-- Events ------------------------------------------------------------------------------------

-- name: InsertCardEvent :execrows
INSERT INTO card_events (source, event_id, account_id, type, received_at)
VALUES (@source, @event_id, sqlc.narg('account_id'), @type, @received_at)
ON CONFLICT (source, event_id) DO NOTHING;

-- Card payments and refunds -----------------------------------------------------------------

-- name: InsertCardPayment :exec
INSERT INTO payments (id, account_id, booking_id, kind, method, amount, currency, note, received_at,
                      recorded_by, provider_ref, status, charged_cents, charged_currency, rate,
                      card_checkout_id, collected_by, refund_of)
VALUES (@id, @account_id, @booking_id, @kind, 'card', @amount, @currency, @note, @received_at,
        sqlc.narg('recorded_by'), sqlc.narg('provider_ref'), @status, @charged_cents, 'LKR',
        sqlc.narg('rate'), sqlc.narg('card_checkout_id'), @collected_by, sqlc.narg('refund_of'));

-- name: CardPaymentExists :one
SELECT EXISTS (
    SELECT 1 FROM payments
    WHERE account_id = @account_id AND provider_ref = @provider_ref AND kind = 'payment'
) AS found;

-- Card payments of a stay that can still be refunded, newest first, with what is left of each
-- after refunds that are pending or went through.
-- name: RefundableCardPayments :many
SELECT p.id, p.amount, p.charged_cents, p.rate, p.collected_by,
       (p.amount - coalesce(sum(r.amount) FILTER (WHERE r.status <> 'failed'), 0))::bigint AS amount_left,
       (coalesce(p.charged_cents, 0) - coalesce(sum(r.charged_cents) FILTER (WHERE r.status <> 'failed'), 0))::bigint AS charged_left
FROM payments p
LEFT JOIN payments r ON r.refund_of = p.id
WHERE p.booking_id = @booking_id AND p.account_id = @account_id AND p.kind = 'payment'
  AND p.method = 'card' AND p.status = 'succeeded' AND p.provider_ref IS NOT NULL
GROUP BY p.id
ORDER BY p.received_at DESC, p.id DESC;

-- Refunds not yet accepted by Payments.lk, claimed for one send. A claim older than
-- stale_before can be taken again; the provider's idempotency key makes a repeat harmless.
-- name: ClaimCardRefunds :many
SELECT r.id, r.account_id, r.booking_id, r.charged_cents, r.collected_by, o.provider_ref AS payment_ref
FROM payments r
JOIN payments o ON o.id = r.refund_of
WHERE r.kind = 'refund' AND r.status = 'pending' AND r.provider_ref IS NULL
  AND (r.attempted_at IS NULL OR r.attempted_at < @stale_before::timestamptz)
  AND (sqlc.narg('booking_id')::uuid IS NULL OR r.booking_id = sqlc.narg('booking_id')::uuid)
ORDER BY r.created_at
LIMIT @row_limit
FOR UPDATE OF r SKIP LOCKED;

-- name: StampCardRefunds :exec
UPDATE payments SET attempted_at = @attempted_at WHERE id = ANY (@ids::uuid[]);

-- name: AcceptCardRefund :exec
UPDATE payments SET provider_ref = @provider_ref WHERE id = @id AND provider_ref IS NULL;

-- name: FailCardRefund :one
UPDATE payments SET status = 'failed', failure = @failure::text
WHERE id = @id AND status = 'pending'
RETURNING account_id, booking_id;

-- name: SettleCardRefund :one
UPDATE payments SET status = @status, failure = sqlc.narg('failure')
WHERE provider_ref = @provider_ref AND kind = 'refund' AND status = 'pending'
  AND (sqlc.narg('account_id')::uuid IS NULL OR account_id = sqlc.narg('account_id')::uuid)
RETURNING id, account_id, booking_id, status;


-- The pending refund of a payment for an amount, when its event arrives before the refund's id
-- was saved.
-- name: PendingRefundOfPayment :one
SELECT r.id
FROM payments r
JOIN payments o ON o.id = r.refund_of
WHERE o.provider_ref = @payment_ref AND r.kind = 'refund' AND r.status = 'pending'
  AND r.provider_ref IS NULL AND r.charged_cents = @charged_cents
  AND (sqlc.narg('account_id')::uuid IS NULL OR r.account_id = sqlc.narg('account_id')::uuid)
ORDER BY r.created_at
LIMIT 1
FOR UPDATE OF r;

-- name: PaymentAmount :one
SELECT amount FROM payments WHERE id = @id;

-- name: SetCollectedFee :exec
UPDATE payments SET provider_fee_cents = @provider_fee_cents
WHERE provider_ref = @provider_ref AND kind = 'payment' AND collected_by = 'staykey';
