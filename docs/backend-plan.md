# StayKey backend and data plan

As of 4 Oct 2026. Live, editable copy: https://claude.ai/code/artifact/5f501309-733d-4bcd-ab38-da01f0ab768d

Status: phase 1 Foundations is done (see `docs/HANDOVER.md`); phase 2 is next.

## Goal and scope

Turn the app's on-device data store into a Postgres-backed Go API in six phases, ending with the booking page and subscriptions running on the same data. The app screens stay as they are. Only the data hooks underneath them change.

In scope:

- The full V1 schema: accounts, people, properties, units, pricing, bookings, payments, calendar sync, activity and billing.
- Owner auth with phone OTP, team roles and tenant scoping on every query.
- An OpenAPI-first REST API for the owner app, a public API for the booking page and widget, and webhooks.
- Background jobs for request expiry, unpaid cancellations, iCal sync and notifications.
- Moving every app screen from `apps/mobile/src/data/store.ts` to the API.

Out of scope for now: the Next.js dashboard (V1.1), a channel manager, card payments beyond the PayHere connection request, and multi-currency settlement.

## Key decisions

| Area | Decision | Why |
| --- | --- | --- |
| Tenancy | Every business table carries `account_id`; every query filters by it through a request-scoped `Tenant`. Postgres row-level security added in phase 6 as a second wall. | One owner can never read another's data, even from a missed filter. |
| Identity | A `user` (one per phone number) joins an `account` through a `membership` with a role: owner, manager or caretaker. | The same caretaker can work for two owners. |
| Auth | Phone OTP, 6 digits, 5 minute expiry, 5 attempts. Short-lived access token (15 min, signed JWT) plus a rotating refresh token stored hashed. | Matches onboarding. Refresh rotation limits a stolen token. |
| IDs | UUIDv7 primary keys generated in Go. Human booking refs (`KV-2041`) unique per account. | Time-ordered keys index well. |
| Money | `bigint` minor units plus a `currency` char(3) on every money row. | Already the rule in the repo. |
| Dates | Stays use `date`. Events use `timestamptz`. Each property stores its IANA time zone, default `Asia/Colombo`. | No stay drifts across time zones. |
| Availability | A night inventory ledger, `unit_nights`, one row per unit per night held. Unique key on (unit_id, night). | Double booking is impossible inside one transaction, including linked whole-house units. |
| Price snapshot | A booking stores its price lines at the moment it is made. | Guests pay what they were quoted. |
| Query layer | `sqlc` over pgx for typed queries; goose stays for migrations. | Compile-time checked SQL, no ORM. |
| Jobs | `river` (Postgres-backed queue). | No extra infrastructure; jobs commit with their data. |
| Files | Cloudflare R2 with presigned uploads from the app; the API stores keys, not files. | Photos and bank slips never pass through the API. |
| Pricing logic | Go is the source of truth. The app keeps `pricing.ts` for instant previews, held in line by shared test fixtures. | Fast UI, one correct total. |
| App state | TanStack Query for server data, persisted for offline reads; zustand stays for UI and session. | Caching, retries and optimistic updates. |

## Database schema

38 tables in seven groups, mapped from `apps/mobile/src/data/types.ts`. All tables have `id uuid` (v7), `created_at`, `updated_at`; business tables also carry `account_id` with an index leading on it.

### Accounts and people

| Table | Key columns | Notes |
| --- | --- | --- |
| accounts | name, trial_started_at, first_direct_booking_at, status | The tenant. |
| users | phone (unique, E.164), name, email, language, photo_key | One per person, across accounts. |
| memberships | account_id, user_id, role, status | owner, manager, caretaker. Unique (account_id, user_id). |
| membership_properties | membership_id, property_id | Empty means every property. |
| invites | account_id, phone, role, property_ids, token_hash, expires_at, accepted_at | WhatsApp invite links. |
| otp_challenges | phone, code_hash, attempts, expires_at, consumed_at | Rate limited per phone and IP. |
| sessions | user_id, refresh_hash, device_name, last_used_at, expires_at, revoked_at | One per device. |
| push_tokens | user_id, token, platform | Expo push tokens. |
| notification_prefs | user_id, account_id, event, push, whatsapp | Unique (user_id, account_id, event). |

