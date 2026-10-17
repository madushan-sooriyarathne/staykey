# StayKey handover

Development continues in Claude Code on your own machine. The repo carries everything it needs: `CLAUDE.md` (loaded automatically), this handover, `docs/ROADMAP.md`, the plan, project permissions in `.claude/settings.json`, and two commands, `/next-phase` and `/verify`.

---

## Getting started in Claude Code

1. Unzip `staykey.zip` and open a terminal in the `staykey` folder. Git history comes with it.
2. Optional: push to your own GitHub repository so CI runs: `git remote add origin git@github.com:<you>/staykey.git && git push -u origin main`.
3. Run `bun run setup`. It checks for Bun, Go, Node and Docker, installs dependencies, creates `apps/mobile/.env`, starts Postgres, migrates and seeds the sample account.
4. Run `claude` in the same folder and paste the prompt below, or simply type `/next-phase`.

### Prompt

I'm continuing StayKey (staykey.direct), a direct booking SaaS for small Sri Lankan villa and guesthouse owners. This repo is the monorepo with full git history, and phases 1 to 4 are done. Read `docs/HANDOVER.md`, `docs/ROADMAP.md`, `docs/backend-plan.md`, `CLAUDE.md` and `apps/mobile/AGENTS.md`, run `/verify` to confirm the starting point is green, then start **phase 5 Calendar sync and booking page**. Ask me the open questions in the handover first only if any of them block it; otherwise use the defaults given there and flag them. Working style: no em dashes, smooth sentence flow, concise and direct, understated professional tone. Commit after each meaningful step, and tick the roadmap as items land.

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
| `1f4af3f` | Monorepo scaffold: Go API, Expo app, Next booking and marketing, OpenAPI spec, tokens, widget |
| `3efe484` | Owner onboarding: animated direction-aware steps, haptics, paywall |
| `3bdb0c4` | Every owner app screen, running on an on-device store |
| `c4e1eb1` | This handover, the backend plan and design references in `docs/` |
| `742b69e` | Phase 1 migrations: accounts, people, auth tables, `account_id` on properties |
| `143f846` | Phase 1 API: sqlc store, phone sign-in, accounts, tenant scoping, tests |
| `cdccbbb` | `cmd/seed` for the sample account |
| `ce37014` | Phase 1 app: real sign-in, secure tokens, TanStack Query, authenticated client |
| `ece1ca0` | Turborepo passes API settings through to tasks |
| `7d712cc` | README and handover for phase 1 |
| `7ffa691` | Phase 2 `internal/files` (local and R2 presigned uploads) and `internal/secret` (AES-GCM) |
| `1dee357` | Phase 2 API: migration 00005, full property setup, sectioned `PATCH`, uploads |
| `daf9304` | `cmd/seed` creates the sample properties with their full setup |
| `b5dfe25` | Phase 2 app: onboarding publish, Properties tab and every settings screen on the API |
| `dac5ec3` | README, plan and handover for phase 2 |
| `0b15bad` | `docs/ROADMAP.md`, Claude Code setup (`CLAUDE.md`, `.claude/`), `bun run setup` |
| `ef732e1` | Phase 3 migration 00006: guests, bookings, lines, extras, payments, slips, events, blocks, the `unit_nights` ledger, rate overrides, idempotency keys |
| `cf1c122` | `internal/domain/pricing` and `packages/api-spec/fixtures/pricing.json`, run by `go test` and `bun test` |
| `02cce3d` | Booking rules: transitions per role, ledger units, refs |
| `dcf63c3` | Store for stays, payments, blocks and overrides, with the 50-way gate test |
| `360d6a4` | Quote, booking, payment, slip, block, calendar and override routes |
| `8a62a5f` | Jobs on river (migration 00007): request expiry, unpaid cancellation, expired holds and keys, orphaned uploads |
| `3c395c7` | `cmd/seed` adds the sample stays, slips and block |
| `377387f` | Phase 3 app: Today, Calendar, Bookings, detail, new and edit, payment, cancel, block and rates on the API |
| `74fcf94` | Agent config: `.agents/` and `.claude/` rules, hooks, reviewers and skills |
| `a301dc5` | `internal/sms`: oZone Sender for Sri Lankan numbers, behind `STAYKEY_SMS_PROVIDER=ozone` |
| `08c45f2` | `docs/phase-4-plan.md` and the roadmap brought up to date |
| `bf65c2a` | Phase 4 migration 00008: `activity` (also the alert outbox), `activity_reads`, `message_templates`, `notification_prefs.last_sent_on`, `push_tokens.session_id`, with a backfill; sqlc queries for the team, activity and messages |
| `8925cb5` | Domain: invite tokens and validation, member patches, activity kinds and the text for each stay change, default templates and alert preferences |
| `1207d4d` | Store: team and invites, activity written inside the booking transactions and by the jobs, templates, preferences, push tokens, alert and summary queries |
| `683d410` | Team, invite, activity, template, preference and push token routes, in `TestTenantIsolation` |
| `3410963` | `internal/notify` (Expo push, WhatsApp log and Cloud API senders) and the `notify` and `morning_summary` jobs |
| `1e52520` | Marketing `/join/[token]` page that hands the invite to the app |
| `3feebbb` | App hooks for the team, activity, templates, preferences and push registration |
| `145f102` | Phase 4 app: Team, Join, Activity, Templates, Notifications, Profile and the account switcher on the API; `data/store.ts` deleted |
| `c990385`, `3bf2855` | The seed's stays don't send alerts; joining lands on Today |

