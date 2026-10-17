-- Billing: subscriptions, provider events, web checkouts and the account status they decide.

-- name: GetSubscription :one
SELECT * FROM subscriptions WHERE account_id = @account_id;

-- A newer event for the same source replaces the row. A different source replaces it only when
-- it gives access for longer, so a late event about an old store plan never undoes a web period.
-- name: UpsertSubscription :execrows
INSERT INTO subscriptions (account_id, source, plan, period, status, will_renew, current_period_end,
                           product_id, external_id, last_event_at)
VALUES (@account_id, @source, @plan, @period, @status, @will_renew, @current_period_end,
        @product_id, @external_id, @last_event_at)
ON CONFLICT (account_id) DO UPDATE
SET source = excluded.source, plan = excluded.plan, period = excluded.period, status = excluded.status,
    will_renew = excluded.will_renew, current_period_end = excluded.current_period_end,
    product_id = excluded.product_id, external_id = excluded.external_id, last_event_at = excluded.last_event_at
WHERE (subscriptions.source = excluded.source AND subscriptions.last_event_at < excluded.last_event_at)
   OR (subscriptions.source <> excluded.source AND subscriptions.current_period_end < excluded.current_period_end);

-- name: InsertBillingEvent :execrows
INSERT INTO billing_events (id, account_id, source, event_id, type, payload)
VALUES (@id, sqlc.narg('account_id'), @source, @event_id, @type, @payload)
ON CONFLICT (source, event_id) DO NOTHING;

-- name: InsertWebCheckout :exec
INSERT INTO web_checkouts (id, account_id, plan, period, amount, currency, created_at)
VALUES (@id, @account_id, @plan, @period, @amount, @currency, @created_at);

-- name: SetWebCheckoutProvider :exec
UPDATE web_checkouts SET provider_id = @provider_id WHERE id = @id;

-- name: GetWebCheckoutForUpdate :one
SELECT * FROM web_checkouts WHERE id = @id FOR UPDATE;

-- name: MarkWebCheckoutPaid :exec
UPDATE web_checkouts
SET status = 'paid', provider_order_id = @provider_order_id, period_start = @period_start,
    period_end = @period_end, paid_at = @paid_at
WHERE id = @id;

-- name: MarkWebCheckoutExpired :execrows
UPDATE web_checkouts SET status = 'failed' WHERE provider_id = @provider_id AND status = 'pending';

-- Everything the account's billing state is worked out from. owner_phone is the first owner's
-- number, which the booking page offers on WhatsApp once the account lapses.
-- name: GetAccountBilling :one
SELECT a.id, a.status, a.trial_started_at, a.first_direct_booking_at,
       s.source AS sub_source, s.plan AS sub_plan, s.period AS sub_period, s.status AS sub_status,
       s.will_renew AS sub_will_renew, s.current_period_end AS sub_period_end,
       coalesce((SELECT u.phone FROM memberships m JOIN users u ON u.id = m.user_id
                 WHERE m.account_id = a.id AND m.role = 'owner' AND m.status = 'active'
                 ORDER BY m.created_at, m.id LIMIT 1), '')::text AS owner_phone
FROM accounts a
LEFT JOIN subscriptions s ON s.account_id = a.id
WHERE a.id = @account_id;

-- name: ListAccountsForBilling :many
SELECT id FROM accounts WHERE status <> 'closed' ORDER BY id;

-- name: SetAccountStatus :execrows
UPDATE accounts SET status = @status
WHERE id = @id AND status <> @status AND status <> 'closed';

-- name: MarkFirstDirectBooking :execrows
UPDATE accounts SET first_direct_booking_at = @at
WHERE id = @id AND first_direct_booking_at IS NULL;

-- Units guests sleep in, as the owner app counts them: a property with both whole-house units
-- (those with linked rooms) and rooms counts only the rooms.
-- name: BookableUnits :one
SELECT coalesce(sum(CASE WHEN c.parents > 0 AND c.total > c.parents THEN c.total - c.parents
                         ELSE c.total END), 0)::int AS units
FROM (
    SELECT p.id,
           count(u.id) AS total,
           count(u.id) FILTER (WHERE EXISTS (SELECT 1 FROM unit_links l WHERE l.parent_unit_id = u.id)) AS parents
    FROM properties p
    JOIN units u ON u.property_id = p.id AND u.archived_at IS NULL
    WHERE p.account_id = @account_id
    GROUP BY p.id
) c;
