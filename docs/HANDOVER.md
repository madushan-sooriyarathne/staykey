# StayKey handover

Paste the prompt below into a new chat and attach `staykey.zip` (the full repo with git history). Everything else the new chat needs is inside the repo under `docs/`.

---

## Prompt for the new chat

I'm building StayKey (staykey.direct), a direct booking SaaS for small Sri Lankan villa and guesthouse owners. The attached `staykey.zip` is the monorepo with full git history. Unzip it, read `docs/HANDOVER.md`, `docs/backend-plan.md`, `CLAUDE.md` and `apps/mobile/AGENTS.md`, then start **phase 2 Properties and setup** from the plan. Ask me the open questions listed in the handover first if any of them block phase 2; otherwise use the defaults given there and flag them.

Working style: no em dashes, smooth sentence flow, concise and direct, understated professional tone. Commit after each meaningful step.

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

**Phase 1 is done.** Sign-in, accounts and property creation run on the API. Everything else still reads and writes the zustand store persisted to AsyncStorage (`apps/mobile/src/data/store.ts`). Phase 2 onward replaces that store with the API, screen by screen, without changing the screens. Until then, signing in copies the account's properties into the store (`src/data/from-api.ts`, `src/features/auth/enter.ts`), one unit per property at its base rate.

### Stack

- Bun 1.4.2 workspaces, Turborepo 2.11, Biome 2.5, go.work with Go 1.27.
- **API** (`apps/api`, module `staykey.direct/api`): oapi-codegen v2 strict server, pgx v5, goose migrations, slog. Spec first at `packages/api-spec/openapi.yaml`; `bun run generate` builds `api.gen.go` and the TS client (`packages/api-client`, openapi-typescript plus openapi-fetch). Never edit generated files.
- **Mobile** (`apps/mobile`): Expo SDK 57, expo-router (routes in `src/app`, `Stack.Protected` guards, formSheet sheets), RN 0.86, Reanimated 4.5, zustand 5, expo-symbols, react-native-svg.
- **Booking** (`apps/booking`): Next 16 with subdomain routing and the embed bridge. **Marketing** (`apps/marketing`): Next 16.
- Rules: money is integer minor units plus currency; stay dates are local `YYYY-MM-DD`; linked units (whole house and its rooms) block each other.

### API today

- `apps/api/cmd/api`, `cmd/migrate`, `cmd/seed`.
- `internal/auth`: codes (HMAC-hashed, 5 min, 5 attempts; 1 per 30 s and 5 per hour per number, 20 per hour per IP), 15 min HS256 access tokens checked against their session, refresh tokens `<session id>.<secret>` stored as SHA-256 and rotated on every use. A stale refresh token revokes the session, except within 30 s of a rotation (a retried, lost response). `SMSSender` with `LogSender` only; dev servers return `devCode`.
- `internal/tenant`: `Tenant` (account, user, membership, role, property scope) and `Resolve` for `X-Account-Id`. Unknown account or no membership is 404.
- `internal/server`: each route's security comes from the embedded spec (`bearerAuth`, plus `accountHeader` for owner routes) and is checked by router-level middleware before the body is read. A route missing from the spec is refused.
- `internal/store`: sqlc queries in `internal/db/queries` generate `internal/store/queries`; `store.Postgres` wraps them with the transaction carried in the context. Owner methods take a `tenant.Tenant`. `storetest.New(t)` gives each test its own migrated schema.
- Migrations `00001` to `00004` in `internal/db/migrations`.
- Endpoints: health; `POST /v1/auth/otp`, `/verify`, `/refresh`, `/logout`; `GET` and `PATCH /v1/me`; `GET` and `POST /v1/accounts`; `GET` and `POST /v1/properties` (owner routes; only owners create); public property by slug.
- Tests: sign-in flow, code limits and expiry, refresh rotation and reuse, caretaker scope, owner-only creation, and `TestTenantIsolation`, which walks every owner route in the spec. Routes with path parameters need an entry in its `aliceIDs` fixture or the test fails.

### Mobile app map

| Area | Files |
| --- | --- |
| Root | `src/app/_layout.tsx` waits for fonts and store hydration (`useHydrated`), guards onboarding vs app routes, declares modals and sheets |
| Onboarding | `welcome`, `onboarding`, `live`, `join`, `paywall`; `src/features/onboarding/*` (store, flow, steps, stage header, `to-property.ts`) |
| Tabs | `(tabs)/index` Today (owner and caretaker), `calendar` (month grid, range bar, timeline), `bookings`, `properties`, `more` (prototype tools) |
| Bookings | `booking/[id]`, `booking/new` (new and edit), `payment`, `cancel`, `contact`; `range/block`, `range/rates`; `activity` |
| Property settings | `property/new` (real API call), `property/[id]/` index, details, photos, units, rates, rules, policies, taxes, payments, booking, branding, share, ical, extras, promos |
| More | `insights`, `team`, `templates`, `template/[id]`, `notifications`, `subscription`, `profile`, `help` |
| Components | `controls.tsx` (Button, Radio, inputs), `brand.tsx` (Tag, IconBox, Tick, Row, PulseDot), `kit.tsx` (AppBar, Page, SheetPage, List rows, Card, Segmented, Menu, StatusTag, PropertySwitcher and more), `calendar.tsx`, `icons.ts` |
| Data | `data/types.ts` (the model the schema maps from), `pricing.ts` (quote, conflicts, relatedUnits, refunds), `dates.ts`, `defaults.ts`, `labels.ts`, `seed.ts` (sample villa, guesthouse, team), `store.ts`, `hooks.ts` (useFilter, useProperty, useBooking, useCan), `__tests__/pricing.test.ts` |
| API | `src/api/client.ts` (authenticated openapi-fetch client: token, `X-Account-Id`, refresh ahead of expiry, single-flight refresh and retry on 401), `query-client.ts`, `errors.ts` (`unwrap`, `ApiError`), one file per area: `me.ts`, `accounts.ts`, `properties.ts` |
| Auth | `src/lib/auth.ts` (request, verify, sign out), `src/lib/tokens.ts` (expo-secure-store, AsyncStorage on web), `src/features/auth/enter.ts` (after a verified code: app or setup), `session-check.tsx` (confirms the session on launch) |
| Lib | `session.ts` (user, active account, role, subscription, free period), `purchases.ts`, `contact.ts` (WhatsApp, call, email, templates), `haptics.ts`, `motion.ts`, `prototype.ts`, `storage.ts` |

