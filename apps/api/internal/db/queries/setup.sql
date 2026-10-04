-- A property's setup collections. Lists load every property in a page at once; writes are
-- scoped by property and account. Collections are replaced as a whole: items whose id is kept
-- are updated, new ones inserted, and the rest removed (units are archived instead).

-- Units ------------------------------------------------------------------------------------

-- name: ListUnits :many
SELECT id, property_id, name, sleeps, beds, rate, weekend_rate, position
FROM units
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[]) AND archived_at IS NULL
ORDER BY property_id, position, created_at;

-- name: ListUnitLinks :many
SELECT parent_unit_id, child_unit_id
FROM unit_links
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[]);

-- name: UnitIDs :many
SELECT id FROM units
WHERE property_id = @property_id AND account_id = @account_id AND archived_at IS NULL;

-- name: InsertUnit :exec
INSERT INTO units (id, account_id, property_id, name, sleeps, beds, rate, weekend_rate, position)
VALUES (@id, @account_id, @property_id, @name, @sleeps, @beds, @rate, sqlc.narg('weekend_rate'), @position);

-- name: UpdateUnit :exec
UPDATE units SET name = @name, sleeps = @sleeps, beds = @beds, rate = @rate,
                 weekend_rate = sqlc.narg('weekend_rate'), position = @position
WHERE id = @id AND property_id = @property_id AND account_id = @account_id;

-- name: ArchiveUnitsNotIn :many
UPDATE units SET archived_at = @now::timestamptz
WHERE property_id = @property_id AND account_id = @account_id AND archived_at IS NULL
  AND NOT (id = ANY (@keep::uuid[]))
RETURNING id;

-- name: DeleteUnitLinks :exec
DELETE FROM unit_links WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertUnitLink :exec
INSERT INTO unit_links (account_id, property_id, parent_unit_id, child_unit_id)
VALUES (@account_id, @property_id, @parent_unit_id, @child_unit_id);

-- Photos -----------------------------------------------------------------------------------

-- name: ListPhotos :many
SELECT id, property_id, key, caption
FROM property_photos
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[])
ORDER BY property_id, position;

-- name: DeletePhotos :exec
DELETE FROM property_photos WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertPhoto :exec
INSERT INTO property_photos (id, account_id, property_id, key, caption, position)
VALUES (@id, @account_id, @property_id, @key, @caption, @position);

-- Payment settings -------------------------------------------------------------------------

-- name: ListPaymentSettings :many
SELECT property_id, bank_enabled, bank_name, account_name, account_number_enc, pay_within_hours,
       cancel_if_unpaid, at_property, cards_status
FROM payment_settings
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[]);

-- name: UpsertPaymentSettings :exec
INSERT INTO payment_settings (property_id, account_id, bank_enabled, bank_name, account_name,
                              account_number_enc, pay_within_hours, cancel_if_unpaid, at_property,
                              cards_status)
VALUES (@property_id, @account_id, @bank_enabled, @bank_name, @account_name,
        sqlc.narg('account_number_enc'), @pay_within_hours, @cancel_if_unpaid, @at_property,
        @cards_status)
ON CONFLICT (property_id) DO UPDATE SET
    bank_enabled       = excluded.bank_enabled,
    bank_name          = excluded.bank_name,
    account_name       = excluded.account_name,
    account_number_enc = excluded.account_number_enc,
    pay_within_hours   = excluded.pay_within_hours,
    cancel_if_unpaid   = excluded.cancel_if_unpaid,
    at_property        = excluded.at_property,
    cards_status       = excluded.cards_status
WHERE payment_settings.account_id = excluded.account_id;

-- Seasons and length discounts -------------------------------------------------------------

-- name: ListSeasons :many
SELECT id, property_id, name, start_md, end_md, min_nights
FROM seasons
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[])
ORDER BY property_id, position;

-- name: ListSeasonPrices :many
SELECT season_id, unit_id, price
FROM season_prices
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[]);

-- name: SeasonIDs :many
SELECT id FROM seasons WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertSeason :exec
INSERT INTO seasons (id, account_id, property_id, name, start_md, end_md, min_nights, position)
VALUES (@id, @account_id, @property_id, @name, @start_md, @end_md, sqlc.narg('min_nights'), @position);

-- name: UpdateSeason :exec
UPDATE seasons SET name = @name, start_md = @start_md, end_md = @end_md,
                   min_nights = sqlc.narg('min_nights'), position = @position
WHERE id = @id AND property_id = @property_id AND account_id = @account_id;

-- name: DeleteSeasonsNotIn :exec
DELETE FROM seasons
WHERE property_id = @property_id AND account_id = @account_id AND NOT (id = ANY (@keep::uuid[]));

-- name: DeleteSeasonPrices :exec
DELETE FROM season_prices WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertSeasonPrice :exec
INSERT INTO season_prices (account_id, property_id, season_id, unit_id, price)
VALUES (@account_id, @property_id, @season_id, @unit_id, @price);

-- name: ListLengthDiscounts :many
SELECT property_id, nights, percent
FROM length_discounts
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[])
ORDER BY property_id, nights;

