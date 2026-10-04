-- +goose Up
CREATE TABLE properties (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            text        NOT NULL UNIQUE
                    CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$'),
    name            text        NOT NULL CHECK (char_length(name) BETWEEN 2 AND 80),
    booking_type    text        NOT NULL CHECK (booking_type IN ('entire', 'rooms')),
    location        text,
    currency        char(3)     NOT NULL,
    base_rate_minor bigint      NOT NULL CHECK (base_rate_minor > 0),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX properties_created_at_idx ON properties (created_at DESC);

-- +goose Down
DROP TABLE properties;