### Properties and setup

| Table | Key columns | Notes |
| --- | --- | --- |
| properties | slug (unique), name, booking_type, description, location, lat, lng, time_zone, currency, check_in_time, check_out_time, amenities text[], policy, deposit_percent, balance_due_days, house_rules text[], booking_mode, reply_hours, hold_minutes, display_currencies text[], brand_color, logo_key, ical_export_token, status, min_nights, max_nights, same_day_cutoff, window_months, closed_arrival smallint[] | Extends today's table. |
| property_photos | property_id, key, caption, position | Cover = position 0. |
| units | property_id, name, sleeps, beds, rate, weekend_rate, position, archived_at | Archived, never deleted, once booked. |
| unit_links | parent_unit_id, child_unit_id | Whole-house unit and its rooms. |
| payment_settings | property_id (PK), bank_enabled, bank_name, account_name, account_number_enc, pay_within_hours, cancel_if_unpaid, at_property, cards_status | Account number encrypted at rest. |

### Pricing

| Table | Key columns | Notes |
| --- | --- | --- |
| seasons | property_id, name, start_md, end_md, min_nights | "MM-DD", may wrap the new year. |
| season_prices | season_id, unit_id, price | Unique (season_id, unit_id). |
| rate_overrides | unit_id, night, price, min_nights, closed_to_arrival | PK (unit_id, night). |
| length_discounts | property_id, nights, percent | Unique (property_id, nights). |
| charges | property_id, name, kind, amount, per, enabled, position | Service charge, VAT, cleaning fee. |
| extras | property_id, name, price, per, on_request, enabled, position | |
| promos | property_id, code, kind, amount, starts_on, ends_on, usage_limit, used_count, min_nights | Unique (property_id, upper(code)). |

### Guests, bookings and payments

| Table | Key columns | Notes |
| --- | --- | --- |
| guests | account_id, name, phone, email, country | Matched by phone or email per account. |
| bookings | property_id, unit_id, ref, source, status, guest_id, adults, children, check_in, check_out, currency, total, guest_note, owner_note, request_expires_at, hold_expires_at, payment_due_at, external_uid, feed_id, promo_id, version | Unique (account_id, ref), (feed_id, external_uid). |
| booking_lines | booking_id, label, amount, kind, position | Frozen price breakdown. |
| booking_extras | booking_id, extra_id, name, amount | Copied at booking time. |
| payments | booking_id, kind (payment or refund), method, amount, received_at, recorded_by, slip_id, provider_ref | Balance computed in SQL. |
| payment_slips | booking_id, file_key, amount, status, uploaded_at, reviewed_by, reviewed_at | |
| booking_events | booking_id, from_status, to_status, actor_user_id, reason, created_at | Audit trail. |

Booking statuses (same as the app): requested, awaiting_payment, confirmed, checked_in, checked_out, cancelled, declined. Sources: page, widget, whatsapp, phone, walkin, other, airbnb, booking, agoda, expedia.

### Calendar

| Table | Key columns | Notes |
| --- | --- | --- |
| blocks | property_id, starts_on, ends_on, reason, note, created_by | Ends exclusive. |
| block_units | block_id, unit_id | |
| unit_nights | unit_id, night, booking_id, block_id, hold_expires_at | PK (unit_id, night). Exactly one of booking_id or block_id. |
| ical_feeds | property_id, unit_id, channel, url, status, last_synced_at, last_error, etag, consecutive_failures | |

### Messaging and activity

| Table | Key columns | Notes |
| --- | --- | --- |
| message_templates | account_id, key, name, channels text[], timing, body | Four defaults seeded per account. |
| activity | account_id, property_id, kind, title, subtitle, booking_id, created_at | |
| activity_reads | user_id, account_id, last_read_at | |

### Billing and plumbing

