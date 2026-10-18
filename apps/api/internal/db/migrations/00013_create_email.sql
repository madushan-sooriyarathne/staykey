-- Email through AWS SES. Every message is one row in the emails outbox, queued inside the
-- transaction that causes it and sent later by a job, outside any transaction.
--
-- Rows with an account_id follow the tenant wall. Account emails to a person (welcome, new
-- sign-in, deletion) have no account_id, so tenant requests never see them; only jobs do.
-- user_id is SET NULL so the email confirming a finished deletion survives the purge.

-- +goose Up
CREATE TABLE emails (
    id                  uuid        PRIMARY KEY,
    account_id          uuid        REFERENCES accounts (id) ON DELETE CASCADE,
    booking_id          uuid,
    user_id             uuid        REFERENCES users (id) ON DELETE SET NULL,
    kind                text        NOT NULL CHECK (kind IN (
        'welcome', 'confirm_email', 'email_changed', 'new_sign_in', 'deletion_scheduled',
        'account_restored', 'deletion_completed', 'plan_receipt',
        'received', 'confirmed', 'declined', 'reminder', 'prearrival', 'thanks')),
    to_address          text        NOT NULL CHECK (char_length(to_address) BETWEEN 3 AND 254),
    payload             jsonb       NOT NULL DEFAULT '{}',
    dedupe_key          text        UNIQUE,
    status              text        NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'skipped', 'suppressed', 'failed')),
    attempts            smallint    NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    scheduled_for       timestamptz NOT NULL,
    claimed_at          timestamptz,
    sent_at             timestamptz,
    provider_message_id text,
    last_error          text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (booking_id IS NULL OR account_id IS NOT NULL),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE
);

CREATE INDEX emails_due_idx ON emails (scheduled_for) WHERE status = 'pending';
CREATE INDEX emails_booking_id_idx ON emails (booking_id) WHERE booking_id IS NOT NULL;
CREATE INDEX emails_user_id_idx ON emails (user_id) WHERE user_id IS NOT NULL;

ALTER TABLE emails ENABLE ROW LEVEL SECURITY;
ALTER TABLE emails FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_wall ON emails
    USING (nullif(current_setting('staykey.account_id', true), '') IS NULL
           OR account_id = nullif(current_setting('staykey.account_id', true), '')::uuid)
    WITH CHECK (nullif(current_setting('staykey.account_id', true), '') IS NULL
           OR account_id = nullif(current_setting('staykey.account_id', true), '')::uuid);

-- Addresses SES reported as a permanent bounce or a complaint. Nothing is sent to them again.
CREATE TABLE email_suppressions (
    address    text        PRIMARY KEY CHECK (address = lower(address)),
    reason     text        NOT NULL CHECK (reason IN ('bounce', 'complaint')),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- One row per confirmation link sent. Only the SHA-256 of the token is kept.
CREATE TABLE email_confirmations (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email      text        NOT NULL CHECK (char_length(email) <= 254),
    token_hash bytea       NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX email_confirmations_user_id_idx ON email_confirmations (user_id);

ALTER TABLE users ADD COLUMN email_confirmed_at timestamptz;
ALTER TABLE notification_prefs ADD COLUMN email boolean NOT NULL DEFAULT false;

-- The guest's stay token, sealed with the server's secret box and bound to the booking id, so
-- emails can carry the stay page link. guest_token_hash stays the lookup key.
ALTER TABLE bookings ADD COLUMN guest_token_sealed bytea;

-- +goose Down
ALTER TABLE bookings DROP COLUMN guest_token_sealed;
ALTER TABLE notification_prefs DROP COLUMN email;
ALTER TABLE users DROP COLUMN email_confirmed_at;
DROP TABLE email_confirmations;
DROP TABLE email_suppressions;
DROP TABLE emails;
