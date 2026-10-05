-- Guests, bookings with their frozen price lines, payments and bank slips, blocks, rate
-- overrides and the night ledger that keeps two stays off the same unit on the same night.
--
-- Every table carries account_id, and composite foreign keys keep children inside their
-- booking's or property's account.

-- +goose Up
-- Booking refs (KV-2041) count up per account. The counter row is locked while a booking is
-- created, which also queues concurrent bookings in one account behind each other.
ALTER TABLE accounts ADD COLUMN booking_seq integer NOT NULL DEFAULT 2040;

CREATE TABLE guests (
    id         uuid        PRIMARY KEY,
    account_id uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    phone      text        CHECK (char_length(phone) <= 32),
    email      text        CHECK (char_length(email) <= 254),
    country    text        CHECK (char_length(country) <= 2),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, account_id)
);

CREATE INDEX guests_account_id_phone_idx ON guests (account_id, phone) WHERE phone IS NOT NULL;
CREATE INDEX guests_account_id_email_idx ON guests (account_id, lower(email)) WHERE email IS NOT NULL;

CREATE TRIGGER guests_updated_at BEFORE UPDATE ON guests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE bookings (
    id                 uuid        PRIMARY KEY,
    account_id         uuid        NOT NULL,
    property_id        uuid        NOT NULL,
    unit_id            uuid        NOT NULL,
    ref                text        NOT NULL,
    source             text        NOT NULL CHECK (source IN ('page', 'widget', 'whatsapp', 'phone', 'walkin', 'other', 'airbnb', 'booking', 'agoda', 'expedia')),
    status             text        NOT NULL CHECK (status IN ('requested', 'awaiting_payment', 'confirmed', 'checked_in', 'checked_out', 'cancelled', 'declined')),
    guest_id           uuid        NOT NULL,
    adults             smallint    NOT NULL CHECK (adults BETWEEN 1 AND 30),
    children           smallint    NOT NULL DEFAULT 0 CHECK (children BETWEEN 0 AND 30),
    check_in           date        NOT NULL,
    check_out          date        NOT NULL,
    currency           char(3)     NOT NULL,
    total              bigint      NOT NULL CHECK (total >= 0),
    guest_note         text        NOT NULL DEFAULT '' CHECK (char_length(guest_note) <= 2000),
    owner_note         text        NOT NULL DEFAULT '' CHECK (char_length(owner_note) <= 2000),
    request_expires_at timestamptz,
    hold_expires_at    timestamptz,
    payment_due_at     timestamptz,
    cancel_reason      text        CHECK (char_length(cancel_reason) <= 500),
    cancelled_at       timestamptz,
    external_uid       text,
    feed_id            uuid        REFERENCES ical_feeds (id) ON DELETE SET NULL,
    promo_id           uuid        REFERENCES promos (id) ON DELETE SET NULL,
    created_by         uuid        REFERENCES users (id) ON DELETE SET NULL,
    version            integer     NOT NULL DEFAULT 1,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (check_out > check_in),
    UNIQUE (account_id, ref),
    UNIQUE (feed_id, external_uid),
    UNIQUE (id, account_id),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (guest_id, account_id) REFERENCES guests (id, account_id)
);

CREATE INDEX bookings_property_id_check_in_idx ON bookings (property_id, check_in);
CREATE INDEX bookings_account_id_check_in_idx ON bookings (account_id, check_in);
CREATE INDEX bookings_guest_id_idx ON bookings (guest_id);
CREATE INDEX bookings_request_expires_at_idx ON bookings (request_expires_at) WHERE status = 'requested';
CREATE INDEX bookings_payment_due_at_idx ON bookings (payment_due_at) WHERE status = 'awaiting_payment';

CREATE TRIGGER bookings_updated_at BEFORE UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The price breakdown at the moment of booking, so guests pay what they were quoted.
CREATE TABLE booking_lines (
    account_id uuid     NOT NULL,
    booking_id uuid     NOT NULL,
    position   smallint NOT NULL,
    label      text     NOT NULL CHECK (char_length(label) BETWEEN 1 AND 120),
    amount     bigint   NOT NULL,
    kind       text     CHECK (kind IN ('discount', 'charge', 'extra')),
    PRIMARY KEY (booking_id, position),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE
);

CREATE INDEX booking_lines_account_id_idx ON booking_lines (account_id);

CREATE TABLE booking_extras (
    account_id uuid   NOT NULL,
    booking_id uuid   NOT NULL,
    extra_id   uuid   NOT NULL,
    name       text   NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    amount     bigint NOT NULL CHECK (amount >= 0),
    PRIMARY KEY (booking_id, extra_id),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE
);

CREATE INDEX booking_extras_account_id_idx ON booking_extras (account_id);

CREATE TABLE payment_slips (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL,
    booking_id  uuid        NOT NULL,
    file_key    text        NOT NULL,
    amount      bigint      NOT NULL CHECK (amount > 0),
    status      text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    reviewed_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    reviewed_at timestamptz,
    UNIQUE (id, account_id),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE
);

CREATE INDEX payment_slips_account_id_idx ON payment_slips (account_id);
CREATE INDEX payment_slips_booking_id_idx ON payment_slips (booking_id, uploaded_at);

