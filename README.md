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
bun install
bun run db:up        # start PostgreSQL in Docker
bun run db:migrate   # apply migrations
bun run db:seed      # optional: the sample account below
bun run dev          # API on :8080, booking on :3001, marketing on :3000, Expo on :8081
```

The seed creates an owner (+94 77 000 0001, or `-owner-phone` for your own number) with Kingfisher Villa and Coral Bay House, each fully set up with units, seasons, length discounts, charges, extras, promo codes, bank details and iCal links, plus a manager (+94 77 222 3344) and a caretaker for the villa (+94 71 333 8899). Open a booking page at http://kingfisher.localhost:3001.

### Signing in

Owners sign in with a 6-digit code sent to their phone. No SMS provider is connected yet, so in development the code is written to the API log and returned by the API, and development builds of the app show it under the code boxes. To sign in on a phone, set `EXPO_PUBLIC_API_URL` in `apps/mobile/.env` to your computer's LAN address, tap "I have an account" and use one of the seeded numbers.

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
- Booking app with subdomain routing, a property page and the embed bridge
- Widget loader with auto-resize and analytics events
- Expo owner app with every V1 screen from the designs: Today (owner and caretaker), month and timeline Calendar with range actions, Bookings, Properties, booking detail, new and edit booking, record payment with bank slips, cancel with policy refunds, contact sheet, Activity, block dates and edit rates sheets, all property settings (details, photos, units, rates and seasons, stay rules, policies, taxes and charges, payment methods, booking settings, branding, share and embed with QR, iCal sync, extras, promo codes), Insights, Team, message templates, notifications, subscription, profile and help
- Owner onboarding in the app: Welcome, 10 or 11 steps across four stages (depending on whole place or rooms), You're live, team invites, the setup checklist on Today and the subscription paywall. Steps slide in from the direction of travel with subtle haptics, and progress autosaves so "Finish later" resumes in place. Phone sign-in and publishing are real; purchases and team invites are still stubbed
- Marketing site landing page

How the app gets its data today: sign-in, accounts, properties and every property setting go through the API, with tokens in the secure store and server data through TanStack Query, cached on the device for offline reads (`apps/mobile/src/api`). Bookings, blocks, rate overrides, activity, team and templates still run on an on-device store (`apps/mobile/src/data`) with typed domain models, pricing and availability rules covered by tests, and sample bookings generated around today's date. Screens read the store through small hooks, so moving each area to the API means swapping the store actions for API calls. In development (or with `EXPO_PUBLIC_PROTOTYPE_TOOLS=true`) More has prototype tools: view as owner, manager or caretaker, add a sample guesthouse with rooms and a whole-house unit, preview the paywall and restart onboarding.

Next (see `docs/backend-plan.md`):
- Phase 3: the night inventory ledger, quotes, bookings and the state machine, payments and slips, blocks and rate overrides on the server, replacing the on-device store
- Team invites, activity, templates and notifications
- iCal import and export workers, and the booking page on live data
- An SMS provider for codes, and RevenueCat for in-app subscriptions
