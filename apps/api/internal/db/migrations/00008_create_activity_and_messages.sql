-- Activity, read state, message templates, and the columns alerts need: when a summary last went
-- out, and which session a push token belongs to.
--
-- Activity rows double as the alert outbox: notified_at stays null until the notify job has sent
-- the push and WhatsApp messages for the row.

-- +goose Up
CREATE TABLE activity (
    id            uuid        PRIMARY KEY,
    account_id    uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    property_id   uuid,
    booking_id    uuid,
    kind          text        NOT NULL
                  CHECK (kind IN ('request', 'booking', 'slip', 'payment', 'cancellation', 'sync', 'import')),
    title         text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    subtitle      text        NOT NULL DEFAULT '' CHECK (char_length(subtitle) <= 300),
    actor_user_id uuid        REFERENCES users (id) ON DELETE SET NULL,
    notified_at   timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (property_id, account_id) REFERENCES properties (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE CASCADE
);

CREATE INDEX activity_account_id_created_at_idx ON activity (account_id, created_at DESC, id DESC);
CREATE INDEX activity_unnotified_idx ON activity (created_at) WHERE notified_at IS NULL;

-- Everything at or before last_read_at counts as read for that person in that account.
CREATE TABLE activity_reads (
    account_id   uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    last_read_at timestamptz NOT NULL,
    PRIMARY KEY (account_id, user_id)
);

CREATE TABLE message_templates (
    id         uuid        PRIMARY KEY,
    account_id uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    key        text        NOT NULL CHECK (key IN ('confirmed', 'reminder', 'prearrival', 'thanks')),
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    channels   text[]      NOT NULL CHECK (channels <@ ARRAY['email', 'whatsapp']::text[]),
    timing     text        NOT NULL DEFAULT '' CHECK (char_length(timing) <= 80),
    body       text        NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, key),
    UNIQUE (id, account_id)
);

CREATE TRIGGER message_templates_updated_at BEFORE UPDATE ON message_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The day the morning summary last went out, on the 'summary' row.
ALTER TABLE notification_prefs ADD COLUMN last_sent_on date;

-- A token belongs to the session that registered it, so signing out stops its alerts.
ALTER TABLE push_tokens ADD COLUMN session_id uuid REFERENCES sessions (id) ON DELETE CASCADE;

-- Existing accounts and members get the same rows new ones get from the API. These rows use
-- gen_random_uuid() because the API, which makes UUIDv7 keys, isn't involved here.
INSERT INTO message_templates (id, account_id, key, name, channels, timing, body)
SELECT gen_random_uuid(), a.id, t.key, t.name, t.channels, t.timing, t.body
FROM accounts a
CROSS JOIN (VALUES
    ('confirmed', 'Booking confirmed', ARRAY['email']::text[], 'Right after confirmation',
     'Hi {guest_name}, your stay at {property_name} is confirmed for {check_in} to {check_out}. We can''t wait to host you.'),
    ('reminder', 'Payment reminder', ARRAY['email', 'whatsapp']::text[], 'A day before it''s due',
     'Hi {guest_name}, a friendly reminder that {balance} is due for your stay at {property_name}. You can pay here: {booking_link}'),
    ('prearrival', 'Pre-arrival info', ARRAY['email', 'whatsapp']::text[], '2 days before check-in',
     'Hi {guest_name}, we look forward to welcoming you on {check_in}. Here is the map pin: {map_link}. Check-in is from {check_in_time}.'),
    ('thanks', 'Thank you', ARRAY['email']::text[], 'The day after check-out',
     'Thank you for staying at {property_name}, {guest_name}. Book directly next time and we''ll take care of the rest: {booking_link}')
) AS t (key, name, channels, timing, body)
ON CONFLICT (account_id, key) DO NOTHING;

INSERT INTO notification_prefs (id, account_id, user_id, event, push, whatsapp)
SELECT gen_random_uuid(), m.account_id, m.user_id, p.event, p.push, p.whatsapp
FROM memberships m
CROSS JOIN (VALUES
    ('booking', true, true),
    ('request', true, true),
    ('payment', true, false),
    ('cancellation', true, false),
    ('sync', true, false),
    ('summary', false, true)
) AS p (event, push, whatsapp)
ON CONFLICT (user_id, account_id, event) DO NOTHING;

-- +goose Down
ALTER TABLE push_tokens DROP COLUMN session_id;
ALTER TABLE notification_prefs DROP COLUMN last_sent_on;
DROP TABLE message_templates;
DROP TABLE activity_reads;
DROP TABLE activity;