| Table | Key columns | Notes |
| --- | --- | --- |
| subscriptions | account_id, plan, period, store, status, current_period_end, revenuecat_app_user_id | |
| billing_events | account_id, source, type, payload jsonb, received_at | Raw webhooks. |
| idempotency_keys | account_id, key, request_hash, response jsonb, expires_at | |

### How the ledger blocks double bookings

Creating a booking or block runs in one transaction that inserts a `unit_nights` row for every night on the unit and on every related unit (its linked rooms if it is a whole-house unit, or the whole-house unit if it is a room). A clash fails the unique key and the API returns 409 with the conflicting stay. Cancelling or declining deletes the rows. A checkout hold inserts rows with `hold_expires_at`; a job clears expired holds every minute.

## API surface

REST under `/v1`, spec-first in `packages/api-spec/openapi.yaml`, Go handlers (oapi-codegen strict) and the TS client generated from it. Owner routes take a bearer token and `X-Account-Id`. Lists use cursor pagination and `updated_since`; money and booking writes take `Idempotency-Key`; errors keep `{code, message, field}`; a stale booking `version` returns 409 `stale`.

| Group | Endpoints | Used by |
| --- | --- | --- |
| Auth | `POST /auth/otp`, `POST /auth/verify`, `POST /auth/refresh`, `POST /auth/logout`, `GET/PATCH/DELETE /me` | Onboarding phone steps, Profile |
| Accounts and team | `GET /accounts`, `GET/POST /team`, `PATCH/DELETE /team/{id}`, `POST /invites/{token}/accept` | Team, Join |
| Onboarding | `POST /onboarding/publish` (property, units, prices, payment settings, feeds in one transaction) | You're live |
| Properties | `GET/POST /properties`, `GET/PATCH /properties/{id}`, sub-resources `/units`, `/photos` (presign, confirm, reorder), `/seasons`, `/rate-overrides`, `/discounts`, `/charges`, `/extras`, `/promos`, `/payment-settings`, `/ical-feeds` (+ `/sync`) | Properties tab, 14 settings screens |
| Calendar | `GET /properties/{id}/calendar?from&to`, `POST /blocks`, `DELETE /blocks/{id}`, `PUT /rate-overrides` | Calendar, Block dates, Edit rates |
| Bookings | `GET /bookings`, `POST /bookings`, `GET/PATCH /bookings/{id}`, `POST /bookings/{id}/transitions`, `POST /bookings/{id}/cancel`, `POST /quote` | Today, Bookings, detail, forms |
| Payments | `POST /bookings/{id}/payments`, `POST /slips/{id}/accept`, `POST /slips/{id}/reject` | Record payment |
| Activity and insights | `GET /activity`, `POST /activity/read`, `GET /insights?period&property` | Activity, Insights |
| Messaging | `GET/PATCH /templates/{key}`, `GET/PUT /notification-prefs`, `POST /push-tokens` | Templates, Notifications |
| Billing | `GET /subscription` | Subscription, paywall |
| Public | `GET /public/properties/{slug}`, `/availability`, `POST /public/quote`, `POST /public/holds`, `POST /public/bookings`, `POST /public/bookings/{ref}/slip` | Booking page, widget |
| iCal export | `GET /ical/{slug}-{token}.ics` | OTAs |
| Webhooks | `POST /webhooks/revenuecat`, `POST /webhooks/payhere` | Billing |

## Backend structure

One Go binary with an API mode and a worker mode. Handlers stay thin; rules live in domain packages with no database imports.

| Package | Holds |
| --- | --- |
| `internal/auth` | OTP, tokens, refresh rotation, request `Principal` |
| `internal/tenant` | Resolves `X-Account-Id` against memberships |
| `internal/domain/pricing` | Port of `pricing.ts`, checked against shared fixtures |
| `internal/domain/availability` | Related units, ledger rows, conflicts |
| `internal/domain/booking` | State machine per status and role |
| `internal/store` | sqlc queries and transaction helpers; only package that touches SQL |
| `internal/server` | oapi-codegen strict handlers |
| `internal/jobs` | river workers |
| `internal/notify` | Expo push, WhatsApp Cloud API, email, log sender for dev |
| `internal/files` | R2 presign and confirm |
| `internal/ical` | Feed fetch and parse, export writer |

