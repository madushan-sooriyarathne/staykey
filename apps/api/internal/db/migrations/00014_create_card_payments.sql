-- Guest card payments through Payments.lk hosted checkouts.
--
-- Each account picks where card money goes (accounts.card_mode): its own Payments.lk merchant
-- account (own, through card_connections) or StayKey's collection account, paid out to the
-- owner later (payouts). Each property then switches cards on or off.
--
-- payment_settings.cards_status now holds what the owner chose, off or on. Whether cards are
-- ready (a mode, a working connection, a rate for properties not priced in rupees) is worked
-- out when the settings are read.
--
-- Payments.lk only charges rupees, so card checkouts and card payments carry the rupee amount
-- charged and the rate used beside the booking-currency amount.

-- +goose Up
ALTER TABLE accounts ADD COLUMN card_mode text NOT NULL DEFAULT 'off'
    CHECK (card_mode IN ('off', 'own', 'payouts'));

-- The owner's own Payments.lk secret key, sealed with the account id. Only its last four
-- characters and its mode are ever shown. events_since is where the next events poll starts.
CREATE TABLE card_connections (
    account_id        uuid        PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    secret_key_sealed bytea       NOT NULL,
    key_last4         text        NOT NULL CHECK (char_length(key_last4) = 4),
    mode              text        NOT NULL CHECK (mode IN ('test', 'live')),
    status            text        NOT NULL DEFAULT 'connected' CHECK (status IN ('connected', 'failed')),
    checked_at        timestamptz NOT NULL,
    events_since      timestamptz NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER card_connections_updated_at BEFORE UPDATE ON card_connections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

UPDATE payment_settings SET cards_status = 'on' WHERE cards_status = 'pending';

-- card_rate is rupee cents per 1.00 of the property's currency, needed unless it is LKR.
ALTER TABLE payment_settings
    ADD COLUMN card_charge text NOT NULL DEFAULT 'deposit' CHECK (card_charge IN ('deposit', 'full')),
    ADD COLUMN card_rate   bigint CHECK (card_rate BETWEEN 1 AND 10000000);

-- One hosted checkout for part of a stay. Its id is the reference and idempotency key sent to
-- Payments.lk. amount is in the booking's currency, charged_cents in rupees.
CREATE TABLE card_checkouts (
    id                   uuid        PRIMARY KEY,
    account_id           uuid        NOT NULL,
    booking_id           uuid        NOT NULL,
    collected_by         text        NOT NULL CHECK (collected_by IN ('owner', 'staykey')),
    purpose              text        NOT NULL CHECK (purpose IN ('deposit', 'full', 'balance')),
    amount               bigint      NOT NULL CHECK (amount > 0),
    currency             char(3)     NOT NULL,
    charged_cents        bigint      NOT NULL CHECK (charged_cents > 0),
    rate                 bigint      CHECK (rate > 0),
    mode                 text        NOT NULL CHECK (mode IN ('test', 'live')),
    provider_checkout_id text        UNIQUE CHECK (char_length(provider_checkout_id) <= 200),
    provider_payment_id  text        CHECK (char_length(provider_payment_id) <= 200),
    status               text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'paid', 'expired')),
    expires_at           timestamptz NOT NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    paid_at              timestamptz,
    UNIQUE (id, account_id),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE
);

CREATE INDEX card_checkouts_account_id_idx ON card_checkouts (account_id);
CREATE INDEX card_checkouts_booking_id_idx ON card_checkouts (booking_id, created_at DESC);

-- Every Payments.lk event acted on, per connection: owner:<account id> or staykey.
CREATE TABLE card_events (
    source      text        NOT NULL CHECK (char_length(source) <= 60),
    event_id    text        NOT NULL CHECK (char_length(event_id) BETWEEN 1 AND 200),
    account_id  uuid        REFERENCES accounts (id) ON DELETE CASCADE,
    type        text        NOT NULL CHECK (char_length(type) <= 100),
    received_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (source, event_id)
);

CREATE INDEX card_events_account_id_idx ON card_events (account_id) WHERE account_id IS NOT NULL;

-- Where the next events poll starts for StayKey's own collection key. Owner connections keep
-- theirs on card_connections.
CREATE TABLE card_cursors (
    source       text        PRIMARY KEY CHECK (char_length(source) <= 60),
    events_since timestamptz NOT NULL
);

