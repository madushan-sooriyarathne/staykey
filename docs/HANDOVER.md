# StayKey handover

Development continues in Claude Code on your own machine. The repo carries everything it needs: `CLAUDE.md` (loaded automatically), this handover, `docs/ROADMAP.md`, the plan, project permissions in `.claude/settings.json`, and two commands, `/next-phase` and `/verify`.

---

## Getting started in Claude Code

1. Unzip `staykey.zip` and open a terminal in the `staykey` folder. Git history comes with it.
2. Optional: push to your own GitHub repository so CI runs: `git remote add origin git@github.com:<you>/staykey.git && git push -u origin main`.
3. Run `bun run setup`. It checks for Bun, Go, Node and Docker, installs dependencies, creates `apps/mobile/.env`, starts Postgres, migrates and seeds the sample account.
4. Run `claude` in the same folder and paste the prompt below, or simply type `/next-phase`.

### Prompt

I'm continuing StayKey (staykey.direct), a direct booking SaaS for small Sri Lankan villa and guesthouse owners. This repo is the monorepo with full git history, and phases 1 and 2 are done. Read `docs/HANDOVER.md`, `docs/ROADMAP.md`, `docs/backend-plan.md`, `CLAUDE.md` and `apps/mobile/AGENTS.md`, run `/verify` to confirm the starting point is green, then start **phase 3 Bookings core**. Ask me the open questions in the handover first only if any of them block phase 3; otherwise use the defaults given there and flag them. Working style: no em dashes, smooth sentence flow, concise and direct, understated professional tone. Commit after each meaningful step, and tick the roadmap as items land.

---

## 1. Product context

- **What it is:** owners get a hosted booking page (`<slug>.staykey.direct`), a website widget and iFrame, and one calendar, all set up from the mobile app in about 10 minutes.
- **V1:** Go backend, React Native owner app (Expo SDK 57), guest booking surfaces (booking page, widget, iFrame). The Next.js owner dashboard is V1.1.
- **Calendar sync:** iCal import and export only. No channel manager.
- **Pricing, by bookable units:**

  | Plan | Units | Monthly | Yearly |
  | --- | --- | --- | --- |
  | Villa | 1 | $9 | $90 |
  | Guesthouse | up to 6 | $19 | $190 |
  | Collection | up to 20 | $39 | $390 |

  Free until the first direct booking, capped at 60 days, then a 7-day grace period. A lapsed subscription turns the booking page into "message the owner on WhatsApp". Billing runs through RevenueCat (in-app) and PayHere (web).
- **Booking state machine:** requested → awaiting_payment → confirmed → checked_in → checked_out, plus cancelled and declined.
- **Roles:** owner, manager, caretaker. Caretakers see Today and bookings for their assigned properties only.

## 2. Brand rules

- Colours: obsidian `#09090b` for primary buttons, paper `#f4f4f5` canvas, cloud `#ececee` borders.
- Ember `#ff5a00` marks direct bookings and anything the owner must act on. Magenta Spark `#fe45e2` marks wins and live status.
- Radii: 12 badge, 14 button and input, 28 card, 36 large card. Font: DM Sans.
- All tokens come from `@staykey/tokens`. Full spec in `docs/design/DESIGN.md`; reference mockups in `docs/design/*.html`.

## 3. Repo and current state

Commits so far:

