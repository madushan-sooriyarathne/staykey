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
bun run dev          # API on :8080, booking on :3001, marketing on :3000, Expo on :8081
```

Create a property, then open its booking page:

```sh
curl -X POST localhost:8080/v1/properties \
  -H 'content-type: application/json' \
  -d '{"name":"Kingfisher Villa","slug":"kingfisher","bookingType":"entire","currency":"USD","baseRate":18000}'

open http://kingfisher.localhost:3001
```

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

New migrations go in `apps/api/internal/db/migrations` with goose's `-- +goose Up` and `-- +goose Down` markers.

## Embedding the widget

```html
<script src="https://cdn.staykey.direct/widget.js" async></script>
<div data-staykey="kingfisher"></div>
```

The loader mounts `kingfisher.staykey.direct/embed` in an iFrame that resizes to fit its content. Booking events are dispatched on the container as `staykey:event` and pushed to `window.dataLayer` when it exists. See `packages/widget/example.html` for a local test page.

## What's built and what's next

Built:
- Monorepo with Bun workspaces, Turborepo, Biome and a Go workspace
- Go API with health, create and list properties, and public property lookup, backed by PostgreSQL with tests
- Booking app with subdomain routing, a property page and the embed bridge
- Widget loader with auto-resize and analytics events
- Expo owner app with every V1 screen from the designs: Today (owner and caretaker), month and timeline Calendar with range actions, Bookings, Properties, booking detail, new and edit booking, record payment with bank slips, cancel with policy refunds, contact sheet, Activity, block dates and edit rates sheets, all property settings (details, photos, units, rates and seasons, stay rules, policies, taxes and charges, payment methods, booking settings, branding, share and embed with QR, iCal sync, extras, promo codes), Insights, Team, message templates, notifications, subscription, profile and help
- Owner onboarding in the app: Welcome, 10 or 11 steps across four stages (depending on whole place or rooms), You're live, team invites, the setup checklist on Today and the subscription paywall. Steps slide in from the direction of travel with subtle haptics, and progress autosaves so "Finish later" resumes in place. Phone codes and purchases are stubbed (any 6 digits except 000000 verify)
- Marketing site landing page

How the app gets its data today: property creation goes through the API, and everything else runs on an on-device store (`apps/mobile/src/data`) with typed domain models, pricing and availability rules covered by tests, and sample bookings generated around today's date. Screens read the store through small hooks, so moving each area to the API means swapping the store actions for API calls. In development (or with `EXPO_PUBLIC_PROTOTYPE_TOOLS=true`) More has prototype tools: view as owner, manager or caretaker, add a sample guesthouse with rooms and a whole-house unit, preview the paywall and restart onboarding.

Next:
- Owner auth with phone OTP, plus accounts and tenant scoping on every table
- API endpoints for units, rates, bookings, blocks, payments and team, replacing the on-device store
- The night inventory ledger, holds and the booking state machine on the server
- iCal import and export workers
- Real OTP endpoints, photo upload, and RevenueCat for in-app subscriptions