## 4. Running it

```sh
bun install
bun run db:up          # Postgres 17 in Docker (or point DATABASE_URL at any Postgres 16+)
bun run db:migrate
bun run db:seed        # sample owner +94770000001, manager +94772223344, caretaker +94713338899
bun run dev            # API :8080, booking :3001, marketing :3000, Expo :8081
```

- App env (`apps/mobile/.env`): `EXPO_PUBLIC_API_URL=http://<your-LAN-IP>:8080`, `EXPO_PUBLIC_PROTOTYPE_TOOLS=true` to show prototype tools outside dev builds.
- Sign-in codes go to the API log, and development builds show the code the dev server returns. Tap "I have an account" with a seeded number to land in that account.
- More tab → Prototype tools: switch owner and caretaker, add a sample guesthouse, preview the paywall, restart onboarding.

### Verification

```sh
cd apps/mobile && bunx tsc --noEmit && bun test src && bunx expo export --platform ios
bunx biome check .                       # from the repo root
cd apps/api && go vet ./... && go test ./...   # needs Postgres; CI=true fails instead of skipping
bun run typecheck                        # turbo, all workspaces
bun run generate && git diff --exit-code # generated code is up to date
```

All of these pass at the latest commit (17 app tests, 14 Go integration tests against Postgres plus domain tests).

## 5. Known gotchas

- zustand v5: never return a new array or object from a selector. Select the raw value, then filter outside.
- Reanimated custom entering and exiting worklets don't run on web; web falls back to predefined `FadeInRight`, `FadeInLeft` and `FadeOut`.
- The calendar tab and a folder called `calendar/` would collide in expo-router, so date-range sheets live in `range/`.
- Add mobile packages with `bunx expo install`, never plain `bun add`.
- Turborepo runs in strict environment mode. A new environment variable an API task reads must be added to that task's `passThroughEnv` in `turbo.json`.
- sqlc runs through `go run ...@v1.31.1` from `bun run generate`; the first run compiles it (about a minute).
- `afterSignIn` resets the on-device store when a different person signs in, and keeps it for the same person.

## 6. Next step: phase 2 Properties and setup

Full plan: `docs/backend-plan.md` (live copy: https://claude.ai/code/artifact/5f501309-733d-4bcd-ab38-da01f0ab768d). PRD: https://claude.ai/code/artifact/5bc88e13-3f9b-41f5-8aa9-db852d6bd55d

Backend: extend `properties` and add `property_photos`, `units`, `unit_links`, `payment_settings`, `seasons`, `season_prices`, `rate_overrides`, `length_discounts`, `charges`, `extras`, `promos`, all with `account_id`. Photos go to R2 with presigned uploads. `POST /v1/onboarding/publish` takes the whole draft in one transaction. App: onboarding publish, the Properties tab and all 14 settings screens move from the on-device store to `src/api/`, and `src/data/from-api.ts` goes away. Add each new owner route's path parameters to `TestTenantIsolation`. Extend `cmd/seed` with units, seasons and settings from `data/seed.ts`.

**Gate:** publish from onboarding and edit every setting against the API.

Phase 1 gate status: the isolation tests pass. Sign-in was checked end to end in a browser against the local API (seeded owner, and a new owner through setup and publish); signing in on a real phone is still to be done.

## 7. Open questions (defaults if unanswered)

| Question | Default for now |
| --- | --- |
| SMS provider for OTP | Log sender only; production refuses to start without `STAYKEY_SMS_PROVIDER` |
| Hosting region | Fly.io in Singapore with managed Postgres alongside (`STAYKEY_CLIENT_IP_HEADER=Fly-Client-IP`) |
| File storage | Cloudflare R2 (S3 API, so swappable) |
| WhatsApp Business number | Deep links only until a number is registered |
| OTA stays | Bookings with an OTA source, as the app does today |
| Row-level security | Phase 6, with `account_id` on every table from the start |
| Demo data | Development only: `cmd/seed` refuses production |

Phase 1 calls made without asking, to confirm or change:

- Account creation happens at publish, named after the first property. There is no account rename yet.
- Owners land in their first owned account, team members in the first account they joined. An account switcher waits for phase 4.
- Only owners can add properties, since a property counts towards the plan. Managers get 403.
- `PATCH /v1/me` was added in phase 1 so onboarding can save the owner's name.
- The 30 second refresh grace period exists for retries on flaky mobile networks.
- The mock "I was invited to a team" flow is unchanged until invites land in phase 4.

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