| Commit | What |
| --- | --- |
| `6076a5f` | Monorepo scaffold: Go API, Expo app, Next booking and marketing, OpenAPI spec, tokens, widget |
| `06289b4` | Owner onboarding: animated direction-aware steps, haptics, paywall |
| `bc43945` | Every owner app screen, running on an on-device store |
| `6596d58` | This handover, the backend plan and design references in `docs/` |
| `6b1acfc` | Phase 1 migrations: accounts, people, auth tables, `account_id` on properties |
| `fe48a21` | Phase 1 API: sqlc store, phone sign-in, accounts, tenant scoping, tests |
| `9ed2b20` | `cmd/seed` for the sample account |
| `94efd69` | Phase 1 app: real sign-in, secure tokens, TanStack Query, authenticated client |
| `f1f9861` | Turborepo passes API settings through to tasks |
| `917e2af` | README and handover for phase 1 |
| `1fc1ff4` | Phase 2 `internal/files` (local and R2 presigned uploads) and `internal/secret` (AES-GCM) |
| `79a46b4` | Phase 2 API: migration 00005, full property setup, sectioned `PATCH`, uploads |
| `775549f` | `cmd/seed` creates the sample properties with their full setup |
| `6c39832` | Phase 2 app: onboarding publish, Properties tab and every settings screen on the API |
| `1acb93e` | README, plan and handover for phase 2 |
| `d03948f` | `docs/ROADMAP.md`, Claude Code setup (`CLAUDE.md`, `.claude/`), `bun run setup` |

**Phases 1 and 2 are done.** Sign-in, accounts, properties and every property setting run on the API, and onboarding publishes in one call. Bookings, blocks, rate overrides, activity, team and templates still live in the zustand store persisted to AsyncStorage (`apps/mobile/src/data/store.ts`, version 2). Phase 3 moves bookings, blocks and rate overrides; phase 4 the rest.

### Stack

- Bun 1.4.2 workspaces, Turborepo 2.11, Biome 2.5, go.work with Go 1.27.
- **API** (`apps/api`, module `staykey.direct/api`): oapi-codegen v2 strict server, pgx v5, goose migrations, sqlc, slog, aws-sdk-go-v2 for R2. Spec first at `packages/api-spec/openapi.yaml`; `bun run generate` builds `api.gen.go`, the sqlc queries and the TS client (`packages/api-client`, openapi-typescript plus openapi-fetch). Never edit generated files.
- **Mobile** (`apps/mobile`): Expo SDK 57, expo-router (routes in `src/app`, `Stack.Protected` guards, formSheet sheets), RN 0.86, Reanimated 4.5, zustand 5, TanStack Query 5 with the AsyncStorage persister, expo-secure-store, expo-symbols, react-native-svg.
- **Booking** (`apps/booking`): Next 16 with subdomain routing and the embed bridge. **Marketing** (`apps/marketing`): Next 16.
- Rules: money is integer minor units plus currency; stay dates are local `YYYY-MM-DD`; linked units (whole house and its rooms) block each other.

### API today