Jobs: request expiry, unpaid cancellation, hold sweep (every minute), iCal import (every 15 minutes with backoff), notifications fan-out and 7:00 morning summary, reminders from templates, trial check (free period ends 7 days after first direct booking or at 60 days).

## Connecting the app

1. Auth: `src/lib/auth.ts` calls real endpoints; tokens in `expo-secure-store`; openapi-fetch middleware adds token and account header, refreshes once on 401.
2. Query layer: `src/api/` with one file per area exporting query keys, hooks and mutations; cache persisted to AsyncStorage.
3. Optimistic writes for approve, decline, check in, record payment, block dates. Booking create and edit wait for the server.
4. Pricing parity: `packages/api-spec/fixtures/pricing.json` run by both `bun test` and `go test`.
5. Settings: `useSettings` save becomes a `PATCH` to the matching sub-resource.
6. Uploads: presign, upload to R2, confirm.
7. Onboarding publish sends the whole draft to `POST /onboarding/publish`.
8. The on-device seed leaves app builds; `cmd/seed` creates the same demo data server side.
9. Push token registered after sign-in; tapped notifications route to the booking or Activity.

## Phases

| Phase | Backend | App | Gate |
| --- | --- | --- | --- |
| 1 Foundations | Schema migrations, sqlc, OTP auth, accounts, roles, tenant scoping, `cmd/seed` | Real sign-in, secure tokens, TanStack Query layer | Sign in on a phone; tenant isolation tests pass |
| 2 Properties and setup | Properties, units, photos on R2, rates, seasons, rules, policies, charges, extras, promos, payment and booking settings | Onboarding publish, Properties tab, all 14 settings screens | Publish from onboarding and edit every setting |
| 3 Bookings core (critical) | Night ledger, quote, bookings, state machine, payments, slips, cancel, blocks, overrides | Today, Calendar, Bookings, detail, new booking, payment, cancel, sheets | One winner out of 50 parallel bookings |
| 4 Team, activity, messages | Invites, activity, templates, notification prefs, push, WhatsApp | Team, Join, Activity, Templates, Notifications, Insights | A caretaker joins by link; alerts arrive |
| 5 Calendar sync and booking page | iCal import and export, public API, holds, guest checkout | Booking page and widget on live data; real iCal status | An Airbnb stay closes the page within 15 min |
| 6 Billing and hardening | RevenueCat webhooks, free period rules, lapsed page, RLS, rate limits, backups | Paywall on real purchases, Subscription screen, prototype tools dev only | A real purchase ends the free period |

Phases 4 and 5 can run side by side after phase 3. Order inside each phase: migration, sqlc queries, domain rules with tests, OpenAPI routes, handlers, generated client, app hooks.

## Testing and verification

- Domain unit tests in Go plus shared pricing fixtures in both languages.
- Store integration tests against Postgres 17, each in a rolled-back transaction.
- Double booking: 50 goroutines on overlapping nights on a unit and its whole-house parent; exactly one wins.
- Tenant isolation: for every owner route, another account's token gets 404. Generated from the OpenAPI route list.
- Contract: responses validated against `openapi.yaml`.
- Jobs against a fixed clock.
- App end to end: Playwright web walkthrough against a seeded API, then Maestro on iOS and Android.

## Open questions

- [ ] SMS provider for OTP: a Sri Lankan gateway or Twilio?
- [ ] Hosting and region: Fly.io or Railway in Singapore or Mumbai, managed Postgres in the same region?
- [ ] File storage: Cloudflare R2 or S3?
- [ ] WhatsApp: register a WhatsApp Business number now?
- [ ] OTA stays: bookings with an OTA source (as the app does today) or a separate imported-events table?
- [ ] Row-level security in phase 6 or from the first migration?
- [ ] Demo data: production accounts or development builds only?
