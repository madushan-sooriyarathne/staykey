# StayKey handover

Development continues in Claude Code on your own machine. The repo carries everything it needs: `CLAUDE.md` (loaded automatically), this handover, `docs/ROADMAP.md`, the plan, project permissions in `.claude/settings.json`, and two commands, `/next-phase` and `/verify`.

---

## Getting started in Claude Code

1. Unzip `staykey.zip` and open a terminal in the `staykey` folder. Git history comes with it.
2. Optional: push to your own GitHub repository so CI runs: `git remote add origin git@github.com:<you>/staykey.git && git push -u origin main`.
3. Run `bun run setup`. It checks for Bun, Go, Node and Docker, installs dependencies, creates `apps/mobile/.env`, starts Postgres, migrates and seeds the sample account.
4. Run `claude` in the same folder and paste the prompt below, or simply type `/next-phase`.

### Prompt

I'm continuing StayKey (staykey.direct), a direct booking SaaS for small Sri Lankan villa and guesthouse owners. This repo is the monorepo with full git history, and phases 1 to 5 are done. Read `docs/HANDOVER.md`, `docs/ROADMAP.md`, `docs/backend-plan.md`, `CLAUDE.md` and `apps/mobile/AGENTS.md`, run `/verify` to confirm the starting point is green, then start **phase 6 Billing and hardening**. Ask me the open questions in the handover first only if any of them block it; otherwise use the defaults given there and flag them. Working style: no em dashes, smooth sentence flow, concise and direct, understated professional tone. Commit after each meaningful step, and tick the roadmap as items land.

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

  Free until the first direct booking, capped at 60 days, then a 7-day grace period. A lapsed subscription turns the booking page into "message the owner on WhatsApp". Billing runs through RevenueCat (in-app) and Payments.lk (web).
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
| `ba16e1a` | `docs/phase-5-plan.md` |
| `a0c6d06` | Phase 5 migration 00009: checkout holds on the night ledger (`unit_nights.hold_id`), `bookings.guest_token_hash` and `payment_method`, `ical_feeds.next_sync_at`; sqlc queries for holds, guest access and the feed schedule |
| `56ddcc9` | Domain: guest stay rules and promo checks (`pricing.GuestStay`, `PromoProblem`), feed rules (`ImportUnits`, `ExportRuns`, `NextSync`) and the new activity lines |
| `7130193` | `internal/ical`: feed parsing in the property's time zone, a fetcher that refuses internal addresses, and the export writer |
| `6370e3a`, `b197c46`, `5c3b79b` | Spec: booking page, guest booking and calendar sync routes |
| `6b3d2f7`, `2875c04` | Store: holds, guest bookings and slips, feed imports, the export |
| `2964ddc` | Public routes, `GET /ical/{file}` and `POST /v1/ical-feeds/{feedId}/sync` |
| `50634a2` | The `ical_import` job, `STAYKEY_ICAL_ALLOW_PRIVATE`, and the seed's parked feeds |
| `0a7f417`, `b698698`, `7ed9732` | Booking app: calendar, hold, checkout, stay page and slip upload; the widget carries its dates over |
| `b5af978` | App: "Sync now", feeds per unit, export links per unit, how a direct guest pays |
| `8fccf0f` | `docs/phase-6-plan.md` |
| `572dd34` | Phase 6 migrations: 00010 (`subscriptions`, `billing_events`, `web_checkouts`, the `billing` activity kind, a backfill of `first_direct_booking_at`) and 00011 (row-level security on every tenant table) |
| `9612456`, `2d4426e` | Billing queries; `domain/billing.go` (`BillingState`, plans, product ids), billing activity lines; `internal/billing` (RevenueCat and Payments.lk clients) |
| `4a7985b`, `9b29e8d` | Spec: subscription, checkout and webhook routes, `pausedReason` and `whatsapp` on the public property; store: billing state, the first direct booking, the tenant set on each pooled connection |
| `8fdf61e`, `999ee55`, `f222ddd` | Billing routes and webhooks, `internal/ratelimit` and the limits middleware, the `billing` job, provider config, `scripts/db-backup-check.sh`, `GET /v1/public/plans` |
| `0781080`, `79f4f87` | Marketing `/billing` and `/billing/done`: phone sign-in, plan, Payments.lk checkout |
| `4c8fc8b` | Booking app: WhatsApp fallback for lapsed plans, guest addresses forwarded to the API |
| `a3547a1` | App: RevenueCat purchases, `useSubscription`, paywall, Subscription, More and the Today banner on the server's plan |