- `apps/api/cmd/api`, `cmd/migrate`, `cmd/seed`.
- `internal/auth`: codes (HMAC-hashed, 5 min, 5 attempts; 1 per 30 s and 5 per hour per number, 20 per hour per IP), 15 min HS256 access tokens checked against their session, refresh tokens `<session id>.<secret>` stored as SHA-256 and rotated on every use. A stale refresh token revokes the session, except within 30 s of a rotation (a retried, lost response). `SMSSender` with `LogSender` only; dev servers return `devCode`.
- `internal/tenant`: `Tenant` (account, user, membership, role, property scope) and `Resolve` for `X-Account-Id`. Unknown account or no membership is 404.
- `internal/domain`: slugs, `NewProperty`, and `setup.go` with every setup type, `PropertyPatch` (pointer fields, collections replaced whole), `Normalize`, `Validate` and `DefaultCharges`. New collection items carry a temporary `ref` so links and season prices can point at units created in the same request.
- `internal/files`: `Store` with `PresignUpload`, `Exists` and `URL`. `local.go` signs PUT and GET links served by the API under `/media/` (development); `r2.go` presigns against R2 with the content type and length signed in. Keys are `accounts/{account}/{photos|logos}/{uuidv7}.{ext}` and must belong to the caller's account.
- `internal/secret`: AES-256-GCM, bound to a context string. Bank account numbers are sealed with the property id.
- `internal/server`: each route's security comes from the embedded spec (`bearerAuth`, plus `accountHeader` for owner routes) and is checked by router-level middleware before the body is read. A route missing from the spec is refused. `property_map.go` converts between API and domain types and redacts bank details and promos for caretakers.
- `internal/store`: sqlc queries in `internal/db/queries` generate `internal/store/queries`; `store.Postgres` wraps them with the transaction carried in the context (`InTx`, savepoints when nested). Owner methods take a `tenant.Tenant`. `setup.go` loads and applies the setup: units are archived rather than deleted, promo `used_count` survives edits, a changed iCal link resets its status. `storetest.New(t)` gives each test its own migrated schema.
- Migrations `00001` to `00005` in `internal/db/migrations`. Child tables use composite foreign keys `(id, account_id)` so a row can never point into another account.
- Endpoints: health; `POST /v1/auth/otp`, `/verify`, `/refresh`, `/logout`; `GET` and `PATCH /v1/me`; `GET` and `POST /v1/accounts`; `GET` and `POST /v1/properties` (`POST` takes `setup` for onboarding or `baseRate` for a quick add; only owners create); `GET` and `PATCH /v1/properties/{id}` (full setup; sectioned patch in one transaction); `POST /v1/uploads` (presigned PUT for an exact type and size); public property by slug.
- Permissions: owners and managers edit settings; prices need `PermPrices`; caretakers read their properties only and get 403 on edits.
- Tests: sign-in flow, code limits and expiry, refresh rotation and reuse, caretaker scope, owner-only creation, full publish, partial patches, validation, settings permissions, photo uploads, file stores, encryption, domain validation, and `TestTenantIsolation`, which walks every owner route in the spec. Routes with path parameters need an entry in its `aliceIDs` fixture or the test fails.

### Mobile app map

