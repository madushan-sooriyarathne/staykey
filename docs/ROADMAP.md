# StayKey roadmap to launch

As of 18 Oct 2026. Phases 1 to 5 are done; phases 6 Billing and hardening, 7 Gaps noted in the handover and 8 Email through AWS SES are built, with their gates waiting for sandbox accounts, the AWS account and the live domain. Sign-in codes reach Sri Lankan numbers through oZone Sender. Phases 9 to 12 are next, in order: guest card payments through Payments.lk, the marketing site, error tracking, then end-to-end and load tests. This file lists everything left before StayKey goes live. Tick items as they land, and keep it in step with `docs/HANDOVER.md` and `docs/backend-plan.md`, which hold the detail for each phase.

The shortest route to a first paying owner is now the accounts in section 2: Payments.lk, RevenueCat and the stores, then hosting.

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
- [ ] Email sender (moved to phase 8, AWS SES)
- [x] App: Team, Join, Activity, Templates, Notifications and Profile on the API; the on-device store goes away (Insights already reads stays from the API)
- [x] **Gate:** a caretaker joins by link and alerts arrive (checked in a browser and against Expo's push service; a push on a real phone waits for the EAS project below)

### Phase 5 Calendar sync and booking page (done, see `docs/phase-5-plan.md`)

- [x] iCal import worker (every 10 minutes per feed) with backoff, ETags and real feed status
- [x] iCal export at `GET /ical/{slug}-{token}.ics`, per unit with `?unit=`
- [x] Public API: property, availability, quote, holds, bookings, slip upload
- [x] Booking page and widget (`apps/booking`, `packages/widget`) on live data, with guest checkout and holds
- [ ] Guest emails, confirmation and payment reminders (moved to phase 8, AWS SES)
- [x] **Gate:** an Airbnb stay closes the booking page within 15 minutes (checked locally against a served feed; a real Airbnb link waits for hosting)

### Phase 6 Billing and hardening (built, see `docs/phase-6-plan.md`)

- [x] RevenueCat webhooks, subscription state, `GET /subscription`
- [x] Payments.lk web purchases (prepaid month or year, in LKR) on `staykey.direct/billing`, recorded in RevenueCat too once its External Purchases beta is on
- [x] Free period rules: free until the first direct booking, capped at 60 days, then 7 days' grace
- [x] Lapsed subscription turns the booking page into "message the owner on WhatsApp"
- [x] Postgres row-level security as a second wall behind tenant scoping
- [x] Rate limits on public and owner routes (holds and guest bookings included, with the booking app forwarding the guest's address), backups with a tested restore (`bun run db:backup-check`)
- [x] App: paywall on real purchases, Subscription screen, prototype tools in development builds only
- [ ] **Gate:** a real purchase ends the free period (checked locally with a stand-in Payments.lk; a StoreKit or Play sandbox purchase and a Payments.lk sandbox payment wait for the accounts below)

### Phase 7 Gaps noted in the handover (built, see `docs/phase-7-plan.md`)

- [x] Account deletion (30 days, undone by signing in again) and a data export for owners, as Sri Lanka's Personal Data Protection Act expects; guests' details removable on request
- [x] Refund method picker on the cancel screen
- [x] Account rename
- [x] Profile photo uploaded to the server instead of kept on the phone
- [ ] Invite links as universal links and Android app links: the association files, `app.json` and the link rewrite are in; checking on a device waits for `staykey.direct` and a development build (needs `STAYKEY_APPLE_TEAM_ID` and `STAYKEY_ANDROID_CERT_SHA256`)
- [x] **Gate:** an owner renames the account, changes their photo and sees it from another session, and deletes a test account with nothing of it left in the database or the files after the 30 days (checked locally with the clock moved on)

### Phase 8 Email through AWS SES (built, see `docs/phase-8-plan.md`)

- [x] SES v2 sender in `internal/email`, with a log sender until SES is set up (`STAYKEY_EMAIL_PROVIDER`)
- [ ] AWS account: the sending domain verified (SPF, DKIM, DMARC), SES out of the sandbox, and the SNS topic subscribed to `/webhooks/ses`
- [x] Bounces and complaints from SES (through SNS) stop sending to that address
- [x] Account emails: welcome with email confirmation, email changed, new sign-in, deletion scheduled, restored and completed, web plan receipts (sign-in is by phone code, so there are no password emails)
- [x] Email confirmation: links from Profile and onboarding, confirmed on the marketing site; nothing else goes to an unconfirmed address
- [x] Guest emails: booking received, confirmed and declined, payment reminder before the deadline, pre-arrival and thank-you, built from the account's message templates, with a per-template email switch
- [x] Email as an alert channel for the team, beside push and WhatsApp, morning summary included
- [ ] **Gate:** a guest who books on the page gets the confirmation email, and a bounced address is not emailed again (done with the log sender and a test SNS certificate; waits for SES sandbox credentials)

### Phase 9 Guest card payments through Payments.lk (built, see `docs/phase-9-plan.md`)

- [x] Decide how owners are paid: both, chosen per account. Their own Payments.lk account (no StayKey charge), or StayKey collects and pays out every two weeks less a 3% processing charge
- [x] Card payments per property (`cards_status`), on only once the account's card mode is ready (a connected key, or a payout profile) and non-LKR properties have a rupee rate
- [x] Booking page checkout by card for the deposit, the full amount or a later balance, through a Payments.lk hosted checkout
- [x] Payments.lk events, polled each minute, record the payment on the booking and confirm it, once per event
- [x] Refunds by card through Payments.lk's refunds API from the cancel screen
- [x] StayKey payouts: a ledger per account, a payout every other Monday for money available after check-in, `cmd/payouts` for the bank batch, and payout statements in the app
- [ ] Legal advice and Payable's approval before `STAYKEY_PAYOUTS_ENABLED` is turned on in production
- [ ] **Gate:** a sandbox card payment on the booking page confirms the stay, and a cancellation refunds it

### Phase 10 Marketing site (built, see `docs/phase-10-plan.md`)

- [x] Pricing from `GET /v1/public/plans`, features, how it works, and links to the App Store and Google Play ("Coming soon" until the listings exist)
- [x] Support contact, and terms of service and privacy policy pages (drafts until a lawyer reviews them)
- [x] Account deletion page and payout terms page, linked from the app with terms and privacy (store review needs them)
- [x] Metadata, sitemap and social images for search and sharing
- [ ] **Gate:** a visitor finds the price, installs the app and reaches support from the site

### Phase 11 Error tracking (built, see `docs/phase-11-plan.md`)

- [x] Error tracking in the API, booking app, marketing site and owner app (Sentry by default), with releases and no personal data in reports. Production only, to keep the free quota
- [x] Uptime monitoring for the API, booking pages and marketing site, with alerts to the owner of StayKey (health routes, job heartbeats, and the monitors in `infra/better-stack.md`)
- [x] Log retention and search for the API (JSON logs with request ids; `infra/log-shipper` to Grafana Cloud, `docs/RUNBOOK.md`)
- [ ] **Gate:** an error thrown on purpose in each surface shows up with its release, and stopping the API raises an alert (checked locally with a stand-in Sentry; runs on the first production deploy)

### Phase 12 End-to-end and load tests (next, see `docs/phase-12-plan.md`)

- [ ] Playwright on web against a seeded API: booking page, checkout, stay page and slip, widget, web billing; run in CI
- [ ] Maestro flows on iOS and Android: sign-in, onboarding, new booking, approve a request, record a payment
- [ ] Load test booking creation and the public availability route, with targets and results in the repo
- [ ] **Gate:** both suites pass in CI, and the load test meets its targets on the production setup

## 2. Accounts and decisions (owner)

- [x] SMS provider for sign-in codes: oZone Sender for Sri Lankan numbers. Production won't start without one
- [ ] International SMS gateway (international numbers are refused until then)
- [ ] Hosting: default is Fly.io in Singapore with managed Postgres 17 alongside (the river migration fails on 16)
- [ ] Cloudflare R2 bucket and media domain (`media.staykey.direct`); test uploads against the real bucket
- [ ] WhatsApp Business number for alerts, with an approved alert template
- [ ] EAS project (`bunx eas-cli init`) so the app can get Expo push tokens; needed for the phase 4 gate
- [ ] Payments.lk merchant account with sandbox keys (`sk_test_...`) and a webhook endpoint pointing at `/webhooks/payments-lk`
- [ ] Access to RevenueCat's External Purchases API (private beta), for web purchases
- [ ] RevenueCat project linked to App Store Connect and Google Play subscriptions, with products `staykey_<villa|guesthouse|collection>_<monthly|yearly>`, one `staykey` entitlement, and a webhook to `/webhooks/revenuecat`
- [ ] Apple Developer and Google Play Console accounts
- [ ] DNS for `staykey.direct` with wildcard subdomains for booking pages, plus `api`, `cdn` and `app`
- [ ] AWS account with SES: the sending domain verified and production access (out of the sandbox), for phase 8
- [ ] Sentry, Better Stack and Grafana Cloud accounts (free plans), with the DSNs, ops secret and heartbeat URLs set on the first deploy (`docs/RUNBOOK.md`)
- [x] GitHub repository
- [ ] CI green on `main` (`.github/workflows/ci.yml`)

## 3. Launch readiness

- [ ] Sign in on a real phone (the open phase 1 check; possible now with `STAYKEY_SMS_PROVIDER=ozone`)
- [ ] Production secrets set: `STAYKEY_AUTH_SECRET`, `STAYKEY_DATA_KEY`, `STAYKEY_SMS_PROVIDER` with the `STAYKEY_OZONE_*` credentials, the R2 keys. Keep a safe copy of the data key, since losing it makes stored bank details unreadable
- [ ] EAS builds for iOS and Android, store listings, screenshots, app privacy details
- [ ] A reviewer test account with a working sign-in for Apple and Google review
- [ ] Terms of service, privacy policy and payout terms, reviewed against Sri Lanka's Personal Data Protection Act; then set `LEGAL_REVIEWED` in `apps/marketing/src/lib/company.ts`
- [ ] Support channel and a plan for onboarding the first owners (support@staykey.direct and WhatsApp +94 71 688 0657 are on the site and in the app)
- [ ] App Store and Google Play listing links on the site (`STAYKEY_APP_STORE_URL`, `STAYKEY_PLAY_STORE_URL`, `STAYKEY_APP_STORE_ID`), with the stores' official badges
- [ ] The app's real icon and splash screen (both are still Expo's placeholders)

## After launch (not needed to go live)

- Next.js owner dashboard (V1.1)
- Channel manager, multi-currency settlement
