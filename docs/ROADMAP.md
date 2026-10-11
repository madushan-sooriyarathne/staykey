# StayKey roadmap to launch

As of 11 Oct 2026. Phases 1 Foundations, 2 Properties and setup, 3 Bookings core and 4 Team, activity and messages are done, and sign-in codes reach Sri Lankan numbers through oZone Sender. Phase 5 is next. This file lists everything left before StayKey goes live. Tick items as they land, and keep it in step with `docs/HANDOVER.md` and `docs/backend-plan.md`, which hold the detail for each phase.

The shortest route to a first paying owner is phase 5 and the billing half of phase 6.

## 1. Build phases

### Phase 3 Bookings core (done)

- [x] Migration: `guests`, `bookings`, `booking_lines`, `booking_extras`, `payments`, `payment_slips`, `booking_events`, `blocks`, `block_units`, `unit_nights`, `rate_overrides`, `idempotency_keys`, all with `account_id` and composite foreign keys
- [x] `internal/domain/pricing`: port of `apps/mobile/src/data/pricing.ts`, held in line by shared fixtures in `packages/api-spec/fixtures/pricing.json` run by both `go test` and `bun test`
- [x] Night ledger: a `unit_nights` row per night for the unit and its related units, so a clash fails the unique key and returns 409 with the conflicting stay
- [x] Booking state machine per status and role, with `booking_events` as the audit trail
- [x] `POST /v1/quote`, bookings (list, create, get, edit, transitions, cancel) with `Idempotency-Key` and a `version` for stale writes
- [x] Payments, bank slips (uploads reuse `internal/files` with a new kind), refunds by cancellation policy
- [x] Blocks and rate overrides, plus `GET /properties/{id}/calendar`
- [x] Jobs on `river`: request expiry, unpaid cancellation, hold sweep, orphaned upload sweep
- [x] App: Today, Calendar, Bookings, booking detail, new and edit booking, record payment, cancel, block and rates sheets move from the on-device store to `src/api/`
- [x] `cmd/seed` creates the sample bookings from `apps/mobile/src/data/seed.ts`
- [x] New owner routes added to `TestTenantIsolation`
- [x] **Gate:** exactly one winner out of 50 parallel bookings on overlapping nights for a unit and its whole-house parent

### Phase 4 Team, activity and messages (done, see `docs/phase-4-plan.md`)

- [x] Invites by WhatsApp link with accept, team list, role and property changes, removal
- [x] Account switcher for people in more than one account
- [x] Activity feed and read state
- [x] Message templates (four defaults per account) and notification preferences
- [x] Expo push tokens, push fan-out and the 7:00 morning summary
- [x] WhatsApp Cloud API sender for team alerts (log sender until the Business number is registered)
- [ ] Email sender, once an email provider is chosen (not needed for the phase 4 gate)
- [x] App: Team, Join, Activity, Templates, Notifications and Profile on the API; the on-device store goes away (Insights already reads stays from the API)
- [x] **Gate:** a caretaker joins by link and alerts arrive (checked in a browser and against Expo's push service; a push on a real phone waits for the EAS project below)

### Phase 5 Calendar sync and booking page

- [ ] iCal import worker every 15 minutes with backoff, ETags and real feed status
- [ ] iCal export at `GET /ical/{slug}-{token}.ics`
- [ ] Public API: property, availability, quote, holds, bookings, slip upload
- [ ] Booking page and widget (`apps/booking`, `packages/widget`) on live data, with guest checkout and holds
- [ ] **Gate:** an Airbnb stay closes the booking page within 15 minutes

### Phase 6 Billing and hardening

- [ ] RevenueCat webhooks, subscription state, `GET /subscription`
- [ ] PayHere web subscriptions and the card payments connection (cards turn on only from a PayHere approval)
- [ ] Free period rules: free until the first direct booking, capped at 60 days, then 7 days' grace
- [ ] Lapsed subscription turns the booking page into "message the owner on WhatsApp"
- [ ] Postgres row-level security as a second wall behind tenant scoping
- [ ] Rate limits on public and owner routes, backups with a tested restore
- [ ] App: paywall on real purchases, Subscription screen, prototype tools in development builds only
- [ ] **Gate:** a real purchase ends the free period

## 2. Accounts and decisions (owner)

- [x] SMS provider for sign-in codes: oZone Sender for Sri Lankan numbers. Production won't start without one
- [ ] International SMS gateway (international numbers are refused until then)
- [ ] Hosting: default is Fly.io in Singapore with managed Postgres 17 alongside (the river migration fails on 16)
- [ ] Cloudflare R2 bucket and media domain (`media.staykey.direct`); test uploads against the real bucket
- [ ] WhatsApp Business number for alerts, with an approved alert template
- [ ] EAS project (`bunx eas-cli init`) so the app can get Expo push tokens; needed for the phase 4 gate
- [ ] PayHere merchant account
- [ ] RevenueCat project linked to App Store Connect and Google Play subscriptions
- [ ] Apple Developer and Google Play Console accounts
- [ ] DNS for `staykey.direct` with wildcard subdomains for booking pages, plus `api`, `cdn` and `app`
- [ ] Email sending domain and provider
- [x] GitHub repository
- [ ] CI green on `main` (`.github/workflows/ci.yml`)

## 3. Launch readiness

- [ ] Sign in on a real phone (the open phase 1 check; possible now with `STAYKEY_SMS_PROVIDER=ozone`)
- [ ] End-to-end tests: Playwright on web against a seeded API, Maestro on iOS and Android
- [ ] Load test booking creation and the public availability route
- [ ] Error tracking, uptime monitoring and log retention
- [ ] Production secrets set: `STAYKEY_AUTH_SECRET`, `STAYKEY_DATA_KEY`, `STAYKEY_SMS_PROVIDER` with the `STAYKEY_OZONE_*` credentials, the R2 keys. Keep a safe copy of the data key, since losing it makes stored bank details unreadable
- [ ] EAS builds for iOS and Android, store listings, screenshots, app privacy details
- [ ] A reviewer test account with a working sign-in for Apple and Google review
- [ ] Terms of service and privacy policy, reviewed against Sri Lanka's Personal Data Protection Act
- [ ] Marketing site: pricing, signup links, support contact
- [ ] Support channel and a plan for onboarding the first owners

## After launch (not needed to go live)

- Next.js owner dashboard (V1.1)
- Channel manager, multi-currency settlement
