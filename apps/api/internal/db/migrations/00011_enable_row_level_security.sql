-- Row-level security as a second wall behind the tenant scoping in every query. The API sets
-- staykey.account_id on each connection it hands to a request that carries a tenant, and clears
-- it otherwise. With the setting empty (sign-in, public routes, webhooks, jobs, the seed) the
-- policy lets everything through, so the wall only narrows tenant requests to their account.
--
-- FORCE applies the policies to the tables' owner too. Superusers and roles with BYPASSRLS skip
-- them, so production must connect as an ordinary role.

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'activity', 'activity_reads', 'billing_events', 'block_units', 'blocks', 'booking_events',
        'booking_extras', 'booking_lines', 'bookings', 'charges', 'extras', 'guests', 'ical_feeds',
        'idempotency_keys', 'invites', 'length_discounts', 'membership_properties', 'memberships',
        'message_templates', 'notification_prefs', 'payment_settings', 'payment_slips', 'payments',
        'promos', 'properties', 'property_photos', 'rate_overrides', 'season_prices', 'seasons',
        'subscriptions', 'unit_links', 'unit_nights', 'units', 'web_checkouts'
    ] LOOP
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
-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'activity', 'activity_reads', 'billing_events', 'block_units', 'blocks', 'booking_events',
        'booking_extras', 'booking_lines', 'bookings', 'charges', 'extras', 'guests', 'ical_feeds',
        'idempotency_keys', 'invites', 'length_discounts', 'membership_properties', 'memberships',
        'message_templates', 'notification_prefs', 'payment_settings', 'payment_slips', 'payments',
        'promos', 'properties', 'property_photos', 'rate_overrides', 'season_prices', 'seasons',
        'subscriptions', 'unit_links', 'unit_nights', 'units', 'web_checkouts'
    ] LOOP
        EXECUTE format('DROP POLICY tenant_wall ON %I', t);
        EXECUTE format('ALTER TABLE %I NO FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
    END LOOP;
END;
$$;
-- +goose StatementEnd
