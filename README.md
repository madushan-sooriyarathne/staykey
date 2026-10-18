# StayKey

Direct booking platform for small villa and guesthouse owners in Sri Lanka. Owners get a hosted booking page, a website widget and one calendar, set up from the mobile app in about 10 minutes.

## Repository layout

| Path | What it is | Stack |
| --- | --- | --- |
| `apps/api` | Backend API | Go, PostgreSQL, oapi-codegen, goose |
| `apps/mobile` | Owner app for iOS and Android | Expo (React Native), Expo Router |
| `apps/booking` | Hosted booking pages and the iFrame embed | Next.js, Tailwind CSS |
| `apps/marketing` | Marketing site at staykey.direct | Next.js, Tailwind CSS |
| `packages/api-spec` | OpenAPI contract, the single source of truth for the API | OpenAPI 3.0 |
| `packages/api-client` | Typed TypeScript client generated from the contract | openapi-typescript, openapi-fetch |
| `packages/tokens` | Brand tokens for React Native and Tailwind | TypeScript, CSS |
| `packages/widget` | Embed loader served at cdn.staykey.direct/widget.js | TypeScript, `bun build` |
| `packages/config` | Shared TypeScript configs | |

The owner dashboard (`apps/dashboard`, Next.js) arrives in V1.1.

## Domains

| Host | Serves |
| --- | --- |
| `staykey.direct` | Marketing site |
| `<slug>.staykey.direct` | A property's booking page, for example `kingfisher.staykey.direct` |
| `<slug>.staykey.direct/embed` | Widget content loaded in the owner's iFrame |
| `api.staykey.direct` | Go API |
| `cdn.staykey.direct/widget.js` | Embed loader |
| `app.staykey.direct` | Owner dashboard (V1.1) |

Slugs such as `www`, `api`, `app` and `cdn` are reserved. The list lives in `apps/api/internal/domain/slug.go` and is mirrored in `apps/booking/src/lib/tenant.ts`.

## Prerequisites