**Phases 1 to 6 are built.** Sign-in, accounts, properties, every property setting, bookings, payments, slips, blocks, rate overrides, the team and invites, activity, message templates, notification preferences and push alerts run on the API. The on-device data store is gone; zustand only holds the session, the onboarding draft and UI state. Guests book on the booking page and widget against live availability, and OTA calendars come in and go out by iCal. The server decides each account's plan from RevenueCat and Payments.lk; a lapsed account's page sends guests to the owner on WhatsApp. Public and owner routes are rate limited, and tenant requests are walled by row-level security. The phase 6 gate waits for sandbox accounts (section 6).

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
- `internal/jobs`: one river worker, `sweep`, with eight periodic jobs (each also runs when the API starts): `expire_requests` and `cancel_unpaid` every minute, `sweep_expired` (holds, idempotency keys) every minute, `sweep_uploads` hourly (files older than a day that nothing points at), `notify` every 15 seconds, `morning_summary` every 15 minutes `ical_import` every minute (three minutes allowed per run) and `billing` every 15 minutes (moves accounts into grace and lapsed as their access runs out). The river client runs inside the API process; river elects the leader that schedules.
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
- `internal/ical`: `Parse` reads a feed's events as local days in the property's time zone (DATE-TIME values converted, an unknown `TZID` read as the property's zone, a missing end as one night) and leaves out cancelled events, events that ended, StayKey's own `@staykey.direct` events and anything past 24 months. `Fetcher` turns `webcal://` into `https://`, sends the ETag, reads at most 5 MB in 20 s and, unless built with `allowPrivate`, refuses loopback, private, link-local, CGNAT and multicast addresses on every connection, redirects included, with no proxy. `Problem` turns an error into a line for the owner that never repeats the link. `Write` writes the export: one all-day event per run of nights, "Booked" or "Not available".
- `ical_import`: leases up to 20 due feeds for 5 minutes (`next_sync_at`), fetches five at a time outside any transaction, and applies each feed in one transaction (`store.ApplyFeed`): new events become confirmed stays with the channel as source, a "<Channel> stay" guest of their own, total 0 and `feed_id`/`external_uid`; moved events move; events that left the feed cancel stays that haven't checked out; stays the owner cancelled stay cancelled. A stay whose nights are taken is kept without nights and tried again on each sync; the team hears once (`ImportClash`) when the clash is with a direct stay or a block, never for an import against an import. The first sync writes one "calendar connected" line. Working feeds are read every 10 minutes; failures back off 10, 20, 40 minutes up to 6 hours, show as `error` after 2 and alert the team on the 3rd. Feed to unit: the feed's `unitId`, else the only unit, else the whole-house unit, else every unit (recorded on the first).
- Holds: ledger rows with `hold_id` and `hold_expires_at` (the property's `holdMinutes`). `holdNights` clears lapsed holds on the units it is about to take before inserting, so a stale hold never blocks anyone, and a clash with a live hold names nothing. Booking with a `holdId` swaps the hold's rows for the booking's in one transaction; `replaces` on a new hold drops the guest's previous one.
- Guest bookings (`store/public.go`): created through the same `insertBooking` as owner stays with no actor, matched to an existing guest only when the name and the phone or email match (a stored guest is never changed), a go of the promo code used in the same transaction, and a `DirectBooking` or "Request from" activity line. The guest's token (24 random bytes) is returned once; its SHA-256 sits on the booking and is compared in constant time. Request mode makes `requested` with `replyHours`; instant mode makes `awaiting_payment`, with a payment deadline only for bank transfer when the property cancels unpaid stays. Approving a request for a guest paying at the property sets no deadline.
- Phase 5 endpoints: `GET /v1/public/properties/{slug}` (now the whole public setup and `takingBookings`), `/availability?from&to` (up to 13 months; taken nights per unit through linked units, and closed arrival days), `POST /quote` (stay rules, sleeps, enabled extras, promo dates, limit and minimum nights), `POST /holds` (201 with `holdId` and `expiresAt`), `POST /bookings` (requires `Idempotency-Key`; 201 with the booking and token); `GET /v1/public/bookings/{bookingId}`, `POST .../uploads` and `POST .../slips` with `X-Booking-Token`; `GET /ical/{slug}-{token}.ics` (with `?unit=`, `text/calendar`, cached 5 minutes); owner `POST /v1/ical-feeds/{feedId}/sync`. Public conflicts are 409 `dates_taken` ("Those dates were just taken"), `not_taking_bookings` (property paused, or no bank transfer and no paying at the property) or `bad_status`, never with the other stay. Booking page reads reuse owner loaders through `systemTenant`, which has no user and never reaches a write.
- Billing (`domain/billing.go`, `store/billing.go`, `server/billing.go`, `server/webhooks.go`): `BillingState` works out `trial`, `active`, `grace` or `lapsed` from `trial_started_at`, `first_direct_booking_at` and the `subscriptions` row: free until the first direct booking, at most 60 days; paid access to the plan's period end; then 7 days' grace after whichever ends later. `accounts.status` is written by `recomputeBilling` (inside the change that caused it, and by the `billing` job), with one activity line per change (`billing` kind, owners only, on the `booking` alert preference). The first direct booking is the first `page` or `widget` stay that is created as or approved to `awaiting_payment` or `confirmed`.
- Billing routes: `GET /v1/subscription` (every role: status, until, free end, plan, period, source, units, the plan that fits, `canPurchase`); `POST /v1/subscription/refresh` (owners; reads RevenueCat now); `POST /v1/subscription/checkout` (owners; `{plan, period}` gives a Payments.lk checkout URL; 409 `already_subscribed` while a store plan runs); `GET /v1/public/plans` (web prices in LKR); `POST /webhooks/revenuecat` (Authorization compared in constant time; the event only names the account, the plan is read back from RevenueCat's `GET /v1/subscribers/{id}`); `POST /webhooks/payments-lk` (HMAC-SHA256 `Payments-Signature` over the raw body, 5 minute tolerance; `payment.succeeded` pays the checkout its `reference` names, `checkout.expired` closes it). Each delivery is recorded in `billing_events` in the same transaction as its change, so a retry is applied once. Payloads keep ids and amounts only.
- Web periods: one checkout buys one month or year, starting when the current web period ends or now. The amount comes from `domain.Plans`, and a payment whose amount or currency differs is refused and logged. With `STAYKEY_REVENUECAT_EXTERNAL_KEY` each paid period is also posted to RevenueCat's External Purchases API; its failures are logged and never undo the payment.
- Booking page: `pausedReason` is `lapsed` (with the owner's phone in `whatsapp`), `paused` or `no_payment_method`; holds and bookings on a lapsed account get 409 `not_taking_bookings`. The status is worked out at read time, so the page closes when grace ends rather than at the next job run.
- Rate limits (`internal/ratelimit`, `server/limits.go`): fixed windows in memory per API machine. Holds 10 per 10 minutes and guest bookings 5 per 10 minutes per address, uploads and slips 10 per hour per booking, other public routes and `/ical/` 120 per minute per address, owner routes 300 per minute per person. Sign-in has its own limits; webhooks have none. 429 `rate_limited` with `Retry-After`. A request with `X-StayKey-Forward-Key` equal to `STAYKEY_FORWARD_SECRET` is counted by its `X-StayKey-Guest-IP`.
- Row-level security (00011): every table with `account_id` has a `tenant_wall` policy, forced for the owner too. `store.setTenant` runs as each pooled connection is handed out and sets `staykey.account_id` to the request's tenant, or empty (every row) when the context has none. Superusers skip the policies, so `storetest` connects as an ordinary role (`staykey_rls_test`) whenever the test database's user is a superuser.
- Migrations `00001` to `00011` in `internal/db/migrations`. `00007` is river's schema, dumped with `river migrate-get`. Child tables use composite foreign keys `(id, account_id)` so a row can never point into another account.
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
| iCal | `property/[id]/ical.tsx`: "Sync now" (`useSyncFeed` marks the feed due and refetches after 5 s), a unit picker per feed when the property has several units, and export links built from `EXPO_PUBLIC_API_URL`, one per unit plus the whole property |
| Billing | `src/lib/purchases.ts` (RevenueCat: `identify(accountId)` on opening an account, `purchase`, `restore`; without `EXPO_PUBLIC_REVENUECAT_APPLE_KEY` or `_GOOGLE_KEY`, or on web, they say purchases aren't available), `src/api/subscription.ts` (`useSubscription`, `useRefreshSubscription`), `paywall.tsx` (buys, then asks the server a few times until it says `active`; managers and caretakers are told to ask the owner; a web plan shows its end date and no price), `subscription.tsx`, `features/today/plan-banner.tsx` (grace, lapsed, or a plan smaller than the units) |
| Lib | `session.ts` (user, active account, role, profile photo), `push.ts` (permission, Expo token sent to `/v1/push-tokens`, tapped alerts open the stay or Activity), `purchases.ts`, `contact.ts` (WhatsApp, call, email, templates), `haptics.ts`, `motion.ts`, `prototype.ts`, `storage.ts` |

### Booking app map

| Area | Files |
| --- | --- |
| Data | `src/lib/api.ts` (`server-only`: property, availability, quote, hold, book, guest booking, slip upload through the presigned link), `paths.ts` (links on the property's subdomain or under `/p/<slug>`), `dates.ts`, `stay-params.ts` |
| Actions | `src/app/p/[slug]/actions.ts`: `quote`, `hold` (redirects to the checkout), `book` (redirects to the stay page with `?t=<token>`), `sendSlip`. They only call the public API, which checks every rule |
| Pages | `p/[slug]/page.tsx` (photos, description, amenities, rooms, times, house rules, policy, the card; prices a stay a link arrives with), `book/page.tsx` and `checkout-form.tsx` (hold countdown, guest, extras, promo, payment method), `stay/[bookingId]/page.tsx` (status, deadline, bank details, slips; `no-referrer`, not indexed) and `slip-form.tsx`, `embed/` (the compact card opens the page with `unit`, dates, guests and `from=widget`) |
| Components | `availability-calendar.tsx` (two months, stacked in narrow cards; ranges can't cross a taken night), `booking-card.tsx`, `not-taking-bookings.tsx` (the WhatsApp link for a lapsed plan, with the picked dates in the message) |

### Marketing site map

| Area | Files |
| --- | --- |
| Billing | `src/app/billing/page.tsx` (sign in, the accounts the person owns, the plan, `PlanForm`), `actions.ts` (`signIn`, `startCheckout`, `signOutOfBilling`), `done/page.tsx` (reads the plan back after Payments.lk returns), `src/proxy.ts` (refreshes the access cookie before `/billing` renders), `src/lib/api.ts` (`server-only`), `session.ts` (httpOnly cookies scoped to `/billing`). Not indexed, and the apps never link here |

## 4. Running it

```sh
bun run setup          # once: install, Postgres 17 in Docker, migrate, seed
                       # sample owner +94770000001, manager +94772223344, caretaker +94713338899
bun run dev            # API :8080, booking :3001, marketing :3000, Expo :8081
```

- App env (`apps/mobile/.env`): `EXPO_PUBLIC_API_URL=http://<your-LAN-IP>:8080`; `EXPO_PUBLIC_REVENUECAT_APPLE_KEY` and `EXPO_PUBLIC_REVENUECAT_GOOGLE_KEY` for purchases in a development build. Prototype tools show in development builds only.
- Billing: the web billing page is `http://localhost:3000/billing`. With `STAYKEY_PAYMENTSLK_SECRET_KEY` (sk_test_...) and `STAYKEY_PAYMENTSLK_WEBHOOK_SECRET` set, choosing a plan opens a sandbox checkout; the webhook needs the API reachable from Payments.lk (a tunnel to `/webhooks/payments-lk`). Without them, checkout answers 503. To see an account lapse, move `accounts.trial_started_at` back 70 days in SQL and restart the API (the `billing` job runs on start).
- `bun run db:backup-check` dumps the database, restores it into a scratch database and compares row counts. It needs `pg_dump`, `pg_restore` and `psql` of the server's major version (`PG_BIN` points at them).
- API env (`apps/api/.env.example`): in development, photos and logos are stored in `apps/api/.media` and served by the API, and the encryption key is derived from a fixed development value. Production needs `STAYKEY_DATA_KEY` and the `STAYKEY_R2_*` settings.
- Sign-in codes go to the API log, and development builds show the code the dev server returns. Tap "I have an account" with a seeded number to land in that account.
- `cmd/seed` prints an invite for +94715550199 as both `https://staykey.direct/join/<token>` and `staykey://join?token=<token>`. Open the second on a simulator (`xcrun simctl openurl booted '<link>'`) to try joining.
- Push alerts need an EAS project id in `app.json` (`bunx eas-cli init`) and a development build; without one the app skips registration with a single warning. WhatsApp alerts and summaries are written to the API log until `STAYKEY_WHATSAPP_PROVIDER=cloud`.
- Booking page: `http://kingfisher.localhost:3001` (the booking app needs `STAYKEY_API_URL` when the API isn't on `localhost:8080`). Pick dates, continue, book with bank transfer, then send a slip from the stay page; the owner sees "New direct booking" and "Bank slip" in Activity.
- Calendar export: `cmd/seed` prints the villa's link. To try an import, serve a file (`python3 -m http.server 8765` in a folder with an `.ics`), point a feed at `http://127.0.0.1:8765/<file>.ics` in the app's iCal screen and tap "Sync now". Local links work because `STAYKEY_ICAL_ALLOW_PRIVATE` defaults to true outside production. The seed's own feeds are parked until 2100 so their made-up links are never fetched.
- More tab → Prototype tools: switch owner and caretaker, add a sample guesthouse (published to the API as `coralbay`), preview the paywall, restart onboarding.

### Verification

```sh
cd apps/mobile && bunx tsc --noEmit && bun test src && bunx expo export --platform ios
bunx biome check .                       # from the repo root
cd apps/api && go vet ./... && go test ./...   # needs Postgres; CI=true fails instead of skipping
bun run typecheck                        # turbo, all workspaces
bun run generate && git diff --exit-code # generated code is up to date
```

All of these pass at the latest commit (32 app tests; 223 Go tests, most of them integration tests against Postgres 17, behind row-level security), and `cd apps/booking && bun run build` and `cd apps/marketing && bun run build` build. Integration tests skip when Postgres is down, so check the database is up before trusting a green run.

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
- The booking app only calls the API from its server (server components and actions); `STAYKEY_API_URL` must never become a `NEXT_PUBLIC_` variable. With more than one booking app instance, set the same `NEXT_SERVER_ACTIONS_ENCRYPTION_KEY` on each.
- Don't format dates with `Intl` in client components of the booking app: Node and Chromium disagree ("Wed 2 Dec" against "Wed, 2 Dec") and hydration breaks. `src/lib/dates.ts` spells names out.
- An owner booking onto nights a guest is holding at checkout gets a 409 with an empty clash for up to `holdMinutes`. Holds never show on the owner's calendar.
- Playwright doesn't see `window.open(..., "noopener")` as a popup; wait for the context's `page` event instead.
- Superusers and `BYPASSRLS` roles skip row-level security. Production must connect as an ordinary role (the tables' owner is fine, since the policies are forced).
- A table owner can't `pg_dump` with row security off once policies are forced; `scripts/db-backup-check.sh` passes `--enable-row-security`, under which the policies let every row through. Managed provider snapshots are physical and unaffected.
- The `tenant_wall` policy casts the setting with `nullif(..., '')::uuid`. Postgres doesn't short-circuit `OR`, so a plain `::uuid` on an empty setting fails every query.
- RevenueCat's Play product ids carry the base plan (`staykey_villa_monthly:monthly`); `domain.ParseProduct` reads both shapes.
- Rate limits are per API machine and reset on deploy. Booking app and marketing site traffic all comes from their servers, so set `STAYKEY_FORWARD_SECRET` on the API and both sites, or every guest shares one limit.
- Adding `web` to `SubscriptionSource` renamed `oapi.Ios`, `oapi.Android` and `oapi.Web` to `oapi.PushTokenPlatform*`, and `AccountStatus` constants are now `oapi.AccountStatus*`.
- If the API suddenly reports a missing column after a migration, check for an older API process still holding port 8080. Kill it by pid; `pkill -f` patterns can match your own shell.

## 6. Next step: launch readiness

Everything left before launch is in `docs/ROADMAP.md`; detail per phase in `docs/backend-plan.md`. Phase 6 followed `docs/phase-6-plan.md`.

What remains is accounts and launch work rather than a build phase: the Payments.lk merchant account and sandbox keys, the RevenueCat project with App Store and Google Play products named `staykey_<plan>_<monthly|yearly>`, RevenueCat's External Purchases beta, hosting, and the store submissions. Close the phase 6 gate first (below).

**Phase 6 gate status: open, waiting for sandbox accounts.** Checked locally against the seeded API, the booking app and the marketing site (production builds), with a stand-in Payments.lk API: a guest booking on the page moved the sample account to `grace` with "Your first direct booking is in"; on `/billing` the owner signed in, the Guesthouse plan was picked for its units, and "Continue to payment" went to the checkout URL with Rs 5,900 and the checkout's id as reference; a signed `payment.succeeded` made the account `active` with "Guesthouse plan active, Paid until 17 Nov", a repeat and a second event for the same checkout changed nothing, and `/billing/done` showed "Paid until 17 Nov 2026". With the dates moved back, the `billing` job set `lapsed`, the booking page and widget offered "Message Kingfisher Villa on WhatsApp" with the picked dates, and paying again from the lapsed state reopened the page. A RevenueCat `TEST` webhook sent twice was recorded once; a wrong Authorization got 401. Neither customer details nor secrets reached the log. What's left: a StoreKit sandbox or Play test purchase in a development build, and a real Payments.lk sandbox payment with its webhook through a tunnel.

**Phase 5 gate status: done locally.** With the seeded API: a served Airbnb-style feed pointed at the villa's Airbnb link and "Sync now" were read within a minute ("Synced, 1 upcoming stay", Activity "Airbnb calendar connected, 1 stay imported") and the nights showed as taken in the booking page's availability. A second event added to the file came in on the next scheduled read, ten minutes later, with an "Airbnb stay imported" line. `TestAnAirbnbStayClosesItsNightsAfterOneImport` covers the same below the app. In Chromium against the booking app: free dates were held, booked with bank transfer, the stay page showed the ref, bank details and deadline, a slip uploaded, and the owner's Activity listed "New direct booking from Sam Lee" and the slip; a second browser that had the same dates open got "Those dates were just taken". The widget's card opened the page with its dates and `from=widget`. The export lists the seeded stays as "Booked" with no guest names. A real Airbnb link waits for hosting, since Airbnb has to reach the API.

**Phase 4 gate status: done up to the phone.** In a browser against the seeded API: the owner invited a caretaker from Team, WhatsApp opened with the link, the link's join screen showed the account, inviter and role, signing in with the invited number accepted it and landed on Today, and the caretaker's Activity listed the account's stays without payments. Against the running API, a stay the owner added was routed by the `notify` job to the caretaker's push token and sent to Expo's live push service, which answered for the test token and had it removed; the owner's own device got nothing. `TestNewStayAlertsTheCaretakerNotTheOwner` covers the same routing. A push on a real phone waits for an EAS project id and a development build.

**Phase 3 gate status: done.** `TestOneWinnerOutOfFiftyParallelBookings` races 50 bookings on overlapping nights for a room and its whole-house unit; exactly one wins and 49 get a clash, ten runs in a row under `-race`. Checked end to end in a browser against the seeded API: Today loaded the sample stays, approving a request moved it to awaiting payment, a payment confirmed it with the audit trail `requested > awaiting_payment > confirmed`, a clashing new booking showed the overlap, a free one was created and priced by the server (KV-2084) and then cancelled, and a block and a rate override saved and showed on the Calendar. Signing in on a real phone (the phase 1 gate) is still to be done.

## 7. Open questions (defaults if unanswered)

| Question | Default for now |
| --- | --- |
| SMS provider for OTP | oZone Sender for Sri Lankan numbers; international numbers are refused until a second gateway is chosen. Production refuses to start without `STAYKEY_SMS_PROVIDER` |
| Hosting region | Fly.io in Singapore with managed Postgres alongside (`STAYKEY_CLIENT_IP_HEADER=Fly-Client-IP`) |
| File storage | Cloudflare R2, now in use through the S3 API (so swappable). Not yet tried against a real bucket |
| WhatsApp Business number | Deep links only until a number is registered |
| OTA stays | Bookings with an OTA source, total 0, one guest record each (done in phase 5) |
| Row-level security | On (00011) for tenant requests; jobs, webhooks and public routes run with no tenant set |
| Demo data | Development only: `cmd/seed` refuses production |

Calls made without asking, to confirm or change:

- Account creation happens at publish, named after the first property. There is no account rename yet.
- Owners land in their first owned account, team members in the first account they joined. An account switcher waits for phase 4.
- Only owners can add properties, since a property counts towards the plan. Managers get 403.
- `PATCH /v1/me` was added in phase 1 so onboarding can save the owner's name.
- The 30 second refresh grace period exists for retries on flaky mobile networks.
- Phase 2 uses one sectioned `PATCH /v1/properties/{id}` and `POST /v1/properties` with `setup`, instead of a route per sub-resource and `POST /v1/onboarding/publish`. Each settings screen sends only its own section, and the whole save is one transaction.
- Caretakers can read their properties but not the bank details or promo codes, and cannot edit anything.
- Card payments for guests can't be switched on from the app; the app can ask (pending). Guest card payments aren't built; Payments.lk takes the owners' web plan payments only.
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
- Phase 5: imported stays are confirmed bookings with the channel as source, guest "<Channel> stay", 1 adult, total 0. Every event counts, "Not available" blocks included.
- Phase 5: feeds are read every 10 minutes rather than 15 so the gate holds with the job's one-minute tick; failures back off from 10 minutes to 6 hours. A feed shows `error` after 2 failures in a row, and the team hears on the 3rd.
- Phase 5: the first sync of a feed writes one "calendar connected" line; later syncs write one per new or cancelled stay.
- Phase 5: a feed covers the unit it names, else the only unit, else the whole-house unit, else every unit, recorded on the first unit by position.
- Phase 5: an imported stay whose nights are taken is kept without nights and retried on every sync. The team hears once per stay, and only when the clash is with a direct stay or a block.
- Phase 5: an imported stay that leaves its feed before check-out is cancelled; one the owner cancelled stays cancelled even if the event comes back.
- Phase 5: the export lists nights from yesterday on, one event per stay or block per run of nights, and `?unit=` limits it to the nights that unit can't be sold.
- Phase 5: the guest's way back is a token returned once and put in the stay page's link (`?t=`); the page sets `no-referrer`. The idempotent reply keeps the token for 24 hours so a retry gets the same one.
- Phase 5: a booking page guest is matched to an existing guest only when the name matches as well as the phone or email; otherwise they get a guest record of their own. The plan reused any match, but that put a stranger's stay on someone else's record. Stored guests are never changed from the page, and the guest's own booking leaves the guest record out.
- Phase 5: guests must give a phone number; email and country are optional. `Idempotency-Key` is required on guest bookings.
- Phase 5: request mode makes a request that expires after `replyHours`; instant mode waits for payment, with a deadline only for bank transfer when the property cancels unpaid stays. Paying at the property has no deadline, and approving such a request sets none.
- Phase 5: the page shows but takes no holds or bookings (409 `not_taking_bookings`) when the property is paused or has neither bank transfer nor paying at the property on. Guest card payments are not built.
- Phase 5: promo codes on the page must be in date, under their limit and meet their minimum nights, and a booking uses a go of the code in its own transaction.
- Phase 5: the booking app's server action body limit is 11 MB, a little over the 10 MB slip limit, for the form's multipart framing.
- Phase 5: server actions check their inputs by hand rather than with Zod, which the repo doesn't use; the public API checks every rule again.
- Phase 5: `STAYKEY_ICAL_ALLOW_PRIVATE` defaults to true in development and is refused in production.
- Phase 6: Payments.lk turned out to be a REST API (`POST /v1/checkouts` with a bearer secret key, signed webhooks), documented through its `@payments-lk/node` library, rather than Payable's form-post IPG the plan expected. The client follows that library: checkouts keyed by our checkout id as `Idempotency-Key`, a `Payments-Signature` of `t=<unix>,v1=<HMAC-SHA256 of "t.body">`.
- Phase 6: Payments.lk charges only in LKR, so web prices are set in rupees in `domain.Plans`: Villa Rs 2,900 a month or Rs 29,000 a year, Guesthouse Rs 5,900 or Rs 59,000, Collection Rs 11,900 or Rs 119,000 (about the store prices at Rs 300 to the dollar). Change them there; `GET /v1/public/plans` serves them.
- Phase 6: web purchases are prepaid periods with no renewal, although Payments.lk now offers subscriptions; those need its Advanced plan and saved cards.
- Phase 6: checkout returns a URL to redirect to, not form fields, and the web page lives on the marketing site with its own phone sign-in. The apps never link to it.
- Phase 6: `canPurchase` is true for owners while no paid plan gives access; the web page still lets an active web plan pay for the next period.
- Phase 6: refresh and webhooks re-read RevenueCat and order rows by when they were read. A different source replaces the plan only when it gives access for longer, so a late event about an old store plan can't undo a web period.
- Phase 6: the shared forward secret is `STAYKEY_FORWARD_SECRET` (the plan said `STAYKEY_BOOKING_FORWARD_SECRET`), since the marketing site forwards owners' addresses for sign-in code limits too. The sites take the first `x-forwarded-for` address, so they must sit behind a host that sets that header.
- Phase 6: lapsing changes only the booking page; the owner app keeps working. Plan size only nudges (the Today banner and the paywall's "Fits your property"); nothing is blocked.
- Phase 6: billing dates in activity lines are written in Sri Lanka's time, since accounts have no time zone of their own.
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