-- name: DeleteLengthDiscounts :exec
DELETE FROM length_discounts WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertLengthDiscount :exec
INSERT INTO length_discounts (account_id, property_id, nights, percent)
VALUES (@account_id, @property_id, @nights, @percent);

-- Charges and extras -----------------------------------------------------------------------

-- name: ListCharges :many
SELECT id, property_id, name, kind, amount, per, enabled, note
FROM charges
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[])
ORDER BY property_id, position;

-- name: ChargeIDs :many
SELECT id FROM charges WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertCharge :exec
INSERT INTO charges (id, account_id, property_id, name, kind, amount, per, enabled, note, position)
VALUES (@id, @account_id, @property_id, @name, @kind, @amount, @per, @enabled, @note, @position);

-- name: UpdateCharge :exec
UPDATE charges SET name = @name, kind = @kind, amount = @amount, per = @per, enabled = @enabled,
                   note = @note, position = @position
WHERE id = @id AND property_id = @property_id AND account_id = @account_id;

-- name: DeleteChargesNotIn :exec
DELETE FROM charges
WHERE property_id = @property_id AND account_id = @account_id AND NOT (id = ANY (@keep::uuid[]));

-- name: ListExtras :many
SELECT id, property_id, name, price, per, on_request, enabled
FROM extras
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[])
ORDER BY property_id, position;

-- name: ExtraIDs :many
SELECT id FROM extras WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertExtra :exec
INSERT INTO extras (id, account_id, property_id, name, price, per, on_request, enabled, position)
VALUES (@id, @account_id, @property_id, @name, @price, @per, @on_request, @enabled, @position);

-- name: UpdateExtra :exec
UPDATE extras SET name = @name, price = @price, per = @per, on_request = @on_request,
                  enabled = @enabled, position = @position
WHERE id = @id AND property_id = @property_id AND account_id = @account_id;

-- name: DeleteExtrasNotIn :exec
DELETE FROM extras
WHERE property_id = @property_id AND account_id = @account_id AND NOT (id = ANY (@keep::uuid[]));

-- Promo codes ------------------------------------------------------------------------------

-- name: ListPromos :many
SELECT id, property_id, code, kind, amount, starts_on, ends_on, usage_limit, used_count,
       min_nights, note
FROM promos
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[])
ORDER BY property_id, position;

-- name: PromoIDs :many
SELECT id FROM promos WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertPromo :exec
INSERT INTO promos (id, account_id, property_id, code, kind, amount, starts_on, ends_on,
                    usage_limit, min_nights, note, position)
VALUES (@id, @account_id, @property_id, @code, @kind, @amount, sqlc.narg('starts_on'),
        sqlc.narg('ends_on'), sqlc.narg('usage_limit'), sqlc.narg('min_nights'), @note, @position);

-- name: UpdatePromo :exec
-- The used count belongs to bookings and is never written here.
UPDATE promos SET code = @code, kind = @kind, amount = @amount, starts_on = sqlc.narg('starts_on'),
                  ends_on = sqlc.narg('ends_on'), usage_limit = sqlc.narg('usage_limit'),
                  min_nights = sqlc.narg('min_nights'), note = @note, position = @position
WHERE id = @id AND property_id = @property_id AND account_id = @account_id;

-- name: DeletePromosNotIn :exec
DELETE FROM promos
WHERE property_id = @property_id AND account_id = @account_id AND NOT (id = ANY (@keep::uuid[]));

-- Imported calendars -----------------------------------------------------------------------

-- name: ListIcalFeeds :many
SELECT id, property_id, channel, url, status, last_synced_at, last_error, upcoming
FROM ical_feeds
WHERE account_id = @account_id AND property_id = ANY (@property_ids::uuid[])
ORDER BY property_id, position;

-- name: IcalFeedIDs :many
SELECT id FROM ical_feeds WHERE property_id = @property_id AND account_id = @account_id;

-- name: InsertIcalFeed :exec
INSERT INTO ical_feeds (id, account_id, property_id, channel, url, position)
VALUES (@id, @account_id, @property_id, @channel, @url, @position);

-- name: UpdateIcalFeed :exec
-- A new link starts over: pending until the import worker reads it.
UPDATE ical_feeds SET
    channel              = @channel,
    position             = @position,
    status               = CASE WHEN url = @url THEN status ELSE 'pending' END,
    last_error           = CASE WHEN url = @url THEN last_error END,
    consecutive_failures = CASE WHEN url = @url THEN consecutive_failures ELSE 0 END,
    etag                 = CASE WHEN url = @url THEN etag END,
    url                  = @url
WHERE id = @id AND property_id = @property_id AND account_id = @account_id;

-- name: DeleteIcalFeedsNotIn :exec
DELETE FROM ical_feeds
WHERE property_id = @property_id AND account_id = @account_id AND NOT (id = ANY (@keep::uuid[]));

-- name: DeleteSeasonPricesForUnits :exec
DELETE FROM season_prices
WHERE property_id = @property_id AND account_id = @account_id AND unit_id = ANY (@unit_ids::uuid[]);