- Bun 1.4 or newer (package manager and script runner)
- Node.js 22 LTS (Expo's Metro bundler and the Next.js runtime run on Node)
- Go 1.27 or newer (the toolchain directive downloads it automatically)
- Docker, for the local PostgreSQL
- Xcode or Android Studio for running the mobile app on a simulator

## Getting started

```sh
bun run setup        # once: install, start PostgreSQL in Docker, migrate, seed the sample account
bun run dev          # API on :8080, booking on :3001, marketing on :3000, Expo on :8081
```

The individual steps are `bun install`, `bun run db:up`, `bun run db:migrate` and `bun run db:seed`.

The seed creates an owner (+94 77 000 0001, or `-owner-phone` for your own number) with Kingfisher Villa and Coral Bay House, each fully set up with units, seasons, length discounts, charges, extras, promo codes, bank details and iCal links, plus a manager (+94 77 222 3344), a caretaker for the villa (+94 71 333 8899) and an open invite for +94 71 555 0199, whose link it prints. Open a booking page at http://kingfisher.localhost:3001.

### Signing in

Owners sign in with a 6-digit code sent to their phone. Codes reach Sri Lankan numbers through oZone Sender when `STAYKEY_SMS_PROVIDER=ozone` and its credentials are set (see `apps/api/.env.example`); other countries are not supported yet. In development the default `log` provider writes the code to the API log, the API returns it, and development builds of the app show it under the code boxes. To sign in on a phone, set `EXPO_PUBLIC_API_URL` in `apps/mobile/.env` to your computer's LAN address, tap "I have an account" and use one of the seeded numbers.

From the command line:

```sh
curl -s -X POST localhost:8080/v1/auth/otp -H 'content-type: application/json' \
  -d '{"phone":"+94770000001"}'                  # returns devCode in development
curl -s -X POST localhost:8080/v1/auth/verify -H 'content-type: application/json' \
  -d '{"phone":"+94770000001","code":"<devCode>"}' # returns tokens and accounts
curl -s localhost:8080/v1/properties \
  -H "Authorization: Bearer <accessToken>" -H "X-Account-Id: <account id>"
curl -s -X PATCH localhost:8080/v1/properties/<id> -H 'content-type: application/json' \
  -H "Authorization: Bearer <accessToken>" -H "X-Account-Id: <account id>" \
  -d '{"rules":{"minNights":2,"maxNights":30,"sameDayCutoff":null,"windowMonths":12,"closedArrival":[]}}'
```

Owner routes need the access token (15 minutes, renewed with `POST /v1/auth/refresh`) and an `X-Account-Id` the caller is a member of. Anyone else gets 404.

`GET /v1/properties/{id}` returns the property's whole setup. `PATCH` takes any of its sections and replaces only those, in one transaction; a collection such as units or promos is sent whole, with ids for the items it keeps.

### Team, activity and alerts

Owners invite managers and caretakers from Team. `POST /v1/team/invites` returns a link (`https://staykey.direct/join/<token>`, a marketing page that opens `staykey://join?token=<token>`) and the app opens WhatsApp with it. The invited number signs in and the invite is accepted on the server; any other number gets 404.

Every change to a stay writes a line of activity on the server. A job sends each new line as an Expo push notification to the team members who asked for that event and can see the property, never to the person who made the change, and sends each person's morning summary at 7:00 in the property's time zone. Push needs an EAS project id in `apps/mobile/app.json` (`bunx eas-cli init`) and a development build. WhatsApp alerts go to the API log until a WhatsApp Business number is set up (`STAYKEY_WHATSAPP_*` in `apps/api/.env.example`).

### Email

Email goes through Amazon SES. Guests get their booking emails (received, confirmed, declined, payment reminder, pre-arrival and thank-you) from the account's message templates, people get account emails (welcome, email changes, new sign-ins, deletion and restore, web plan receipts) once their address is confirmed, and team members can turn on Email in Notifications for alerts and the morning summary. A bounce or complaint reported through SNS to `/webhooks/ses` stops all email to that address.

In development the default `log` provider sends nothing: the API log records each email with the address masked and its link, never the subject or body. Add an email in Profile, wait for the next minute's send, and open the logged `http://localhost:3000/verify-email/<token>` link with the marketing site running to confirm it. To try SES, set `STAYKEY_EMAIL_PROVIDER=ses`, `STAYKEY_SES_REGION`, `STAYKEY_SES_ACCESS_KEY_ID`, `STAYKEY_SES_SECRET_ACCESS_KEY` and `STAYKEY_EMAIL_FROM` (see `apps/api/.env.example`); in the SES sandbox the recipients must be verified addresses. Addresses on example domains, like the seed's sample guests, are never emailed.

### Booking page and calendar sync

Open `http://kingfisher.localhost:3001` after seeding. Guests pick dates on a live calendar, the dates are held while they fill in the checkout, and they book with bank transfer or paying at the property. Their stay page (its link carries a private token) shows the deadline and bank details and takes a photo of the bank slip. The booking app talks to the API only from its server, through `STAYKEY_API_URL`.

The API exports each property's calendar for OTAs at `/ical/<slug>-<token>.ics` (the seed prints the villa's link; `?unit=<id>` gives one unit's) and imports OTA calendar links every 10 minutes. To try an import locally, serve a file with `python3 -m http.server 8765`, point a feed at `http://127.0.0.1:8765/<file>.ics` in the app's iCal screen and tap "Sync now". Links to local and private addresses only work outside production (`STAYKEY_ICAL_ALLOW_PRIVATE`).

### Photos and sensitive data

The app uploads photos and logos straight to storage: `POST /v1/uploads` returns a presigned PUT for the exact type and size, and the property then saves the file's key. In development (`STAYKEY_STORAGE=local`) files are kept in `apps/api/.media` and served by the API under `/media/`, with links built from the address each request came in on, so a phone on your network can load them. Production uses Cloudflare R2 (`STAYKEY_R2_*` and `STAYKEY_MEDIA_URL`).

Bank account numbers are encrypted at rest with `STAYKEY_DATA_KEY` (32 bytes, base64; `openssl rand -base64 32`). Development falls back to a fixed key, and production refuses to start without one. See `apps/api/.env.example` for every setting.

Browsers resolve `*.localhost` to your machine, so subdomain routing works locally without editing hosts files.

To run one app, filter it: `bunx turbo run dev --filter=@staykey/mobile`.

Copy each `.env.example` to `.env` (or `.env.local` for Next.js) when the defaults don't fit. The defaults match `docker-compose.yml`.

## Changing the API

The contract in `packages/api-spec/openapi.yaml` drives both sides:

1. Edit `openapi.yaml`.
2. Run `bun run generate`. This regenerates `apps/api/internal/oapi/api.gen.go` (Go server interfaces) and `packages/api-client/src/schema.ts` (TypeScript types).
3. Implement the new handler methods in `apps/api/internal/server`. The Go compiler lists anything missing.

Generated files are committed, so a fresh checkout builds without running generation first.

## Common commands

| Command | Does |
| --- | --- |
| `bun run setup` | One-time local setup, safe to rerun |
| `bun run dev` | Runs every app in watch mode |
| `bun run build` | Builds everything Turborepo can cache |
| `bun run typecheck` | `tsc` for TypeScript and `go vet` for Go |
| `bun run test` | Go tests, and any TypeScript tests as they're added |
| `bun run lint` | Biome check across the repo (Go is handled by `gofmt` and `go vet`) |
| `bun run format` | Biome format and safe fixes |
| `bun run generate` | Regenerates code from the OpenAPI contract |
| `bun run db:migrate` | Applies database migrations |
| `bun run db:seed` | Creates the sample account (development only) |

New migrations go in `apps/api/internal/db/migrations` with goose's `-- +goose Up` and `-- +goose Down` markers. SQL queries live in `apps/api/internal/db/queries`; `bun run generate` also runs sqlc, which writes typed Go to `apps/api/internal/store/queries`. Owner queries take the request's tenant, so they always filter by account.

## Embedding the widget

```html
<script src="https://cdn.staykey.direct/widget.js" async></script>
<div data-staykey="kingfisher"></div>
```

The loader mounts `kingfisher.staykey.direct/embed` in an iFrame that resizes to fit its content. Booking events are dispatched on the container as `staykey:event` and pushed to `window.dataLayer` when it exists. See `packages/widget/example.html` for a local test page.

## What's built and what's next

Built:
- Monorepo with Bun workspaces, Turborepo, Biome and a Go workspace
- Go API with phone sign-in (codes, rotating refresh tokens), accounts with owner, manager and caretaker roles, tenant scoping on every owner query, and properties with their full setup: units and linked units, photos, seasons, length discounts, stay rules, policies, charges, extras, promo codes, payment methods with encrypted bank details, booking settings, branding and iCal links. Photos upload through presigned links. Backed by PostgreSQL with sqlc and tested against a real database, including tenant isolation for every owner route
- Booking app with subdomain routing, the property page on live availability, holds, guest checkout, a stay page with bank slip upload, and the embed card
- Calendar sync: OTA calendars imported every 10 minutes with backoff, ETags, feed status and alerts, and an export link per property and per unit
- Widget loader with auto-resize and analytics events
- Expo owner app with every V1 screen from the designs: Today (owner and caretaker), month and timeline Calendar with range actions, Bookings, Properties, booking detail, new and edit booking, record payment with bank slips, cancel with policy refunds, contact sheet, Activity, block dates and edit rates sheets, all property settings (details, photos, units, rates and seasons, stay rules, policies, taxes and charges, payment methods, booking settings, branding, share and embed with QR, iCal sync, extras, promo codes), Insights, Team, message templates, notifications, subscription, profile and help
- Owner onboarding in the app: Welcome, 10 or 11 steps across four stages (depending on whole place or rooms), You're live, joining a team by invite link, the setup checklist on Today and the subscription paywall. Steps slide in from the direction of travel with subtle haptics, and progress autosaves so "Finish later" resumes in place. Phone sign-in, publishing, invites and purchases through RevenueCat are real
- Team, activity and alerts on the server: invites by WhatsApp link, roles and property access, an activity feed with read state, message templates, alert preferences per event, Expo push and a 7:00 morning summary, with WhatsApp ready for a Business number
- Bookings on the server: a night ledger that makes double booking impossible (one winner out of 50 parallel bookings for a room and its whole house), quotes priced in Go and held to the app's pricing by shared fixtures, bookings with a status per role and an audit trail, payments, bank slips and refunds, blocks, rate overrides, idempotency keys for money writes and stale-write checks, and river jobs for request expiry, unpaid cancellation and sweeps. `cmd/seed` adds 43 sample stays
- Billing: the server decides each account's plan from RevenueCat (App Store and Google Play) and Payments.lk (prepaid months or years in LKR on the marketing site's `/billing` page). Free until the first direct booking, at most 60 days, then 7 days' grace; a lapsed account's booking page asks guests to message the owner on WhatsApp
- Account controls: deleting yourself (30 days, undone by signing in again), a data export, renaming the account, removing a guest's details, profile photos on the server, and the refund method on cancellation
- Email through AWS SES: guest emails from templates, account emails with email confirmation, team alerts by email, and bounces and complaints suppressed through SNS
- Hardening: Postgres row-level security behind the tenant scoping, rate limits on public and owner routes, and `bun run db:backup-check` to prove a backup restores
- Marketing site landing page, the invite page, the web billing page and the email confirmation page

