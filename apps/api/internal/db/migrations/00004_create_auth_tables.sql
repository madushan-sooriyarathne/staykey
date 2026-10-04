-- Phone sign-in, device sessions, push tokens and per-account notification preferences.

-- +goose Up
-- One row per code sent. Codes are stored hashed; rows also drive the per-phone and per-IP
-- rate limits, so they are kept for a while after use.
CREATE TABLE otp_challenges (
    id          uuid        PRIMARY KEY,
    phone       text        NOT NULL CHECK (phone ~ '^\+[1-9][0-9]{6,14}$'),
    code_hash   bytea       NOT NULL,
    attempts    smallint    NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    ip          inet,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX otp_challenges_phone_created_at_idx ON otp_challenges (phone, created_at DESC);
CREATE INDEX otp_challenges_ip_created_at_idx ON otp_challenges (ip, created_at DESC)
    WHERE ip IS NOT NULL;

CREATE TRIGGER otp_challenges_updated_at BEFORE UPDATE ON otp_challenges
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per signed-in device. The refresh token rotates on every use; only hashes are stored.
-- previous_refresh_hash lets a client retry a refresh whose response it never received, for a
-- short grace period after rotation. Any other stale token revokes the session.
CREATE TABLE sessions (
    id                    uuid        PRIMARY KEY,
    user_id               uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    refresh_hash          bytea       NOT NULL UNIQUE,
    previous_refresh_hash bytea,
    rotated_at            timestamptz NOT NULL DEFAULT now(),
    device_name           text        NOT NULL DEFAULT '' CHECK (char_length(device_name) <= 80),
    last_used_at          timestamptz NOT NULL DEFAULT now(),
    expires_at            timestamptz NOT NULL,
    revoked_at            timestamptz,
    revoked_reason        text        CHECK (revoked_reason IN ('logout', 'reuse')),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TRIGGER sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Expo push tokens. A token moves to whoever signs in on that device last.
CREATE TABLE push_tokens (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token      text        NOT NULL UNIQUE,
    platform   text        NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX push_tokens_user_id_idx ON push_tokens (user_id);

CREATE TRIGGER push_tokens_updated_at BEFORE UPDATE ON push_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE notification_prefs (
    id         uuid        PRIMARY KEY,
    account_id uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    event      text        NOT NULL
               CHECK (event IN ('booking', 'request', 'payment', 'cancellation', 'sync', 'summary')),
    push       boolean     NOT NULL DEFAULT true,
    whatsapp   boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, account_id, event)
);

CREATE INDEX notification_prefs_account_id_idx ON notification_prefs (account_id, user_id);

CREATE TRIGGER notification_prefs_updated_at BEFORE UPDATE ON notification_prefs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE notification_prefs;
DROP TABLE push_tokens;
DROP TABLE sessions;
DROP TABLE otp_challenges;
