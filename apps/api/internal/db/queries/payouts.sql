-- StayKey payouts: the ledger of card money StayKey collects for owners, and the payouts that
-- pay it out.
--
-- An entry is payable on a date when its own available_on has come and, if it belongs to a
-- stay, the stay checked in before that date. Every query below uses the same test.

-- Payout details ----------------------------------------------------------------------------

-- name: GetPayoutProfile :one
SELECT account_id, legal_name, nic_sealed, nic_last4, bank_name, branch, account_name,
       account_number_sealed, account_last4, terms_version, accepted_at, bank_changed_at, updated_at
FROM payout_profiles
WHERE account_id = @account_id;

-- name: UpsertPayoutProfile :exec
INSERT INTO payout_profiles (account_id, legal_name, nic_sealed, nic_last4, bank_name, branch,
                             account_name, account_number_sealed, account_last4, terms_version,
                             accepted_at, bank_changed_at)
VALUES (@account_id, @legal_name, @nic_sealed, @nic_last4, @bank_name, @branch, @account_name,
        @account_number_sealed, @account_last4, @terms_version, @accepted_at, @bank_changed_at)
ON CONFLICT (account_id) DO UPDATE SET
    legal_name            = excluded.legal_name,
    nic_sealed            = excluded.nic_sealed,
    nic_last4             = excluded.nic_last4,
    bank_name             = excluded.bank_name,
    branch                = excluded.branch,
    account_name          = excluded.account_name,
    account_number_sealed = excluded.account_number_sealed,
    account_last4         = excluded.account_last4,
    terms_version         = excluded.terms_version,
    accepted_at           = excluded.accepted_at,
    bank_changed_at       = excluded.bank_changed_at;

-- name: HasPayoutProfile :one
SELECT EXISTS (SELECT 1 FROM payout_profiles WHERE account_id = @account_id);

-- Ledger ------------------------------------------------------------------------------------

-- A repeat of the same payment, refund or payout and kind is left out.
-- name: InsertPayoutEntry :execrows
INSERT INTO payout_entries (id, account_id, booking_id, payment_id, kind, amount, available_on,
                            payout_id, note, created_at)
VALUES (@id, @account_id, @booking_id, @payment_id, @kind, @amount, @available_on, @payout_id,
        @note, @created_at)
ON CONFLICT DO NOTHING;

-- Puts back a refund that was taken out of the ledger and then failed. Refunds that never were
-- taken out are left alone.
-- name: ReturnRefundEntry :execrows
INSERT INTO payout_entries (id, account_id, booking_id, payment_id, kind, amount, available_on, note, created_at)
SELECT @id, e.account_id, e.booking_id, e.payment_id, 'refund_returned', -e.amount, @available_on,
       'The refund failed', @created_at
FROM payout_entries e
WHERE e.payment_id = @payment_id AND e.kind = 'refund'
ON CONFLICT DO NOTHING;

-- The account's money not yet in a payout, payable by the given day or later, and what is on its way in
-- prepared payouts.
-- name: PayoutBalance :one
SELECT
    coalesce(sum(e.amount) FILTER (WHERE e.available_on <= @payable_on AND (b.check_in IS NULL OR b.check_in < @payable_on)), 0)::bigint AS available,
    coalesce(sum(e.amount) FILTER (WHERE NOT (e.available_on <= @payable_on AND (b.check_in IS NULL OR b.check_in < @payable_on))), 0)::bigint AS pending,
    (SELECT coalesce(sum(p.amount), 0) FROM payouts p
     WHERE p.account_id = @account_id AND p.status = 'prepared')::bigint AS on_the_way
FROM payout_entries e
LEFT JOIN bookings b ON b.id = e.booking_id AND b.account_id = e.account_id
WHERE e.account_id = @account_id AND e.payout_id IS NULL;

-- Payouts -----------------------------------------------------------------------------------

-- name: InsertPayoutBatch :one
INSERT INTO payout_batches (id, period_end, prepared_at)
VALUES (@id, @period_end, @prepared_at)
ON CONFLICT (period_end) DO NOTHING
RETURNING id;