How the app gets its data today: sign-in, accounts, properties, every property setting, bookings, payments, blocks and rate overrides go through the API, with tokens in the secure store and server data through TanStack Query, cached on the device for offline reads (`apps/mobile/src/api`). So do the team, activity, templates, alert preferences and the profile; there is no on-device data store any more. The app keeps its own copy of the pricing rules for instant previews; the server's total is the one stored. In development builds More has prototype tools: view as owner, manager or caretaker, add a sample guesthouse with rooms and a whole-house unit, preview the paywall and restart onboarding.

Next (the full checklist to launch is `docs/ROADMAP.md`; detail per phase in `docs/backend-plan.md`):
- Phase 9: guest card payments through Payments.lk
- Then the marketing site, error tracking, and end-to-end and load tests
- The phase 8 gate: SES sandbox credentials, then production access and the sending domain's DNS records
- The phase 6 gate: a sandbox purchase in the stores and a Payments.lk sandbox payment, once those accounts exist
- An international SMS gateway, hosting and the store submissions

Trying billing locally: set `STAYKEY_PAYMENTSLK_SECRET_KEY` (sk_test_...) and `STAYKEY_PAYMENTSLK_WEBHOOK_SECRET` for the API, open `http://localhost:3000/billing`, sign in as `+94770000001` and choose a plan; the payment counts once Payments.lk's webhook reaches `/webhooks/payments-lk` (through a tunnel). For the app, set the RevenueCat keys in `apps/mobile/.env`, use a development build with a StoreKit sandbox or Play test account, and set `STAYKEY_REVENUECAT_SECRET_KEY` on the API so "Restore" and the webhook can read the plan back.