-- Money in and out of a booking. A refund is a row with kind refund; the balance is computed.
CREATE TABLE payments (
    id           uuid        PRIMARY KEY,
    account_id   uuid        NOT NULL,
    booking_id   uuid        NOT NULL,
    kind         text        NOT NULL CHECK (kind IN ('payment', 'refund')),
    method       text        NOT NULL CHECK (method IN ('bank', 'cash', 'card')),
    amount       bigint      NOT NULL CHECK (amount > 0),
    currency     char(3)     NOT NULL,
    note         text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
    received_at  timestamptz NOT NULL,
    recorded_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    slip_id      uuid,
    provider_ref text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (slip_id, account_id) REFERENCES payment_slips (id, account_id) ON DELETE SET NULL (slip_id)
);

CREATE INDEX payments_account_id_idx ON payments (account_id);
CREATE INDEX payments_booking_id_idx ON payments (booking_id, received_at);

CREATE TABLE booking_events (
    id            uuid        PRIMARY KEY,
    account_id    uuid        NOT NULL,
    booking_id    uuid        NOT NULL,
    from_status   text,
    to_status     text        NOT NULL,
    actor_user_id uuid        REFERENCES users (id) ON DELETE SET NULL,
    reason        text        NOT NULL DEFAULT '' CHECK (char_length(reason) <= 500),
    created_at    timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE
);

CREATE INDEX booking_events_account_id_idx ON booking_events (account_id);
CREATE INDEX booking_events_booking_id_idx ON booking_events (booking_id, created_at);

-- Dates closed by the owner. Ends exclusive, like a check-out date.
CREATE TABLE blocks (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    starts_on   date        NOT NULL,
    ends_on     date        NOT NULL,
    reason      text        NOT NULL CHECK (reason IN ('maintenance', 'owner', 'other')),
    note        text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
    created_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_on > starts_on),
    UNIQUE (id, property_id),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX blocks_account_id_idx ON blocks (account_id);
CREATE INDEX blocks_property_id_starts_on_idx ON blocks (property_id, starts_on);

CREATE TABLE block_units (
    account_id  uuid NOT NULL,
    property_id uuid NOT NULL,
    block_id    uuid NOT NULL,
    unit_id     uuid NOT NULL,
    PRIMARY KEY (block_id, unit_id),
    FOREIGN KEY (block_id, property_id) REFERENCES blocks (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX block_units_account_id_idx ON block_units (account_id);

-- One row per unit per night a stay or block holds. The primary key is what makes a double
-- booking impossible: a whole-house unit holds the nights of every room it links, so a room
-- and the house clash while two rooms don't.
CREATE TABLE unit_nights (
    unit_id         uuid        NOT NULL,
    night           date        NOT NULL,
    account_id      uuid        NOT NULL,
    property_id     uuid        NOT NULL,
    booking_id      uuid,
    block_id        uuid,
    hold_expires_at timestamptz,
    PRIMARY KEY (unit_id, night),
    CHECK (num_nonnulls(booking_id, block_id) = 1),
    FOREIGN KEY (unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (block_id, property_id) REFERENCES blocks (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX unit_nights_account_id_idx ON unit_nights (account_id);
CREATE INDEX unit_nights_booking_id_idx ON unit_nights (booking_id) WHERE booking_id IS NOT NULL;
CREATE INDEX unit_nights_block_id_idx ON unit_nights (block_id) WHERE block_id IS NOT NULL;
CREATE INDEX unit_nights_hold_expires_at_idx ON unit_nights (hold_expires_at) WHERE hold_expires_at IS NOT NULL;

-- Manual changes to one unit's night: price, minimum stay from that night, closed to arrival.
CREATE TABLE rate_overrides (
    unit_id           uuid        NOT NULL,
    night             date        NOT NULL,
    account_id        uuid        NOT NULL,
    property_id       uuid        NOT NULL,
    price             bigint      CHECK (price > 0),
    min_nights        smallint    CHECK (min_nights BETWEEN 1 AND 365),
    closed_to_arrival boolean     NOT NULL DEFAULT false,
    updated_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (unit_id, night),
    FOREIGN KEY (unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX rate_overrides_account_id_idx ON rate_overrides (account_id);
CREATE INDEX rate_overrides_property_id_night_idx ON rate_overrides (property_id, night);

CREATE TRIGGER rate_overrides_updated_at BEFORE UPDATE ON rate_overrides
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Replies to writes sent with an Idempotency-Key, so a retried request gets the same answer.
-- A row without a response belongs to a request still in flight.
CREATE TABLE idempotency_keys (
    account_id   uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    key          text        NOT NULL CHECK (char_length(key) BETWEEN 1 AND 200),
    request_hash bytea       NOT NULL,
    status       smallint,
    response     jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    PRIMARY KEY (account_id, key)
);

CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TABLE rate_overrides;
DROP TABLE unit_nights;
DROP TABLE block_units;
DROP TABLE blocks;
DROP TABLE booking_events;
DROP TABLE payments;
DROP TABLE payment_slips;
DROP TABLE booking_extras;
DROP TABLE booking_lines;
DROP TABLE bookings;
DROP TABLE guests;
ALTER TABLE accounts DROP COLUMN booking_seq;
