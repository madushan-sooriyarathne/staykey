-- StayKey payouts: card money StayKey collects for an owner, paid out by bank transfer every
-- other Monday.
--
-- payout_entries is the ledger, one row per money movement in rupee cents, signed: a guest
-- payment is a credit, StayKey's processing charge and refunds are debits. A balance is always
-- a sum of entries. An entry tied to a stay becomes payable the day after the stay's check-in,
-- whatever its own available_on says, so a cancellation before then nets out of money StayKey
-- still holds.
--
-- Preparing a payout attaches every payable entry to it and adds a payout entry for minus their
-- sum. A failed payout keeps its entries for the statement and adds a payout_returned entry
-- that puts the money back.

-- +goose Up
-- The owner's payout details. The NIC and the account number are sealed with the account id;
-- only their last four characters are shown. bank_changed_at holds the next payout for 72 hours.
CREATE TABLE payout_profiles (
    account_id            uuid        PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    legal_name            text        NOT NULL CHECK (char_length(legal_name) BETWEEN 2 AND 120),
    nic_sealed            bytea       NOT NULL,
    nic_last4             text        NOT NULL CHECK (char_length(nic_last4) = 4),
    bank_name             text        NOT NULL CHECK (char_length(bank_name) BETWEEN 2 AND 80),
    branch                text        NOT NULL CHECK (char_length(branch) BETWEEN 2 AND 80),
    account_name          text        NOT NULL CHECK (char_length(account_name) BETWEEN 2 AND 120),
    account_number_sealed bytea       NOT NULL,
    account_last4         text        NOT NULL CHECK (char_length(account_last4) = 4),
    terms_version         text        NOT NULL CHECK (char_length(terms_version) BETWEEN 1 AND 40),
    accepted_at           timestamptz NOT NULL,
    bank_changed_at       timestamptz NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER payout_profiles_updated_at BEFORE UPDATE ON payout_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One batch per payout Monday, prepared once. period_end is the Sunday before; bank_ref is the
-- bank's reference for the transfer batch once StayKey's team has paid it.
CREATE TABLE payout_batches (
    id          uuid        PRIMARY KEY,
    period_end  date        NOT NULL UNIQUE,
    prepared_at timestamptz NOT NULL,
    paid_at     timestamptz,
    bank_ref    text        CHECK (char_length(bank_ref) BETWEEN 1 AND 100)
);

CREATE TABLE payouts (
    id          uuid        PRIMARY KEY,
    account_id  uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    batch_id    uuid        NOT NULL REFERENCES payout_batches (id),
    period_end  date        NOT NULL,
    amount      bigint      NOT NULL CHECK (amount > 0),
    status      text        NOT NULL DEFAULT 'prepared' CHECK (status IN ('prepared', 'paid', 'failed')),
    reason      text        CHECK (char_length(reason) <= 300),
    prepared_at timestamptz NOT NULL,
    paid_at     timestamptz,
    failed_at   timestamptz,
    UNIQUE (id, account_id),
    UNIQUE (account_id, batch_id)
);

CREATE INDEX payouts_account_id_idx ON payouts (account_id, period_end DESC);
CREATE INDEX payouts_batch_id_idx ON payouts (batch_id);

CREATE TABLE payout_entries (
    id           uuid        PRIMARY KEY,
    account_id   uuid        NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    booking_id   uuid,
    payment_id   uuid,
    kind         text        NOT NULL CHECK (kind IN (
        'payment', 'processing', 'refund', 'refund_returned', 'adjustment', 'payout', 'payout_returned')),
    amount       bigint      NOT NULL CHECK (amount <> 0),
    available_on date        NOT NULL,
    payout_id    uuid,
    note         text        NOT NULL DEFAULT '' CHECK (char_length(note) <= 300),
    created_at   timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (booking_id, account_id) REFERENCES bookings (id, account_id) ON DELETE SET NULL (booking_id),
    FOREIGN KEY (payment_id, account_id) REFERENCES payments (id, account_id) ON DELETE SET NULL (payment_id),
    FOREIGN KEY (payout_id, account_id) REFERENCES payouts (id, account_id)
);

-- One payment or refund is entered once per kind, and one payout once. A payout_returned entry
-- stays out of every payout until the next one takes it, so it names its payout in the note.
CREATE UNIQUE INDEX payout_entries_payment_kind_key ON payout_entries (payment_id, kind) WHERE payment_id IS NOT NULL;
CREATE UNIQUE INDEX payout_entries_payout_key ON payout_entries (payout_id) WHERE kind = 'payout';
CREATE INDEX payout_entries_account_id_idx ON payout_entries (account_id, available_on);
CREATE INDEX payout_entries_unpaid_idx ON payout_entries (account_id) WHERE payout_id IS NULL;
CREATE INDEX payout_entries_payout_id_idx ON payout_entries (payout_id) WHERE payout_id IS NOT NULL;

ALTER TABLE emails DROP CONSTRAINT emails_kind_check;
ALTER TABLE emails ADD CONSTRAINT emails_kind_check CHECK (kind IN (
    'welcome', 'confirm_email', 'email_changed', 'new_sign_in', 'deletion_scheduled',
    'account_restored', 'deletion_completed', 'plan_receipt',
    'received', 'confirmed', 'declined', 'reminder', 'prearrival', 'thanks',
    'payout_sent', 'payout_bank_changed'));

-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['payout_profiles', 'payouts', 'payout_entries'] LOOP
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
DELETE FROM emails WHERE kind IN ('payout_sent', 'payout_bank_changed');
ALTER TABLE emails DROP CONSTRAINT emails_kind_check;
ALTER TABLE emails ADD CONSTRAINT emails_kind_check CHECK (kind IN (
    'welcome', 'confirm_email', 'email_changed', 'new_sign_in', 'deletion_scheduled',
    'account_restored', 'deletion_completed', 'plan_receipt',
    'received', 'confirmed', 'declined', 'reminder', 'prearrival', 'thanks'));

UPDATE accounts SET card_mode = 'off' WHERE card_mode = 'payouts';

DROP TABLE payout_entries;
DROP TABLE payouts;
DROP TABLE payout_batches;
DROP TABLE payout_profiles;
