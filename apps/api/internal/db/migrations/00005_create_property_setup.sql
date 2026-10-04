-- Everything an owner sets up for a property: details, stay rules, policies, booking and
-- branding settings on the property itself, and units, photos, pricing, payment settings and
-- calendar feeds in their own tables. Every table carries account_id, and composite foreign keys
-- keep children inside their property's account.
--
-- The base rate moves from properties onto units: a property's base rate is now its lowest unit
-- rate. Existing development properties get one unit at their old base rate.

-- +goose Up
ALTER TABLE properties
    ADD COLUMN description        text       NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    ADD COLUMN lat                double precision CHECK (lat BETWEEN -90 AND 90),
    ADD COLUMN lng                double precision CHECK (lng BETWEEN -180 AND 180),
    ADD COLUMN time_zone          text       NOT NULL DEFAULT 'Asia/Colombo',
    ADD COLUMN check_in_time      text       NOT NULL DEFAULT '14:00' CHECK (check_in_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    ADD COLUMN check_out_time     text       NOT NULL DEFAULT '11:00' CHECK (check_out_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    ADD COLUMN amenities          text[]     NOT NULL DEFAULT '{}',
    ADD COLUMN policy             text       NOT NULL DEFAULT 'moderate' CHECK (policy IN ('flexible', 'moderate', 'strict')),
    ADD COLUMN deposit_percent    smallint   NOT NULL DEFAULT 30 CHECK (deposit_percent BETWEEN 0 AND 100),
    ADD COLUMN balance_due_days   smallint   NOT NULL DEFAULT 14 CHECK (balance_due_days BETWEEN 0 AND 90),
    ADD COLUMN house_rules        text[]     NOT NULL DEFAULT '{"No smoking indoors","No parties or events"}',
    ADD COLUMN booking_mode       text       NOT NULL DEFAULT 'instant' CHECK (booking_mode IN ('instant', 'request')),
    ADD COLUMN reply_hours        smallint   NOT NULL DEFAULT 24 CHECK (reply_hours BETWEEN 1 AND 72),
    ADD COLUMN hold_minutes       smallint   NOT NULL DEFAULT 15 CHECK (hold_minutes BETWEEN 5 AND 120),
    ADD COLUMN display_currencies text[]     NOT NULL DEFAULT '{}',
    ADD COLUMN brand_color        text       NOT NULL DEFAULT '#09090b' CHECK (brand_color ~ '^#[0-9a-f]{6}$'),
    ADD COLUMN logo_key           text,
    ADD COLUMN ical_export_token  text,
    ADD COLUMN status             text       NOT NULL DEFAULT 'live' CHECK (status IN ('live', 'paused')),
    ADD COLUMN min_nights         smallint   NOT NULL DEFAULT 1 CHECK (min_nights BETWEEN 1 AND 30),
    ADD COLUMN max_nights         smallint   NOT NULL DEFAULT 30 CHECK (max_nights BETWEEN 1 AND 365),
    ADD COLUMN same_day_cutoff    smallint   DEFAULT 12 CHECK (same_day_cutoff BETWEEN 0 AND 23),
    ADD COLUMN window_months      smallint   NOT NULL DEFAULT 12 CHECK (window_months BETWEEN 1 AND 24),
    ADD COLUMN closed_arrival     smallint[] NOT NULL DEFAULT '{}',
    ADD COLUMN extra_guest_above  smallint   NOT NULL DEFAULT 0 CHECK (extra_guest_above BETWEEN 0 AND 30),
    ADD COLUMN extra_guest_amount bigint     NOT NULL DEFAULT 0 CHECK (extra_guest_amount >= 0),
    ADD CONSTRAINT properties_nights_check CHECK (max_nights >= min_nights);

UPDATE properties
SET ical_export_token  = substr(encode(sha256(gen_random_uuid()::text::bytea), 'hex'), 1, 24),
    display_currencies = ARRAY[currency::text];

ALTER TABLE properties ALTER COLUMN ical_export_token SET NOT NULL;
CREATE UNIQUE INDEX properties_ical_export_token_key ON properties (ical_export_token);

-- Bookable units: the whole place, or each room. Archived rather than deleted, so stays keep
-- pointing at the unit they booked.
CREATE TABLE units (
    id           uuid        PRIMARY KEY,
    account_id   uuid        NOT NULL,
    property_id  uuid        NOT NULL,
    name         text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    sleeps       smallint    NOT NULL CHECK (sleeps BETWEEN 1 AND 30),
    beds         text        NOT NULL DEFAULT '' CHECK (char_length(beds) <= 80),
    rate         bigint      NOT NULL CHECK (rate > 0),
    weekend_rate bigint      CHECK (weekend_rate > 0),
    position     smallint    NOT NULL DEFAULT 0,
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, account_id),
    UNIQUE (id, property_id),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX units_account_id_idx ON units (account_id);
CREATE INDEX units_property_id_idx ON units (property_id, position) WHERE archived_at IS NULL;

CREATE TRIGGER units_updated_at BEFORE UPDATE ON units
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO units (id, account_id, property_id, name, sleeps, rate)
SELECT gen_random_uuid(), account_id, id,
       CASE booking_type WHEN 'entire' THEN name ELSE 'Room 1' END,
       CASE booking_type WHEN 'entire' THEN 4 ELSE 2 END,
       base_rate_minor
FROM properties;

ALTER TABLE properties DROP COLUMN base_rate_minor;

-- A whole-house unit and the rooms it books together.
CREATE TABLE unit_links (
    account_id     uuid        NOT NULL,
    property_id    uuid        NOT NULL,
    parent_unit_id uuid        NOT NULL,
    child_unit_id  uuid        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (parent_unit_id, child_unit_id),
    CHECK (parent_unit_id <> child_unit_id),
    FOREIGN KEY (parent_unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (child_unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX unit_links_account_id_idx ON unit_links (account_id);
CREATE INDEX unit_links_property_id_idx ON unit_links (property_id);

CREATE TABLE property_photos (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    key         text        NOT NULL,
    caption     text        NOT NULL DEFAULT '' CHECK (char_length(caption) <= 120),
    position    smallint    NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (property_id, key),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX property_photos_account_id_idx ON property_photos (account_id);
CREATE INDEX property_photos_property_id_idx ON property_photos (property_id, position);

CREATE TRIGGER property_photos_updated_at BEFORE UPDATE ON property_photos
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per property. The account number is encrypted by the API before it is stored.
CREATE TABLE payment_settings (
    property_id        uuid        PRIMARY KEY,
    account_id         uuid        NOT NULL,
    bank_enabled       boolean     NOT NULL DEFAULT false,
    bank_name          text        NOT NULL DEFAULT '' CHECK (char_length(bank_name) <= 80),
    account_name       text        NOT NULL DEFAULT '' CHECK (char_length(account_name) <= 80),
    account_number_enc bytea,
    pay_within_hours   smallint    NOT NULL DEFAULT 24 CHECK (pay_within_hours BETWEEN 1 AND 168),
    cancel_if_unpaid   boolean     NOT NULL DEFAULT true,
    at_property        boolean     NOT NULL DEFAULT true,
    cards_status       text        NOT NULL DEFAULT 'off' CHECK (cards_status IN ('off', 'pending', 'on')),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX payment_settings_account_id_idx ON payment_settings (account_id);

CREATE TRIGGER payment_settings_updated_at BEFORE UPDATE ON payment_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO payment_settings (property_id, account_id)
SELECT id, account_id FROM properties;

-- Seasons repeat every year between two "MM-DD" days and may wrap the new year.
CREATE TABLE seasons (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    name        text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    start_md    char(5)     NOT NULL CHECK (start_md ~ '^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'),
    end_md      char(5)     NOT NULL CHECK (end_md ~ '^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'),
    min_nights  smallint    CHECK (min_nights BETWEEN 1 AND 30),
    position    smallint    NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, property_id),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX seasons_account_id_idx ON seasons (account_id);
CREATE INDEX seasons_property_id_idx ON seasons (property_id, position);

CREATE TRIGGER seasons_updated_at BEFORE UPDATE ON seasons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE season_prices (
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    season_id   uuid        NOT NULL,
    unit_id     uuid        NOT NULL,
    price       bigint      NOT NULL CHECK (price > 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (season_id, unit_id),
    FOREIGN KEY (season_id, property_id) REFERENCES seasons (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE,
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX season_prices_account_id_idx ON season_prices (account_id);
CREATE INDEX season_prices_property_id_idx ON season_prices (property_id);

CREATE TABLE length_discounts (
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    nights      smallint    NOT NULL CHECK (nights BETWEEN 2 AND 365),
    percent     smallint    NOT NULL CHECK (percent BETWEEN 1 AND 90),
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (property_id, nights),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX length_discounts_account_id_idx ON length_discounts (account_id);

-- Service charge, VAT, cleaning fee: percent amounts are whole numbers, fixed ones minor units.
CREATE TABLE charges (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    name        text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    kind        text        NOT NULL CHECK (kind IN ('percent', 'fixed')),
    amount      bigint      NOT NULL CHECK (amount >= 0),
    per         text        NOT NULL CHECK (per IN ('stay', 'night', 'guest')),
    enabled     boolean     NOT NULL DEFAULT true,
    note        text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 120),
    position    smallint    NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (kind <> 'percent' OR amount <= 100),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX charges_account_id_idx ON charges (account_id);
CREATE INDEX charges_property_id_idx ON charges (property_id, position);

CREATE TRIGGER charges_updated_at BEFORE UPDATE ON charges
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE extras (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    name        text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    price       bigint      NOT NULL CHECK (price >= 0),
    per         text        NOT NULL CHECK (per IN ('stay', 'night', 'guest', 'trip', 'guestNight')),
    on_request  boolean     NOT NULL DEFAULT false,
    enabled     boolean     NOT NULL DEFAULT true,
    position    smallint    NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX extras_account_id_idx ON extras (account_id);
CREATE INDEX extras_property_id_idx ON extras (property_id, position);

CREATE TRIGGER extras_updated_at BEFORE UPDATE ON extras
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE promos (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL,
    property_id uuid        NOT NULL,
    code        text        NOT NULL CHECK (code ~ '^[A-Z0-9-]{3,20}$'),
    kind        text        NOT NULL CHECK (kind IN ('percent', 'fixed')),
    amount      bigint      NOT NULL CHECK (amount > 0),
    starts_on   date,
    ends_on     date,
    usage_limit integer     CHECK (usage_limit > 0),
    used_count  integer     NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    min_nights  smallint    CHECK (min_nights BETWEEN 1 AND 365),
    note        text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 120),
    position    smallint    NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (kind <> 'percent' OR amount <= 100),
    CHECK (starts_on IS NULL OR ends_on IS NULL OR starts_on <= ends_on),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX promos_property_id_code_key ON promos (property_id, upper(code));
CREATE INDEX promos_account_id_idx ON promos (account_id);

CREATE TRIGGER promos_updated_at BEFORE UPDATE ON promos
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Calendars imported from OTAs. Sync status is written by the import worker (phase 5).
CREATE TABLE ical_feeds (
    id                   uuid        PRIMARY KEY,
    account_id           uuid        NOT NULL,
    property_id          uuid        NOT NULL,
    unit_id              uuid,
    channel              text        NOT NULL CHECK (channel IN ('airbnb', 'booking', 'agoda', 'expedia', 'other')),
    url                  text        NOT NULL DEFAULT '' CHECK (char_length(url) <= 1000),
    status               text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ok', 'error')),
    last_synced_at       timestamptz,
    last_error           text,
    etag                 text,
    consecutive_failures integer     NOT NULL DEFAULT 0,
    upcoming             integer     NOT NULL DEFAULT 0,
    position             smallint    NOT NULL DEFAULT 0,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (unit_id, property_id) REFERENCES units (id, property_id) ON DELETE CASCADE
);

CREATE INDEX ical_feeds_account_id_idx ON ical_feeds (account_id);
CREATE INDEX ical_feeds_property_id_idx ON ical_feeds (property_id, position);

CREATE TRIGGER ical_feeds_updated_at BEFORE UPDATE ON ical_feeds
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE ical_feeds;
DROP TABLE promos;
DROP TABLE extras;
DROP TABLE charges;
DROP TABLE length_discounts;
DROP TABLE season_prices;
DROP TABLE seasons;
DROP TABLE payment_settings;
DROP TABLE property_photos;
DROP TABLE unit_links;

ALTER TABLE properties ADD COLUMN base_rate_minor bigint;
UPDATE properties p SET base_rate_minor = coalesce(
    (SELECT min(rate) FROM units u WHERE u.property_id = p.id AND u.archived_at IS NULL), 1);
ALTER TABLE properties
    ALTER COLUMN base_rate_minor SET NOT NULL,
    ADD CONSTRAINT properties_base_rate_minor_check CHECK (base_rate_minor > 0);

DROP TABLE units;

DROP INDEX properties_ical_export_token_key;
ALTER TABLE properties
    DROP CONSTRAINT properties_nights_check,
    DROP COLUMN description, DROP COLUMN lat, DROP COLUMN lng, DROP COLUMN time_zone,
    DROP COLUMN check_in_time, DROP COLUMN check_out_time, DROP COLUMN amenities,
    DROP COLUMN policy, DROP COLUMN deposit_percent, DROP COLUMN balance_due_days,
    DROP COLUMN house_rules, DROP COLUMN booking_mode, DROP COLUMN reply_hours,
    DROP COLUMN hold_minutes, DROP COLUMN display_currencies, DROP COLUMN brand_color,
    DROP COLUMN logo_key, DROP COLUMN ical_export_token, DROP COLUMN status,
    DROP COLUMN min_nights, DROP COLUMN max_nights, DROP COLUMN same_day_cutoff,
    DROP COLUMN window_months, DROP COLUMN closed_arrival, DROP COLUMN extra_guest_above,
    DROP COLUMN extra_guest_amount;