| Area | Files |
| --- | --- |
| Root | `src/app/_layout.tsx` waits for fonts, store hydration (`useHydrated`) and the restored query cache, guards onboarding vs app routes, declares modals and sheets |
| Onboarding | `welcome`, `onboarding`, `live`, `join`, `paywall`; `src/features/onboarding/*` (store, flow, steps, stage header, `to-property.ts`). `flow.ts` publishes: account, owner name, photo uploads, then one `POST /v1/properties` with `setup` |
| Tabs | `(tabs)/index` Today (owner and caretaker), `calendar` (month grid, range bar, timeline), `bookings`, `properties`, `more` (prototype tools) |
| Bookings | `booking/[id]`, `booking/new` (new and edit), `payment`, `cancel`, `contact`; `range/block`, `range/rates`; `activity` |
| Property settings | `property/new`, `property/[id]/` index, details, photos, units, rates, rules, policies, taxes, payments, booking, branding, share, ical, extras, promos. `features/property/settings.tsx` holds `useSettings` (save sends a `PATCH` of the changed keys, with saving and error states) and `useLiveSave` (units, photos and iCal save as they change) |
| More | `insights`, `team`, `templates`, `template/[id]`, `notifications`, `subscription`, `profile`, `help` |
| Components | `controls.tsx` (Button, Radio, inputs, InfoNote), `brand.tsx` (Tag, IconBox, Tick, Row, PulseDot), `kit.tsx` (AppBar, Page, SheetPage, List rows, Card, Segmented, Menu, StatusTag, PropertySwitcher and more), `calendar.tsx`, `icons.ts` |
| Data | `data/types.ts` (the screens' model), `from-api.ts` (`toPropertyConfig`, `toPatch`, `toNewProperty`), `pricing.ts` (quote, conflicts, relatedUnits, refunds), `dates.ts`, `defaults.ts`, `labels.ts`, `seed.ts` (sample bookings and team), `store.ts` (bookings and the rest not yet on the API), `hooks.ts` (useProperties and useProperty from the API, useFilter, useBooking, useCan) |
| API | `src/api/client.ts` (authenticated openapi-fetch client: token, `X-Account-Id`, refresh ahead of expiry, single-flight refresh and retry on 401), `query-client.ts` (cache persisted for a week, `clearCache`), `errors.ts` (`unwrap`, `ApiError`), one file per area: `me.ts`, `accounts.ts`, `properties.ts`, `uploads.ts` |
| Auth | `src/lib/auth.ts` (request, verify, sign out), `src/lib/tokens.ts` (expo-secure-store, AsyncStorage on web), `src/features/auth/enter.ts` (after a verified code: app, setup or a "no invite" error), `session-check.tsx` (confirms the session on launch) |
| Lib | `session.ts` (user, active account, role, subscription, free period), `purchases.ts`, `contact.ts` (WhatsApp, call, email, templates), `haptics.ts`, `motion.ts`, `prototype.ts`, `storage.ts` |

## 4. Running it

```sh
bun run setup          # once: install, Postgres 17 in Docker, migrate, seed
                       # sample owner +94770000001, manager +94772223344, caretaker +94713338899
bun run dev            # API :8080, booking :3001, marketing :3000, Expo :8081
```

- App env (`apps/mobile/.env`): `EXPO_PUBLIC_API_URL=http://<your-LAN-IP>:8080`, `EXPO_PUBLIC_PROTOTYPE_TOOLS=true` to show prototype tools outside dev builds.
- API env (`apps/api/.env.example`): in development, photos and logos are stored in `apps/api/.media` and served by the API, and the encryption key is derived from a fixed development value. Production needs `STAYKEY_DATA_KEY` and the `STAYKEY_R2_*` settings.
- Sign-in codes go to the API log, and development builds show the code the dev server returns. Tap "I have an account" with a seeded number to land in that account.
- More tab → Prototype tools: switch owner and caretaker, add a sample guesthouse (published to the API as `coralbay`), preview the paywall, restart onboarding.

### Verification

```sh
cd apps/mobile && bunx tsc --noEmit && bun test src && bunx expo export --platform ios
bunx biome check .                       # from the repo root
cd apps/api && go vet ./... && go test ./...   # needs Postgres; CI=true fails instead of skipping
bun run typecheck                        # turbo, all workspaces
bun run generate && git diff --exit-code # generated code is up to date
```

All of these pass at the latest commit (20 app tests; 30 Go tests, 19 of them integration tests against Postgres).

## 5. Known gotchas

- zustand v5: never return a new array or object from a selector. Select the raw value, then filter outside. TanStack Query's `select` should be a stable, module-level function for the same reason.
- Reanimated custom entering and exiting worklets don't run on web; web falls back to predefined `FadeInRight`, `FadeInLeft` and `FadeOut`.
- The calendar tab and a folder called `calendar/` would collide in expo-router, so date-range sheets live in `range/`.
- Add mobile packages with `bunx expo install`, never plain `bun add`.
- Turborepo runs in strict environment mode. A new environment variable an API task reads must be added to that task's `passThroughEnv` in `turbo.json`.
- sqlc runs through `go run ...@v1.31.1` from `bun run generate`; the first run compiles it (about a minute).
- Adding an enum to the spec can rename generated Go constants (`oapi.Ok` became `oapi.HealthStatusOk`).
- `afterSignIn` resets the on-device store and the query cache when a different person signs in, and keeps both for the same person. It leaves the onboarding draft alone, since that is the one being filled in.
- The query cache is persisted with a `buster` in `src/api/query-client.ts`. Bump it whenever a cached response shape changes.
- If the API suddenly reports a missing column after a migration, check for an older API process still holding port 8080. Kill it by pid; `pkill -f` patterns can match your own shell.

## 6. Next step: phase 3 Bookings core

Everything left before launch, phase by phase and including accounts and store release, is in `docs/ROADMAP.md`. Full plan: `docs/backend-plan.md` (live copy: https://claude.ai/code/artifact/5f501309-733d-4bcd-ab38-da01f0ab768d). PRD: https://claude.ai/code/artifact/5bc88e13-3f9b-41f5-8aa9-db852d6bd55d

Backend: add `guests`, `bookings`, `booking_lines`, `booking_extras`, `payments`, `payment_slips`, `booking_events`, `blocks`, `block_units`, `unit_nights` and `rate_overrides`, all with `account_id` and composite foreign keys. Port `pricing.ts` to `internal/domain/pricing` and check both against shared fixtures in `packages/api-spec/fixtures/pricing.json`. Add the booking state machine per status and role, `POST /v1/quote`, bookings with `Idempotency-Key` and a `version` for stale writes, payments and slips (slip uploads reuse `internal/files` with a new kind), cancel with policy refunds, blocks and rate overrides. The ledger inserts a `unit_nights` row for every night on the unit and its related units, so a clash fails the unique key and returns 409. App: Today, Calendar, Bookings, booking detail, new and edit booking, record payment, cancel, and the block and rates sheets move from the on-device store to `src/api/`. Add each new owner route's path parameters to `TestTenantIsolation`. Extend `cmd/seed` with the sample bookings from `data/seed.ts`.

**Gate:** exactly one winner out of 50 parallel bookings on overlapping nights for a unit and its whole-house parent.

Phase 2 gate status: done. Checked end to end in a browser against the local API: a new owner published from onboarding with three photos in one call, and the seeded owner edited and saved every settings screen (details, photos, units, rates and seasons, stay rules, policies, taxes, payments, booking, branding, extras, promos, iCal), each read back from the database. The seeded caretaker joined and landed on Today; a number without an invite was told so. Signing in on a real phone (the phase 1 gate) is still to be done.

## 7. Open questions (defaults if unanswered)

| Question | Default for now |
| --- | --- |
| SMS provider for OTP | Log sender only; production refuses to start without `STAYKEY_SMS_PROVIDER` |
| Hosting region | Fly.io in Singapore with managed Postgres alongside (`STAYKEY_CLIENT_IP_HEADER=Fly-Client-IP`) |
| File storage | Cloudflare R2, now in use through the S3 API (so swappable). Not yet tried against a real bucket |
| WhatsApp Business number | Deep links only until a number is registered |
| OTA stays | Bookings with an OTA source, as the app does today |
| Row-level security | Phase 6, with `account_id` on every table from the start |
| Demo data | Development only: `cmd/seed` refuses production |

Calls made without asking, to confirm or change:

- Account creation happens at publish, named after the first property. There is no account rename yet.
- Owners land in their first owned account, team members in the first account they joined. An account switcher waits for phase 4.
- Only owners can add properties, since a property counts towards the plan. Managers get 403.
- `PATCH /v1/me` was added in phase 1 so onboarding can save the owner's name.
- The 30 second refresh grace period exists for retries on flaky mobile networks.
- Phase 2 uses one sectioned `PATCH /v1/properties/{id}` and `POST /v1/properties` with `setup`, instead of a route per sub-resource and `POST /v1/onboarding/publish`. Each settings screen sends only its own section, and the whole save is one transaction.
- Caretakers can read their properties but not the bank details or promo codes, and cannot edit anything.
- Card payments can't be switched on from the app. Only a PayHere approval will set them to on; the app can ask (pending).
- iCal links are saved and stay pending until the sync worker arrives in phase 5.
- Uploaded files that are never attached to a property are not cleaned up yet. A sweep job belongs with the phase 3 jobs.
- The join flow now goes through phone sign-in and checks for an account; real invite links arrive in phase 4.
- Prototype mode still adds sample bookings on the device, but no longer changes a real property's settings.

## 8. Git attribution

End commit messages with:

```
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: <link to the session making the commit>
```

End pull request descriptions with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)

<link to the session making the pull request>
```
