-- Properties belong to an account. Memberships can be limited to some properties, and invites
-- bring new team members into an account.

-- +goose Up
-- Properties created before accounts existed (development data only) move to a placeholder
-- account so the column can be NOT NULL. This is the one id generated in SQL.
ALTER TABLE properties ADD COLUMN account_id uuid REFERENCES accounts (id) ON DELETE CASCADE;

-- +goose StatementBegin
DO $$
DECLARE
    placeholder uuid;
BEGIN
    IF EXISTS (SELECT 1 FROM properties) THEN
        INSERT INTO accounts (id, name) VALUES (gen_random_uuid(), 'Unclaimed properties')
            RETURNING id INTO placeholder;
        UPDATE properties SET account_id = placeholder;
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE properties
    ALTER COLUMN account_id SET NOT NULL,
    ALTER COLUMN id DROP DEFAULT,
    ADD CONSTRAINT properties_id_account_id_key UNIQUE (id, account_id);

DROP INDEX properties_created_at_idx;
CREATE INDEX properties_account_id_created_at_idx ON properties (account_id, created_at DESC);

CREATE TRIGGER properties_updated_at BEFORE UPDATE ON properties
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A membership with no rows here covers every property in the account. The composite keys make
-- sure the membership and the property belong to the same account.
CREATE TABLE membership_properties (
    account_id    uuid        NOT NULL,
    membership_id uuid        NOT NULL,
    property_id   uuid        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (membership_id, property_id),
    FOREIGN KEY (membership_id, account_id) REFERENCES memberships (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE
);

CREATE INDEX membership_properties_account_id_idx ON membership_properties (account_id);
CREATE INDEX membership_properties_property_id_idx ON membership_properties (property_id);

-- WhatsApp invite links. The link carries a random token; only its hash is stored.
CREATE TABLE invites (
    id           uuid        PRIMARY KEY,
    account_id   uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    phone        text        NOT NULL CHECK (phone ~ '^\+[1-9][0-9]{6,14}$'),
    name         text        NOT NULL DEFAULT '',
    role         text        NOT NULL CHECK (role IN ('manager', 'caretaker')),
    property_ids uuid[]      NOT NULL DEFAULT '{}',
    token_hash   bytea       NOT NULL UNIQUE,
    invited_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    expires_at   timestamptz NOT NULL,
    accepted_at  timestamptz,
    accepted_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    revoked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX invites_account_id_idx ON invites (account_id, created_at DESC);

CREATE TRIGGER invites_updated_at BEFORE UPDATE ON invites
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE invites;
DROP TABLE membership_properties;
DROP TRIGGER properties_updated_at ON properties;
DROP INDEX properties_account_id_created_at_idx;
CREATE INDEX properties_created_at_idx ON properties (created_at DESC);
ALTER TABLE properties
    DROP CONSTRAINT properties_id_account_id_key,
    ALTER COLUMN id SET DEFAULT gen_random_uuid(),
    DROP COLUMN account_id;