-- Accounts owed at least @minimum by @period_end, with payout details that haven't changed
-- since @held_before, on open accounts.
-- name: PayoutCandidates :many
SELECT e.account_id, sum(e.amount)::bigint AS amount
FROM payout_entries e
JOIN payout_profiles pp ON pp.account_id = e.account_id
JOIN accounts a ON a.id = e.account_id
LEFT JOIN bookings b ON b.id = e.booking_id AND b.account_id = e.account_id
WHERE e.payout_id IS NULL
  AND e.available_on <= @period_end AND (b.check_in IS NULL OR b.check_in < @period_end)
  AND a.status <> 'closed'
  AND pp.bank_changed_at <= @held_before
GROUP BY e.account_id
HAVING sum(e.amount) >= @minimum::bigint
ORDER BY e.account_id;

-- name: InsertPayout :exec
INSERT INTO payouts (id, account_id, batch_id, period_end, amount, prepared_at)
VALUES (@id, @account_id, @batch_id, @period_end, @amount, @prepared_at);

-- Attaches every payable entry of the account to the payout and returns their sum.
-- name: AttachPayoutEntries :one
WITH moved AS (
    UPDATE payout_entries SET payout_id = @payout_id
    WHERE id IN (
        SELECT e.id FROM payout_entries e
        LEFT JOIN bookings b ON b.id = e.booking_id AND b.account_id = e.account_id
        WHERE e.account_id = @account_id AND e.payout_id IS NULL
          AND e.available_on <= @period_end AND (b.check_in IS NULL OR b.check_in < @period_end)
        FOR UPDATE OF e
    )
    RETURNING amount
)
SELECT coalesce(sum(amount), 0)::bigint FROM moved;

-- name: SetPayoutAmount :exec
UPDATE payouts SET amount = @amount WHERE id = @id;

-- name: ListPayoutBatches :many
SELECT pb.id, pb.period_end, pb.prepared_at, pb.paid_at, pb.bank_ref,
       count(p.id) FILTER (WHERE p.status <> 'failed')::bigint AS payouts,
       coalesce(sum(p.amount) FILTER (WHERE p.status <> 'failed'), 0)::bigint AS total
FROM payout_batches pb
LEFT JOIN payouts p ON p.batch_id = pb.id
GROUP BY pb.id
ORDER BY pb.period_end DESC
LIMIT @row_limit;

-- name: GetPayoutBatch :one
SELECT id, period_end, prepared_at, paid_at, bank_ref FROM payout_batches WHERE id = @id;

-- The batch's payouts with where to send them, for the bank's bulk upload.
-- name: BatchPayouts :many
SELECT p.id, p.account_id, p.amount, p.status, p.prepared_at, pp.account_name, pp.bank_name, pp.branch,
       pp.account_number_sealed, pp.bank_changed_at
FROM payouts p
JOIN payout_profiles pp ON pp.account_id = p.account_id
WHERE p.batch_id = @batch_id
ORDER BY p.prepared_at, p.id;

-- name: MarkBatchPaid :execrows
UPDATE payout_batches SET paid_at = @paid_at, bank_ref = @bank_ref
WHERE id = @id AND paid_at IS NULL;

-- name: MarkBatchPayoutsPaid :many
UPDATE payouts SET status = 'paid', paid_at = @paid_at
WHERE batch_id = @batch_id AND status = 'prepared'
RETURNING id, account_id, amount, period_end;

-- A payout the bank sent back, or one that was never sent.
-- name: FailPayout :one
UPDATE payouts SET status = 'failed', reason = @reason, failed_at = @failed_at
WHERE id = @id AND status IN ('prepared', 'paid')
RETURNING id, account_id, amount;

-- name: ListPayouts :many
SELECT id, period_end, amount, status, reason, prepared_at, paid_at, failed_at
FROM payouts
WHERE account_id = @account_id
ORDER BY period_end DESC, prepared_at DESC
LIMIT @row_limit;

-- name: GetPayout :one
SELECT id, period_end, amount, status, reason, prepared_at, paid_at, failed_at
FROM payouts
WHERE id = @id AND account_id = @account_id;

-- The entries a payout paid, oldest first, with their stays.
-- name: PayoutStatement :many
SELECT e.id, e.kind, e.amount, e.available_on, e.note, e.created_at, b.id AS booking_id,
       b.ref AS booking_ref, b.check_in, g.name AS guest_name
FROM payout_entries e
LEFT JOIN bookings b ON b.id = e.booking_id AND b.account_id = e.account_id
LEFT JOIN guests g ON g.id = b.guest_id
WHERE e.payout_id = @payout_id AND e.account_id = @account_id AND e.kind <> 'payout'
ORDER BY coalesce(b.check_in, e.available_on), e.created_at, e.id;