**Phases 1 to 4 are done.** Sign-in, accounts, properties, every property setting, bookings, payments, slips, blocks, rate overrides, the team and invites, activity, message templates, notification preferences and push alerts run on the API. The on-device data store is gone; zustand only holds the session, the onboarding draft and UI state.

### Stack

- Bun 1.4.2 workspaces, Turborepo 2.11, Biome 2.5, go.work with Go 1.27.
- **API** (`apps/api`, module `staykey.direct/api`): oapi-codegen v2 strict server, pgx v5, goose migrations, sqlc, slog, aws-sdk-go-v2 for R2. Spec first at `packages/api-spec/openapi.yaml`; `bun run generate` builds `api.gen.go`, the sqlc queries and the TS client (`packages/api-client`, openapi-typescript plus openapi-fetch). Never edit generated files.
- **Mobile** (`apps/mobile`): Expo SDK 57, expo-router (routes in `src/app`, `Stack.Protected` guards, formSheet sheets), RN 0.86, Reanimated 4.5, zustand 5, TanStack Query 5 with the AsyncStorage persister, expo-secure-store, expo-symbols, react-native-svg.
- **Booking** (`apps/booking`): Next 16 with subdomain routing and the embed bridge. **Marketing** (`apps/marketing`): Next 16.
- Rules: money is integer minor units plus currency; stay dates are local `YYYY-MM-DD`; linked units (whole house and its rooms) block each other.

### API today

