-- Billing: the plan an account pays for, the provider events that changed it, and web checkouts
-- through Payments.lk. Access itself is worked out from these and kept in accounts.status.
--
-- An account has at most one subscription row. Store plans (App Store, Google Play) come from
-- RevenueCat; web plans are prepaid periods bought on staykey.direct/billing.

-- +goose Up
CREATE TABLE subscriptions (
    account_id         uuid        PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    source             text        NOT NULL CHECK (source IN ('app_store', 'play_store', 'web')),
    plan               text        NOT NULL CHECK (plan IN ('villa', 'guesthouse', 'collection')),
    period             text        NOT NULL CHECK (period IN ('monthly', 'yearly')),
    status             text        NOT NULL CHECK (status IN ('active', 'cancelled', 'billing_issue', 'expired')),
    will_renew         boolean     NOT NULL DEFAULT false,
    current_period_end timestamptz NOT NULL,
    product_id         text        NOT NULL DEFAULT '' CHECK (char_length(product_id) <= 200),
    external_id        text        NOT NULL DEFAULT '' CHECK (char_length(external_id) <= 200),
    last_event_at      timestamptz NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER subscriptions_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Every webhook delivery we acted on, so a retried delivery is applied once. account_id is null
-- for events that name no account of ours.
CREATE TABLE billing_events (
    id          uuid        PRIMARY KEY,
    account_id  uuid        REFERENCES accounts (id) ON DELETE SET NULL,
    source      text        NOT NULL CHECK (source IN ('revenuecat', 'paymentslk')),
    event_id    text        NOT NULL CHECK (char_length(event_id) BETWEEN 1 AND 200),
    type        text        NOT NULL CHECK (char_length(type) <= 100),
    payload     jsonb       NOT NULL DEFAULT '{}',
    received_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, event_id)
);

CREATE INDEX billing_events_account_id_idx ON billing_events (account_id, received_at DESC);

-- A web purchase of one prepaid period. Its id is the reference sent to Payments.lk.
CREATE TABLE web_checkouts (
    id                 uuid        PRIMARY KEY,
    account_id         uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    plan               text        NOT NULL CHECK (plan IN ('villa', 'guesthouse', 'collection')),
    period             text        NOT NULL CHECK (period IN ('monthly', 'yearly')),
    amount             bigint      NOT NULL CHECK (amount > 0),
    currency           text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status             text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed')),
    provider_id        text        NOT NULL DEFAULT '' CHECK (char_length(provider_id) <= 200),
    provider_order_id  text        NOT NULL DEFAULT '' CHECK (char_length(provider_order_id) <= 200),
    period_start       timestamptz,
    period_end         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    paid_at            timestamptz,
    CHECK (status <> 'paid' OR (period_start IS NOT NULL AND period_end IS NOT NULL AND paid_at IS NOT NULL))
);

CREATE INDEX web_checkouts_account_id_idx ON web_checkouts (account_id, created_at DESC);

ALTER TABLE activity DROP CONSTRAINT activity_kind_check;
ALTER TABLE activity ADD CONSTRAINT activity_kind_check
    CHECK (kind IN ('request', 'booking', 'slip', 'payment', 'cancellation', 'sync', 'import', 'billing'));

-- The first direct booking ends the free period. Count it from the moment a page or widget
-- booking first reached awaiting_payment or confirmed, as the API does from now on.
UPDATE accounts a
SET first_direct_booking_at = first.at
FROM (
    SELECT b.account_id, min(e.created_at) AS at
    FROM bookings b
    JOIN booking_events e ON e.booking_id = b.id AND e.account_id = b.account_id
    WHERE b.source IN ('page', 'widget') AND e.to_status IN ('awaiting_payment', 'confirmed')
    GROUP BY b.account_id
) first
WHERE a.id = first.account_id AND a.first_direct_booking_at IS NULL;

-- +goose Down
DELETE FROM activity WHERE kind = 'billing';
ALTER TABLE activity DROP CONSTRAINT activity_kind_check;
ALTER TABLE activity ADD CONSTRAINT activity_kind_check
    CHECK (kind IN ('request', 'booking', 'slip', 'payment', 'cancellation', 'sync', 'import'));
DROP TABLE web_checkouts;
DROP TABLE billing_events;
DROP TABLE subscriptions;