-- A card refund is pending from the cancel screen until Payments.lk reports it. refund_of is the
-- card payment it goes back to; attempted_at marks the last time it was sent.
ALTER TABLE payments
    ADD COLUMN status             text    NOT NULL DEFAULT 'succeeded'
        CHECK (status IN ('pending', 'succeeded', 'failed')),
    ADD COLUMN charged_cents      bigint  CHECK (charged_cents > 0),
    ADD COLUMN charged_currency   char(3),
    ADD COLUMN rate               bigint  CHECK (rate > 0),
    ADD COLUMN card_checkout_id   uuid,
    ADD COLUMN collected_by       text    CHECK (collected_by IN ('owner', 'staykey')),
    ADD COLUMN provider_fee_cents bigint  CHECK (provider_fee_cents >= 0),
    ADD COLUMN refund_of          uuid,
    ADD COLUMN failure            text    CHECK (char_length(failure) <= 300),
    ADD COLUMN attempted_at       timestamptz,
    ADD CONSTRAINT payments_id_account_id_key UNIQUE (id, account_id),
    ADD CONSTRAINT payments_card_checkout_fkey FOREIGN KEY (card_checkout_id, account_id)
        REFERENCES card_checkouts (id, account_id) ON DELETE SET NULL (card_checkout_id),
    ADD CONSTRAINT payments_refund_of_fkey FOREIGN KEY (refund_of, account_id)
        REFERENCES payments (id, account_id) ON DELETE CASCADE,
    ADD CONSTRAINT payments_refund_of_check CHECK (refund_of IS NULL OR kind = 'refund');

CREATE UNIQUE INDEX payments_account_id_provider_ref_key ON payments (account_id, provider_ref)
    WHERE provider_ref IS NOT NULL;
CREATE INDEX payments_provider_ref_idx ON payments (provider_ref) WHERE provider_ref IS NOT NULL;
CREATE INDEX payments_refund_of_idx ON payments (refund_of) WHERE refund_of IS NOT NULL;
CREATE INDEX payments_pending_refunds_idx ON payments (created_at) WHERE status = 'pending';

ALTER TABLE bookings DROP CONSTRAINT bookings_payment_method_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_payment_method_check
    CHECK (payment_method IN ('bank', 'property', 'card'));

-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['card_connections', 'card_checkouts', 'card_events'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$
            CREATE POLICY tenant_wall ON %I
            USING (nullif(current_setting('staykey.account_id', true), '') IS NULL
                   OR account_id = nullif(current_setting('staykey.account_id', true), '')::uuid)
            WITH CHECK (nullif(current_setting('staykey.account_id', true), '') IS NULL
                   OR account_id = nullif(current_setting('staykey.account_id', true), '')::uuid)
        $p$, t);
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- +goose Down
UPDATE bookings SET payment_method = NULL WHERE payment_method = 'card';
ALTER TABLE bookings DROP CONSTRAINT bookings_payment_method_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_payment_method_check
    CHECK (payment_method IN ('bank', 'property'));

DELETE FROM payments WHERE status = 'failed';
DROP INDEX payments_pending_refunds_idx;
DROP INDEX payments_refund_of_idx;
DROP INDEX payments_provider_ref_idx;
DROP INDEX payments_account_id_provider_ref_key;
ALTER TABLE payments
    DROP CONSTRAINT payments_refund_of_check,
    DROP CONSTRAINT payments_refund_of_fkey,
    DROP CONSTRAINT payments_card_checkout_fkey,
    DROP CONSTRAINT payments_id_account_id_key,
    DROP COLUMN attempted_at,
    DROP COLUMN failure,
    DROP COLUMN refund_of,
    DROP COLUMN provider_fee_cents,
    DROP COLUMN collected_by,
    DROP COLUMN card_checkout_id,
    DROP COLUMN rate,
    DROP COLUMN charged_currency,
    DROP COLUMN charged_cents,
    DROP COLUMN status;

DROP TABLE card_cursors;
DROP TABLE card_events;
DROP TABLE card_checkouts;

ALTER TABLE payment_settings DROP COLUMN card_rate, DROP COLUMN card_charge;
UPDATE payment_settings SET cards_status = 'pending' WHERE cards_status = 'on';

DROP TABLE card_connections;
ALTER TABLE accounts DROP COLUMN card_mode;
