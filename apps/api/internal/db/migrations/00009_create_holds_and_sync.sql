-- Checkout holds on the night ledger, guest access and the payment method on bookings, and the
-- import schedule for iCal feeds.
--
-- A hold is a set of ledger rows with a hold_id instead of a booking or block. It lasts until
-- hold_expires_at; the sweep and the next booking on those units clear it after that.

-- +goose Up
ALTER TABLE unit_nights ADD COLUMN hold_id uuid;
ALTER TABLE unit_nights DROP CONSTRAINT unit_nights_check;
ALTER TABLE unit_nights
    ADD CONSTRAINT unit_nights_check CHECK (num_nonnulls(booking_id, block_id, hold_id) = 1),
    ADD CONSTRAINT unit_nights_hold_expires CHECK (hold_id IS NULL OR hold_expires_at IS NOT NULL);
CREATE INDEX unit_nights_hold_id_idx ON unit_nights (hold_id) WHERE hold_id IS NOT NULL;

-- guest_token_hash lets a guest who booked on the page see their stay; the token itself is only
-- returned once, at booking time. payment_method is how a guest from the page said they'd pay.
ALTER TABLE bookings
    ADD COLUMN guest_token_hash bytea,
    ADD COLUMN payment_method   text CHECK (payment_method IN ('bank', 'property'));

-- When the import job next reads the feed. Null means now.
ALTER TABLE ical_feeds ADD COLUMN next_sync_at timestamptz;
CREATE INDEX ical_feeds_next_sync_at_idx ON ical_feeds (next_sync_at) WHERE url <> '';

-- +goose Down
DROP INDEX ical_feeds_next_sync_at_idx;
ALTER TABLE ical_feeds DROP COLUMN next_sync_at;
ALTER TABLE bookings DROP COLUMN payment_method, DROP COLUMN guest_token_hash;
DROP INDEX unit_nights_hold_id_idx;
DELETE FROM unit_nights WHERE hold_id IS NOT NULL;
ALTER TABLE unit_nights DROP CONSTRAINT unit_nights_hold_expires, DROP CONSTRAINT unit_nights_check;
ALTER TABLE unit_nights ADD CONSTRAINT unit_nights_check CHECK (num_nonnulls(booking_id, block_id) = 1);
ALTER TABLE unit_nights DROP COLUMN hold_id;