- `apps/api/cmd/api`, `cmd/migrate`, `cmd/seed`.
- `internal/auth`: codes (HMAC-hashed, 5 min, 5 attempts; 1 per 30 s and 5 per hour per number, 20 per hour per IP), 15 min HS256 access tokens checked against their session, refresh tokens `<session id>.<secret>` stored as SHA-256 and rotated on every use. A stale refresh token revokes the session, except within 30 s of a rotation (a retried, lost response). `SMSSender`, with `LogSender` for development; dev servers return `devCode`.
- `internal/sms`: `Router` sends Sri Lankan numbers (+94) through the configured sender and refuses other countries with a 400 on `phone` ("International numbers are not supported yet."). `Ozone` texts through oZone Sender (`STAYKEY_SMS_PROVIDER=ozone` plus `STAYKEY_OZONE_USER_ID`, `STAYKEY_OZONE_API_KEY`, `STAYKEY_OZONE_SENDER_ID`); its codes 203 and 205 come back as a 400 on `phone`, any other non-204 code as a 500.
- `internal/tenant`: `Tenant` (account, user, membership, role, property scope) and `Resolve` for `X-Account-Id`. Unknown account or no membership is 404.
- `internal/domain/booking.go`: statuses and sources, `StayInput` with validation, the transition table (`CanTransition`: approve, decline and confirm need `PermManage`; check in and out are open to every role, including from `awaiting_payment` for guests paying at the property), `LedgerUnits`, `RefPrefix`, `ConflictError`, `ErrStale`, `ErrBadTransition`, `ErrKeyReused`.
- `internal/domain/pricing`: port of `pricing.ts` (`Price`, `NightlyRate`, `MinNights`, `Deposit`, `Refund`, `CustomLines`). Integer maths; `percentOf` rounds half up like `Math.round`.
- `internal/jobs`: one river worker, `sweep`, with six periodic jobs: `expire_requests` and `cancel_unpaid` every minute, `sweep_expired` (holds, idempotency keys) every minute, `sweep_uploads` hourly (files older than a day that nothing points at), `notify` every 15 seconds and `morning_summary` every 15 minutes. The river client runs inside the API process; river elects the leader that schedules.
- `internal/notify`: `Expo` posts batches of 100 to Expo's push service and returns the tokens it answers `DeviceNotRegistered` for; `LogWhatsApp` logs alerts, `CloudWhatsApp` sends an approved template (title and detail line as its two body parameters) through the Cloud API. Chosen with `STAYKEY_WHATSAPP_PROVIDER` (`log` or `cloud`).
- Alerts: every activity row starts with `notified_at` empty. `notify` claims up to 100 in a short transaction that stamps them (`FOR UPDATE SKIP LOCKED`), works out the recipients (active members who want that event through `domain.EventFor`, can see the property and didn't act themselves, without payments and slips for caretakers), commits, and only then sends. Push goes first; if it fails the rows are released for the next run. A failed WhatsApp message is logged. Tokens only count while their session is signed in.
- Morning summary: for each member whose `summary` preference is on, between 07:00 and 11:00 in the time zone of the account's first property, once a day (`notification_prefs.last_sent_on`): arrivals, departures and requests waiting for their properties.
- `internal/domain/team.go`, `activity.go`, `messages.go`: `NewInviteToken` (32 random bytes, SHA-256 stored), `InviteTTL` (14 days), `NewInvite` and `MemberPatch` validation (manager or caretaker only), `ErrOwnerMembership`, `ErrAlreadyMember`; activity kinds, `HiddenFromCaretakers`, `EventFor`, and builders for each line (`BookingAdded`, `RequestAnswered`, `RequestExpired`, `StayCancelled`, `StayUnpaid`, `PaymentRecorded`, `SlipRejected`) written in the third person with the actor's first name; `DefaultTemplates` and `DefaultNotificationPrefs`.
- `internal/domain`: slugs, `NewProperty`, and `setup.go` with every setup type, `PropertyPatch` (pointer fields, collections replaced whole), `Normalize`, `Validate` and `DefaultCharges`. New collection items carry a temporary `ref` so links and season prices can point at units created in the same request.
- `internal/files`: `Store` with `PresignUpload`, `Exists`, `URL`, `WrittenBefore` and `Delete`. Kinds are photo, logo and slip. `local.go` signs PUT and GET links served by the API under `/media/` (development); `r2.go` presigns against R2 with the content type and length signed in. Keys are `accounts/{account}/{photos|logos}/{uuidv7}.{ext}` and must belong to the caller's account.
- `internal/secret`: AES-256-GCM, bound to a context string. Bank account numbers are sealed with the property id.
- `internal/server`: each route's security comes from the embedded spec (`bearerAuth`, plus `accountHeader` for owner routes) and is checked by router-level middleware before the body is read. A route missing from the spec is refused. `property_map.go` converts between API and domain types and redacts bank details and promos for caretakers.
- `internal/store`: sqlc queries in `internal/db/queries` generate `internal/store/queries`; `store.Postgres` wraps them with the transaction carried in the context (`InTx`, savepoints when nested). Owner methods take a `tenant.Tenant`. `setup.go` loads and applies the setup: units are archived rather than deleted, promo `used_count` survives edits, a changed iCal link resets its status. `storetest.New(t)` gives each test its own migrated schema.
- `internal/store/bookings.go`, `calendar.go`, `idempotency.go`, `jobs.go`: creating a stay locks the account's `booking_seq` (refs like `KV-2041` count per account, and stays in one account queue behind each other), matches the guest by phone or email, writes the frozen lines and extras, then holds the nights. The ledger insert runs in a savepoint with units sorted, so a clash rolls back cleanly, comes back as a `ConflictError` naming the stay or block, and two transactions can't deadlock. `changeBooking` locks the row, checks the caller's property scope and the `version`, and saves; every change moves the version on. `Idempotent` claims the key inside the request's transaction, so a concurrent retry waits for the first attempt and then replays its reply.
- `internal/store/team.go`, `activity.go`, `messages.go`: team and invites (a new invite revokes the open one to the same number; accepting checks the signed-in phone and is idempotent), `addActivity` called inside `CreateBooking`, `TransitionBooking` (answered requests), `CancelBooking`, `RecordPayment`, `RejectSlip` and the jobs' `closeBySystem`, `ListActivity` with the caller's scope, `SendAlerts`, `DueSummaries`, templates and preferences. `CreateAccount` adds the four templates and the owner's preferences; `AddMember` adds the member's preferences.
- Migrations `00001` to `00008` in `internal/db/migrations`. `00007` is river's schema, dumped with `river migrate-get`. Child tables use composite foreign keys `(id, account_id)` so a row can never point into another account.
- Phase 3 endpoints: `POST /v1/quote`; `GET` and `POST /v1/bookings` (list by overlap with `from`, `to`, `propertyId`, cursor pages of up to 500); `GET` and `PATCH /v1/bookings/{bookingId}`; `POST /v1/bookings/{bookingId}/transitions`, `/cancel` and `/payments`; `POST /v1/slips/{slipId}/reject` (accepting a slip is a payment with `slipId`); `GET /v1/properties/{id}/calendar` (blocks and overrides, the next twelve months by default); `POST /v1/blocks`, `DELETE /v1/blocks/{blockId}`; `PUT /v1/rate-overrides`. `POST /bookings`, `/cancel` and `/payments` take `Idempotency-Key`. Errors: 409 `dates_taken` with the clash, 409 `stale`, 409 `bad_status`, 422 `idempotency_key_reused`. Caretakers see stays without money.
- Phase 4 endpoints: `GET /v1/team`; `POST /v1/team/invites` (returns the link once); `DELETE /v1/team/invites/{inviteId}`; `PATCH` and `DELETE /v1/team/members/{membershipId}` (owners only, never the owner's own membership); `POST /v1/invites/accept` and `POST /v1/public/invites/preview` (tokens in the body, never the path); `GET /v1/activity` (cursor pages with the unread count) and `POST /v1/activity/read`; `GET /v1/templates`, `PATCH /v1/templates/{templateId}`; `GET` and `PUT /v1/notification-prefs`; `POST /v1/push-tokens`.
- Endpoints: health; `POST /v1/auth/otp`, `/verify`, `/refresh`, `/logout`; `GET` and `PATCH /v1/me`; `GET` and `POST /v1/accounts`; `GET` and `POST /v1/properties` (`POST` takes `setup` for onboarding or `baseRate` for a quick add; only owners create); `GET` and `PATCH /v1/properties/{id}` (full setup; sectioned patch in one transaction); `POST /v1/uploads` (presigned PUT for an exact type and size); public property by slug.
- Permissions: owners and managers edit settings; prices need `PermPrices`; caretakers read their properties only and get 403 on edits.
- Tests: sign-in flow, code limits and expiry, refresh rotation and reuse, caretaker scope, owner-only creation, full publish, partial patches, validation, settings permissions, photo uploads, file stores, encryption, domain validation, and `TestTenantIsolation`, which walks every owner route in the spec. Routes with path parameters need an entry in its `aliceIDs` fixture or the test fails.

### Mobile app map

| Area | Files |
| --- | --- |
| Root | `src/app/_layout.tsx` waits for fonts, store hydration (`useHydrated`) and the restored query cache, guards onboarding vs app routes, declares modals and sheets, mounts `usePushAlerts`. `join` sits after both guarded groups, open to anyone |
| Onboarding | `welcome`, `onboarding`, `live`, `join` (reads `token` from `staykey://join?token=`, previews the invite, accepts on sign-in or straight away when signed in), `paywall`; `src/features/onboarding/*` (store, flow, steps, stage header, `to-property.ts`). `flow.ts` publishes: account, owner name, photo uploads, then one `POST /v1/properties` with `setup` |
| Tabs | `(tabs)/index` Today (owner and caretaker), `calendar` (month grid, range bar, timeline), `bookings`, `properties`, `more` (account switcher when someone belongs to several accounts, prototype tools) |
| Bookings | `booking/[id]`, `booking/new` (new and edit), `payment`, `cancel`, `contact`; `range/block`, `range/rates`; `activity` |
| Property settings | `property/new`, `property/[id]/` index, details, photos, units, rates, rules, policies, taxes, payments, booking, branding, share, ical, extras, promos. `features/property/settings.tsx` holds `useSettings` (save sends a `PATCH` of the changed keys, with saving and error states) and `useLiveSave` (units, photos and iCal save as they change) |
| More | `insights`, `team`, `templates`, `template/[id]`, `notifications`, `subscription`, `profile`, `help` |
| Components | `controls.tsx` (Button, Radio, inputs, InfoNote), `brand.tsx` (Tag, IconBox, Tick, Row, PulseDot), `kit.tsx` (AppBar, Page, SheetPage, List rows, Card, Segmented, Menu, StatusTag, PropertySwitcher and more), `calendar.tsx`, `icons.ts` |
| Data | `data/types.ts` (the screens' model), `from-api.ts` (`toPropertyConfig`, `toPatch`, `toNewProperty`, `toBooking`, `toCalendar`), `pricing.ts` (instant previews: quote, conflicts, relatedUnits, refunds), `dates.ts`, `defaults.ts`, `labels.ts`, `seed.ts` (sample property setups), `hooks.ts` (useProperties, useBookings and useBooking from the API, useFilter, useCan) |
| API | `src/api/client.ts` (authenticated openapi-fetch client: token, `X-Account-Id`, refresh ahead of expiry, single-flight refresh and retry on 401), `query-client.ts` (cache persisted for a week, `clearCache`), `errors.ts` (`unwrap`, `ApiError`), one file per area: `me.ts`, `accounts.ts`, `properties.ts`, `uploads.ts`, `bookings.ts` (list loaded page by page from 400 days back; approve, decline, check in, payments and slip rejection are optimistic and roll back; create, edit and cancel wait), `calendar.ts` (blocks and overrides per property, optimistic), `team.ts` (team, invites, preview and accept), `activity.ts` (infinite pages, `useUnreadCount`, optimistic mark read; booking changes invalidate it), `messages.ts` (templates, preferences with optimistic switches). `features/bookings/move.ts` holds `useMoveStay` for status changes |
| Auth | `src/lib/auth.ts` (request, verify, sign out), `src/lib/tokens.ts` (expo-secure-store, AsyncStorage on web), `src/features/auth/enter.ts` (after a verified code: accept a pending invite, then app, setup or an error; `openAccount` also serves the account switcher), `session-check.tsx` (confirms the session on launch) |
| Lib | `session.ts` (user, active account, role, subscription, free period, profile photo), `push.ts` (permission, Expo token sent to `/v1/push-tokens`, tapped alerts open the stay or Activity), `purchases.ts`, `contact.ts` (WhatsApp, call, email, templates), `haptics.ts`, `motion.ts`, `prototype.ts`, `storage.ts` |

## 4. Running it

```sh
bun run setup          # once: install, Postgres 17 in Docker, migrate, seed
                       # sample owner +94770000001, manager +94772223344, caretaker +94713338899
bun run dev            # API :8080, booking :3001, marketing :3000, Expo :8081
```

- App env (`apps/mobile/.env`): `EXPO_PUBLIC_API_URL=http://<your-LAN-IP>:8080`, `EXPO_PUBLIC_PROTOTYPE_TOOLS=true` to show prototype tools outside dev builds.
- API env (`apps/api/.env.example`): in development, photos and logos are stored in `apps/api/.media` and served by the API, and the encryption key is derived from a fixed development value. Production needs `STAYKEY_DATA_KEY` and the `STAYKEY_R2_*` settings.
- Sign-in codes go to the API log, and development builds show the code the dev server returns. Tap "I have an account" with a seeded number to land in that account.
- `cmd/seed` prints an invite for +94715550199 as both `https://staykey.direct/join/<token>` and `staykey://join?token=<token>`. Open the second on a simulator (`xcrun simctl openurl booted '<link>'`) to try joining.
- Push alerts need an EAS project id in `app.json` (`bunx eas-cli init`) and a development build; without one the app skips registration with a single warning. WhatsApp alerts and summaries are written to the API log until `STAYKEY_WHATSAPP_PROVIDER=cloud`.
- More tab → Prototype tools: switch owner and caretaker, add a sample guesthouse (published to the API as `coralbay`), preview the paywall, restart onboarding.

### Verification

```sh
cd apps/mobile && bunx tsc --noEmit && bun test src && bunx expo export --platform ios
bunx biome check .                       # from the repo root
cd apps/api && go vet ./... && go test ./...   # needs Postgres; CI=true fails instead of skipping
bun run typecheck                        # turbo, all workspaces
bun run generate && git diff --exit-code # generated code is up to date
```

All of these pass at the latest commit (32 app tests; 111 Go tests, most of them integration tests against Postgres 17). Integration tests skip when Postgres is down, so check the database is up before trusting a green run.

## 5. Known gotchas

- zustand v5: never return a new array or object from a selector. Select the raw value, then filter outside. TanStack Query's `select` should be a stable, module-level function for the same reason.
- Reanimated custom entering and exiting worklets don't run on web; web falls back to predefined `FadeInRight`, `FadeInLeft` and `FadeOut`.
- The calendar tab and a folder called `calendar/` would collide in expo-router, so date-range sheets live in `range/`.
- Add mobile packages with `bunx expo install`, never plain `bun add`.
- Turborepo runs in strict environment mode. A new environment variable an API task reads must be added to that task's `passThroughEnv` in `turbo.json`.
- sqlc runs through `go run ...@v1.31.1` from `bun run generate`; the first run compiles it (about a minute).
- Adding an enum to the spec can rename generated Go constants (`oapi.Ok` became `oapi.HealthStatusOk`).
- `afterSignIn` clears the query cache when a different person signs in and keeps it for the same person. It leaves the onboarding draft alone, since that is the one being filled in.
- Adding `request` to the `ActivityKind` and `NotificationEvent` enums renamed `oapi.Request` to `oapi.BookingSettingsModeRequest`; generated names collide whenever two enums share a value.
- `ListActivity` and `CountUnreadActivity` filter with `NOT (kind = ANY(hidden))`. Pass an empty slice for nobody hidden; a nil slice is a SQL null and hides everything.
- An unguarded screen listed before the guarded groups in `_layout.tsx` becomes where the app lands when a guard flips. Keep `join` last.
- Expo Go doesn't receive remote push on Android since SDK 53; use a development build.
- The query cache is persisted with a `buster` in `src/api/query-client.ts`. Bump it whenever a cached response shape changes.
- The night ledger keeps a row per night for every stay that holds dates, past ones included. A whole-house unit holds its rooms' nights, not its own, so read a house's availability through its rooms.
- Recording a payment moves the booking's `version` on. Send the version from the latest copy, not the one the form opened with.
- River runs in the API process. A second API machine is safe (river elects one scheduler), but stopping the API also stops the sweeps.
- `river migrate-get` needs Go 1.26 or later and is how `00007` was made; regenerate a new goose migration the same way when upgrading river.
- The river migration (`00007`) needs Postgres 17. On 16 it fails with `unsafe use of new value "pending" of enum type river_job_state`, so run 17 locally and pick 17 for the managed database.
- Colima can start other projects' Postgres containers on 5432. If `bun run db:up` says the port is taken, stop the other container, then `docker compose up -d --force-recreate postgres`.
- If the API suddenly reports a missing column after a migration, check for an older API process still holding port 8080. Kill it by pid; `pkill -f` patterns can match your own shell.

## 6. Next step: phase 5

Everything left before launch is in `docs/ROADMAP.md`; detail per phase in `docs/backend-plan.md`. Phase 4 followed `docs/phase-4-plan.md`; phase 5's execution plan is `docs/phase-5-plan.md`.

Phase 5 adds the iCal import and export workers and the public routes; the booking page's holds use `unit_nights.hold_expires_at`, which the `sweep_expired` job already clears, and slips upload with `files.KindSlip`. Requests and slips from the booking page should write activity through the existing builders (`domain.BookingAdded` already writes "Request from ..." for a requested stay; add one for a slip upload), and the iCal worker's failures belong in activity as `sync` items so the alerts reach the team.

**Phase 4 gate status: done up to the phone.** In a browser against the seeded API: the owner invited a caretaker from Team, WhatsApp opened with the link, the link's join screen showed the account, inviter and role, signing in with the invited number accepted it and landed on Today, and the caretaker's Activity listed the account's stays without payments. Against the running API, a stay the owner added was routed by the `notify` job to the caretaker's push token and sent to Expo's live push service, which answered for the test token and had it removed; the owner's own device got nothing. `TestNewStayAlertsTheCaretakerNotTheOwner` covers the same routing. A push on a real phone waits for an EAS project id and a development build.

**Phase 3 gate status: done.** `TestOneWinnerOutOfFiftyParallelBookings` races 50 bookings on overlapping nights for a room and its whole-house unit; exactly one wins and 49 get a clash, ten runs in a row under `-race`. Checked end to end in a browser against the seeded API: Today loaded the sample stays, approving a request moved it to awaiting payment, a payment confirmed it with the audit trail `requested > awaiting_payment > confirmed`, a clashing new booking showed the overlap, a free one was created and priced by the server (KV-2084) and then cancelled, and a block and a rate override saved and showed on the Calendar. Signing in on a real phone (the phase 1 gate) is still to be done.

## 7. Open questions (defaults if unanswered)

| Question | Default for now |
| --- | --- |
| SMS provider for OTP | oZone Sender for Sri Lankan numbers; international numbers are refused until a second gateway is chosen. Production refuses to start without `STAYKEY_SMS_PROVIDER` |
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
- Prototype mode no longer adds sample stays on the device; `cmd/seed` creates them on the server. Its sample alerts stay but no longer link to a stay.
- Phase 3: a whole-house unit holds the nights of the rooms it links rather than its own, so two rooms on the same night don't clash (the plan's "unit and every related unit" would have made them clash through the house).
- Phase 3: booking refs count per account from 2041 with the property's initials, so refs in one account never repeat across properties.
- Phase 3: an `Idempotency-Key` belongs to one account, lasts 24 hours, and covers the record and body it was sent with; another body gets 422.
- Phase 3: owner-created stays are confirmed straight away. Requests (and their expiry) come from the booking page in phase 5; the job that declines them is in place.
- Phase 3: accepting a bank slip is a payment sent with `slipId`, so there is no separate accept route.
- Phase 3: caretakers can check guests in from `awaiting_payment`, for guests who pay at the property.
- Phase 3: the river client runs in the API process instead of a separate worker mode. Split it out when jobs need their own machine.
- Phase 3: refunds go back by the method the guest last paid with; the cancel screen has no method picker yet.
- Phase 3: payment, slip and cancel screens wait for the server and show its error, rather than closing first, so a refused money write is never lost silently.
- Phase 4: invite links are `https://staykey.direct/join/<token>` (`STAYKEY_INVITE_URL`), a marketing page that opens `staykey://join?token=`. Universal links and app links wait for hosting.
- Phase 4: invites last 14 days, only owners manage the team, a new invite to the same number replaces the open one, and accepting needs the signed-in phone to be the invited one (anything else is a 404, like an unknown token). Accepting twice keeps one membership.
- Phase 4: read state is one `last_read_at` per person per account; leaving Activity marks everything read, and the person's own actions never count as unread. The per-item marks are gone.
- Phase 4: activity text is written by the server in the third person ("Nadeesha approved Sam Lee"), so everyone in the account reads the same line. Check-in and check-out don't write activity.
- Phase 4: caretakers never see payment or slip activity, and no other kind carries an amount.
- Phase 4: alerts go out through an outbox (`activity.notified_at`) drained every 15 seconds rather than river jobs inserted inside the booking transaction. Rows are stamped when claimed and sent outside any transaction, so a slow send can't hold locks or replay after a job timeout. A send that reports a failure releases its rows for the next run; an API that stops mid-send loses that batch's alerts.
- Phase 4: until a WhatsApp Business number is set up, the log sender records only that an alert wasn't sent and the last three digits of the number, never the text, since the log provider runs in production too.
- Phase 4: invites only accept +94 numbers, since sign-in codes can't reach other countries until there's an international SMS gateway.
- Phase 4: `POST /v1/push-tokens` only takes Expo tokens (`ExponentPushToken[...]` or `ExpoPushToken[...]`, up to 300 characters). The server doesn't validate bodies against the spec, so limits like this live in the handlers.
- Phase 4: no Expo receipt polling; `DeviceNotRegistered` in the send reply removes the token.
- Phase 4: the morning summary uses the first property's time zone and goes out between 07:00 and 11:00, so an API that was down at 07:00 still sends it that morning but not in the evening.
- Phase 4: the profile photo stays on the phone (`useSession.photoUri`); there's no user photo upload yet.
- Phase 4: Insights needs no endpoint; it already works from the stays the app loads.
- Phase 4: the email sender and automatic guest messages from templates wait for an email provider. Templates are edited and used from the contact sheet.
- Phase 4: `cmd/seed` marks its sample activity as already alerted, so a fresh seed doesn't send a burst of alerts.
- Phase 4: the WhatsApp Cloud sender targets Graph API v23.0; check Meta's changelog when turning it on.
- SMS: oZone is called with a form POST, as its parameter table says, rather than the GET in its sample, so the API key never sits in a URL. Not yet tried against the live gateway. There are no retries, since a retry after a timeout can text the code twice; the person can resend after 30 seconds.
- SMS: a refused international number still stores a challenge, so a repeat within 30 seconds gets "try again" rather than the international message.

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
