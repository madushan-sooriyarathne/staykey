# StayKey handover

Paste the prompt below into a new chat and attach `staykey.zip` (the full repo with git history). Everything else the new chat needs is inside the repo under `docs/`.

---

## Prompt for the new chat

I'm building StayKey (staykey.direct), a direct booking SaaS for small Sri Lankan villa and guesthouse owners. The attached `staykey.zip` is the monorepo with full git history. Unzip it, read `docs/HANDOVER.md`, `docs/backend-plan.md`, `CLAUDE.md` and `apps/mobile/AGENTS.md`, then start **phase 1 Foundations** from the plan. Ask me the open questions listed in the handover first if any of them block phase 1; otherwise use the defaults given there and flag them.

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
| latest | This handover, the backend plan and design references in `docs/` |

**Important:** the app is a complete, clickable prototype, but only **property creation** hits the API. Everything else reads and writes a zustand store persisted to AsyncStorage (`apps/mobile/src/data/store.ts`). Phase 1 onward replaces that store with the API, screen by screen, without changing the screens.

### Stack

- Bun 1.4.2 workspaces, Turborepo 2.11, Biome 2.5, go.work with Go 1.27.
- **API** (`apps/api`, module `staykey.direct/api`): oapi-codegen v2 strict server, pgx v5, goose migrations, slog. Spec first at `packages/api-spec/openapi.yaml`; `bun run generate` builds `api.gen.go` and the TS client (`packages/api-client`, openapi-typescript plus openapi-fetch). Never edit generated files.
- **Mobile** (`apps/mobile`): Expo SDK 57, expo-router (routes in `src/app`, `Stack.Protected` guards, formSheet sheets), RN 0.86, Reanimated 4.5, zustand 5, expo-symbols, react-native-svg.
- **Booking** (`apps/booking`): Next 16 with subdomain routing and the embed bridge. **Marketing** (`apps/marketing`): Next 16.
- Rules: money is integer minor units plus currency; stay dates are local `YYYY-MM-DD`; linked units (whole house and its rooms) block each other.

### API today

- `apps/api/cmd/api`, `cmd/migrate`; `internal/config`, `internal/domain` (property, slug), `internal/store/postgres.go`, `internal/oapi`, `internal/server`.
- One migration: `db/migrations/00001_create_properties.sql`.
- Endpoints: health, create property, list properties, public property by slug.

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
| Lib | `session.ts` (role, subscription, free period), `auth.ts` (mock OTP), `api.ts`, `purchases.ts`, `contact.ts` (WhatsApp, call, email, templates), `haptics.ts`, `motion.ts`, `prototype.ts`, `storage.ts` |

## 4. Running it

```sh
bun install
bun run db:up          # Postgres 17 in Docker (or point DATABASE_URL at any Postgres 16+)
bun run db:migrate
bun run dev            # API :8080, booking :3001, marketing :3000, Expo :8081
```

- App env (`apps/mobile/.env`): `EXPO_PUBLIC_API_URL=http://<your-LAN-IP>:8080`, `EXPO_PUBLIC_PROTOTYPE_TOOLS=true` to show prototype tools outside dev builds.
- Mock OTP accepts any 6 digits except `000000`.
- More tab → Prototype tools: switch owner and caretaker, add a sample guesthouse, preview the paywall, restart onboarding.

### Verification

```sh
cd apps/mobile && bunx tsc --noEmit && bun test src && bunx expo export --platform ios
bunx biome check .                       # from the repo root
cd apps/api && go vet ./... && go test ./...
bun run typecheck                        # turbo, all workspaces
```

All of these pass at the latest commit (10 pricing tests).

## 5. Known gotchas

- zustand v5: never return a new array or object from a selector. Select the raw value, then filter outside.
- Reanimated custom entering and exiting worklets don't run on web; web falls back to predefined `FadeInRight`, `FadeInLeft` and `FadeOut`.
- The calendar tab and a folder called `calendar/` would collide in expo-router, so date-range sheets live in `range/`.
- Add mobile packages with `bunx expo install`, never plain `bun add`.

## 6. Next step: phase 1 Foundations

Full plan: `docs/backend-plan.md` (live copy: https://claude.ai/code/artifact/5f501309-733d-4bcd-ab38-da01f0ab768d). PRD: https://claude.ai/code/artifact/5bc88e13-3f9b-41f5-8aa9-db852d6bd55d

Tasks, in order:

1. **Migrations 00002+:** `accounts`, `users`, `memberships`, `membership_properties`, `invites`, `otp_challenges`, `sessions`, `push_tokens`, `notification_prefs`; add `account_id` (not null, indexed) to `properties`. UUIDv7 keys generated in Go.
2. **sqlc:** add `sqlc.yaml`, queries under `apps/api/db/queries`, and move `store/postgres.go` onto the generated code.
3. **`internal/auth`:** OTP request and verify (6 digits, 5 min expiry, 5 attempts, rate limit per phone and IP, codes stored hashed), 15 min JWT access token, rotating refresh token stored hashed. An `SMSSender` interface with a log sender for development.
4. **Tenant middleware:** reads `X-Account-Id`, checks membership and role, puts a `Tenant` in the request context. Every store query takes it.
5. **OpenAPI:** `POST /auth/otp`, `POST /auth/verify`, `POST /auth/refresh`, `POST /auth/logout`, `GET /me`, `POST /accounts`, `GET /accounts`. Then `bun run generate` and implement handlers.
6. **App:** real `src/lib/auth.ts`; tokens in `expo-secure-store`; openapi-fetch middleware that adds the token and account header and refreshes once on 401; a TanStack Query provider in the root layout; a `src/api/` folder with one file per area. Property creation moves onto the authenticated client.
7. **`cmd/seed`:** recreates the sample villa, guesthouse and team from `data/seed.ts` on the server.
8. **Tests:** auth flow, refresh rotation and reuse detection, and tenant isolation (another account's token gets 404 on every owner route).

**Gate:** sign in on a real phone against the local API, and the isolation tests pass.

Phases 2 to 6 follow the table in the plan: properties and setup, bookings core with the `unit_nights` ledger, team and activity, iCal and the booking page, then billing and hardening.

## 7. Open questions (defaults if unanswered)

| Question | Default for now |
| --- | --- |
| SMS provider for OTP | Log sender in dev; interface ready for a Sri Lankan gateway or Twilio |
| Hosting region | Fly.io in Singapore with managed Postgres alongside |
| File storage | Cloudflare R2 (S3 API, so swappable) |
| WhatsApp Business number | Deep links only until a number is registered |
| OTA stays | Bookings with an OTA source, as the app does today |
| Row-level security | Phase 6, with `account_id` on every table from the start |
| Demo data | Development builds only |

## 8. Git attribution

End commit messages with:

```
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_019RYduPj9tnoC9yKXKS5gnR
```

End pull request descriptions with:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)

https://claude.ai/code/session_019RYduPj9tnoC9yKXKS5gnR
```
