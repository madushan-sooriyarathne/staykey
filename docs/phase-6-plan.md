# Build phase 6: billing and hardening

Drafted 17 Oct 2026 on top of the phase 5 branch (`feat/phase-5-calendar-sync-booking-page`, PR #3). Merge PR #3 before starting. Written for the agent that executes it; the wider plan is `docs/backend-plan.md`, the checklist `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

Built on 17 Oct 2026 (branch `feat/phase-6-billing-hardening`). Where the build differs from this plan, mainly Payments.lk's API and rupee prices, the handover's "Calls made without asking" says why.

## Goal

An owner who buys a plan, in the app through RevenueCat or on the web through Payments.lk, ends their free period on the server. An account whose free period and grace run out turns its booking page into "message the owner on WhatsApp". Public and owner routes are rate limited, tenant queries are walled by Postgres row-level security, and a backup restores cleanly.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- **Payment providers:** RevenueCat for App Store and Google Play subscriptions; Payments.lk (Payable's gateway) for web purchases. PayHere is dropped everywhere. Guest card payments at the booking page are not part of this phase.
- **Web purchases are prepaid periods:** one charge buys one month or one year, with no automatic renewal. The owner renews from the web billing page. This avoids depending on Payments.lk's recurring and cancellation APIs, which aren't confirmed yet. Renewing while a web period is running starts the new period when the current one ends.
- **Web prices are the list prices in USD** (Villa $9 / $90, Guesthouse $19 / $190, Collection $39 / $390), the same as the stores.
- **Web purchases also go to RevenueCat** through its External Purchases API (private beta) when `STAYKEY_REVENUECAT_EXTERNAL_PURCHASES=true`, so RevenueCat sees all revenue. The server stays the source of truth for access either way; the app never reads entitlements from RevenueCat.
- **One RevenueCat customer per account:** the app calls `Purchases.logIn(accountId)`. One entitlement, `staykey`; the plan and period come from the product id (`staykey_<plan>_<monthly|yearly>`, for example `staykey_guesthouse_yearly`).
- **The free period:** free from `accounts.trial_started_at` until the first direct booking, at most 60 days. Then 7 days' grace, then lapsed. A paid plan that ends (store expiry, web period over) also gets 7 days' grace. The end of free access is the later of the free period's end and the last paid period's end.
- **The first direct booking** is the first booking with source `page` or `widget` that reaches `awaiting_payment` or `confirmed`: created that way in instant mode, or an approved request. Declined requests don't count. Migration 00010 backfills it from existing bookings.
- **Statuses:** `trial`, `active`, `grace`, `lapsed` in `accounts.status`. `closed` is never set by billing. The booking page takes holds and bookings in `trial`, `active` and `grace`; `lapsed` shows the WhatsApp message. The owner app keeps working in every status.
- **Plan limits warn, never block:** `GET /v1/subscription` returns the account's bookable units and the plan that fits; the app nudges when the paid plan is smaller. Nothing refuses a unit or property.
- **Billing alerts** are activity of a new kind, `billing`, seen only by roles with `PermBilling` (owners). They follow the `booking` alert preference.
- **Rate limits are in memory per API machine**, fixed windows. Good enough for one or two machines; a shared store waits until there are more.
- **Row-level security is a second wall for tenant requests only:** policies match `account_id` to a per-connection setting that is set when the request context carries a tenant, and allow everything when it is empty (auth, public routes, webhooks, jobs, the seed).
- **The web billing page lives on the marketing site** (`staykey.direct/billing`) with its own phone sign-in. The apps never link to it, since the App Store and Google Play don't allow pointing people to outside payment for subscriptions.
- **Gate:** a sandbox purchase in a development build (StoreKit sandbox or a Play test track) through RevenueCat, and a Payments.lk sandbox payment on the web, each move a trial account to `active`. Real store purchases wait for the store and RevenueCat accounts.

## Context

- `accounts` (`00002_create_accounts_and_people.sql:17`) already has `status` (`trial`, `active`, `grace`, `lapsed`, `closed`), `trial_started_at` and `first_direct_booking_at`; nothing writes the last one or changes `status` yet.
- `domain.Account` (`internal/domain/account.go:104`) carries `Status` and `TrialStartedAt`; `PermBilling` exists and only owners have it (`account.go:37`, `Role.Can`).
- `oapi.Account` returns `status` and `trialStartedAt` (`server/accounts.go:accountToAPI`).
- App billing today is a stub: `apps/mobile/src/lib/purchases.ts` (`PLANS`, `planFor`, `purchase`, `restore` with TODOs), `src/lib/session.ts` keeps `subscription`, `subscribe` and `startedAt` on the device, `src/app/paywall.tsx` and `src/app/subscription.tsx` read them, `src/app/(tabs)/more.tsx:28` too. `TRIAL_DAYS = 60` is hard-coded in `subscription.tsx`.
- `src/lib/prototype.ts`: `PROTOTYPE = __DEV__ || EXPO_PUBLIC_PROTOTYPE_TOOLS === "true"`.
- Bookable units: the app's `physicalUnits(p)` (`src/data/pricing.ts:255`) counts units without `linkedUnitIds` when the property has both, else all units.
- Booking page: `takingBookings(p)` in `server/public.go` is `p.Status != "paused" && (bank || atProperty)`; `errPaused` is 409 `not_taking_bookings`. Guest bookings are created in `store.CreateGuestBooking` (`store/public.go`); requests are approved in `store.TransitionBooking` (`store/bookings.go`).
- Activity: kinds are checked in SQL (`00008_create_activity_and_messages.sql:14`), in the spec (`ActivityKind`), in `domain/activity.go` (`HiddenFromCaretakers`, `VisibleTo`, `EventFor`) and in the app (`src/app/activity.tsx:18` groups and icons). Recipients are filtered in `store.alertFor` (`store/activity.go:195`) through `Kind.VisibleTo`.
- Jobs: periodic sweeps in `internal/jobs/jobs.go` `sweeps()`; network work goes outside transactions (see `ImportFeeds` in `internal/jobs/import.go`).
- Security: each route's requirements come from the spec (`server/security.go`, `requirementsFrom`); a route missing from the spec is refused. The tenant is put in the context with `tenant.With` and read with `tenant.From` (`internal/tenant`). The client address comes from `withClientIP` (`server/security.go:155`) and `STAYKEY_CLIENT_IP_HEADER`.
- The only rate limits today are the sign-in code limits in `internal/auth/otp.go` (`checkSendLimits`), counted in Postgres. The 429 shape exists (`oapi.TooManyRequestsJSONResponse`, `apiError.retryAfter` in `server/errors.go`).
- The pool is created in `store.NewPostgresWithConfig` (`store/postgres.go:48`); pgx is v5.11.0, whose `pgxpool.Config.PrepareConn` gets the acquiring context.
- Tables with `account_id`: activity, activity_reads, block_units, blocks, booking_events, booking_extras, booking_lines, bookings, charges, extras, guests, ical_feeds, idempotency_keys, invites, length_discounts, membership_properties, memberships, message_templates, notification_prefs, payment_settings, payment_slips, payments, promos, properties, property_photos, rate_overrides, season_prices, seasons, unit_links, unit_nights, units.
- The booking app calls the API only from its server (`apps/booking/src/lib/api.ts`, `createStayKeyClient`). The marketing site (`apps/marketing`, Next 16) has only `/` and `/join/[token]`, and no API client dependency yet.
- Payments.lk is Payable's gateway, launched September 2026. Payable's existing IPG takes a hosted checkout with `paymentType` 1 (one-time), a SHA-512 `checkValue` over merchant key, invoice id, amount and currency keyed with the merchant token, and a server-to-server notification with `statusCode`, `payableOrderId`, `payableAmount` and `invoiceNo`. Whether Payments.lk uses the same fields is not confirmed.
- RevenueCat webhooks send an optional `Authorization` header set in the dashboard, retry up to 5 times, deduplicate by `event.id`, and RevenueCat recommends calling `GET /v1/subscribers/{app_user_id}` after each one instead of reading the event's fields.

## Constraints

- Work in this order, one commit per step: migrations, sqlc queries, domain with tests, OpenAPI, `bun run generate`, store, handlers, jobs, marketing billing page, booking app, mobile, docs.
- No em dashes anywhere: code, comments, UI copy, commit messages.
- Never edit `api.gen.go`, `internal/store/queries` or `packages/api-client/src/schema.ts` by hand.
- Add migrations `00010` (billing) and `00011` (row-level security) only, each with a working `Down`. Never change `00001` to `00009`.
- Money is integer minor units with a currency. Every business table carries `account_id`; owner store methods take a `tenant.Tenant`. Webhook and job store methods take the account id from the event or row they act on.
- Wrap Go errors with `%w`, never discard one with `_`, pass `context.Context` first, keep initialisms in one case.
- The server has no request validator; check what the spec states in handlers or domain code.
- Never log secrets, webhook bodies, card details or tokens. Log event ids and account ids.
- Never hold a transaction open across a network call: RevenueCat and Payments.lk calls happen outside transactions.
- Compare webhook secrets and signatures in constant time.
- Every new environment variable goes in `apps/api/.env.example` (or the app's `.env.example`) and in `passThroughEnv` for the `dev` task in `turbo.json`.
- In `apps/mobile`, add packages with `bunx expo install`. Read the Expo SDK 57 docs and RevenueCat's Expo guide before wiring `react-native-purchases`.
- Next 16: read `apps/marketing/AGENTS.md` (and the guides it points to in `node_modules/next/dist/docs/`) before writing server actions, cookies or route handlers.
- Tests check behaviour against Postgres 17 through `storetest.New(t)`, with fixed clocks and `httptest` servers for RevenueCat and Payments.lk.

## Changes

### 0. Before writing provider code
- Get the Payments.lk merchant sandbox credentials and API documentation from Payable. Confirm the checkout URL, the request fields, the `checkValue` formula, the notification's transport (form or JSON), its fields and how to verify it, and whether USD is accepted.
- Request access to RevenueCat's External Purchases API (private beta).
- If the Payments.lk documentation isn't available, build steps 1 to 7 and 9 to 11 with the Payments.lk client behind the interface in step 4 and a fake in tests, leave the sandbox gate open, and say so in the handover.

### 1. `apps/api/internal/db/migrations/00010_create_billing.sql` (new)
- `subscriptions`: `account_id` (PK, references accounts, cascade), `source` (`app_store`, `play_store`, `web`), `plan` (`villa`, `guesthouse`, `collection`), `period` (`monthly`, `yearly`), `status` (`active`, `cancelled`, `billing_issue`, `expired`), `will_renew` bool, `current_period_end` timestamptz, `product_id` text, `external_id` text, `last_event_at` timestamptz, `created_at`, `updated_at` (with the `set_updated_at` trigger).
- `billing_events`: `id`, `account_id` (nullable, references accounts, set null), `source` (`revenuecat`, `paymentslk`), `event_id` text, `type` text, `payload` jsonb, `received_at`; unique `(source, event_id)`.
- `web_checkouts`: `id` uuid (the invoice number sent to Payments.lk), `account_id`, `plan`, `period`, `amount` bigint, `currency`, `status` (`pending`, `paid`, `failed`), `period_start`, `period_end` (null until paid), `provider_order_id`, `created_at`, `paid_at`.
- Allow `billing` in the `activity.kind` check.
- Backfill `accounts.first_direct_booking_at` with the earliest `created_at` of bookings with source `page` or `widget` and status not in (`requested`, `declined`).
- `Down` drops the tables, restores the old kind check after deleting `billing` activity, and leaves `first_direct_booking_at` as it is.

### 2. `apps/api/internal/db/migrations/00011_enable_row_level_security.sql` (new)
- For each table in the Context list: `ENABLE ROW LEVEL SECURITY`, `FORCE ROW LEVEL SECURITY`, and one policy `tenant_wall` for all commands:
  `USING (coalesce(current_setting('staykey.account_id', true), '') = '' OR account_id = current_setting('staykey.account_id', true)::uuid)` with the same `WITH CHECK`.
- Add `subscriptions`, `web_checkouts` and `billing_events` (where `account_id` is set) the same way.
- `Down` drops the policies and disables RLS on each table.

### 3. sqlc queries
- `apps/api/internal/db/queries/billing.sql` (new):
  - `GetSubscription`, `UpsertSubscription` (only applies when `last_event_at` is newer), `ExpireSubscription`.
  - `InsertBillingEvent :execrows` with `ON CONFLICT (source, event_id) DO NOTHING`.
  - `InsertWebCheckout`, `GetWebCheckoutForUpdate`, `MarkWebCheckoutPaid`, `MarkWebCheckoutFailed`.
  - `GetAccountBilling`: status, `trial_started_at`, `first_direct_booking_at`, the subscription columns, and the owner's phone (membership role `owner`).
  - `ListAccountsForBilling`: the same for every account not `closed`.
  - `SetAccountStatus :execrows` (only when it changes).
  - `MarkFirstDirectBooking :execrows`: sets `first_direct_booking_at` when it is null.
  - `BookableUnits`: per account, the count the app's `physicalUnits` would give, summed over active properties and units.
- Run `bun run generate`.

### 4. Domain
- `apps/api/internal/domain/billing.go` (new):
  - `PlanID`, `Plans` (`villa` 1 unit, `guesthouse` 6, `collection` 20, with monthly and yearly prices in US cents), `PlanFor(units int) PlanID`, `ParseProduct(productID string) (PlanID, Period, bool)`.
  - `Subscription` (source, plan, period, status, will renew, period end).
  - `FreeDays = 60`, `GraceDays = 7`.
  - `BillingState(now, trialStartedAt time.Time, firstDirectAt *time.Time, sub *Subscription) (AccountStatus, until time.Time)` implementing the rules in Assumptions.
  - `TakesBookings(AccountStatus) bool` (false only for `lapsed`).
- `apps/api/internal/domain/activity.go`: `ActivityBilling`; `VisibleTo` hides it from roles without `PermBilling`; `EventFor(ActivityBilling) = EventBooking`. Builders, with no amounts:
  - `FirstDirectBookingIn(until)`: "Your first direct booking is in", "Free until 24 Oct, then choose a plan".
  - `FreePeriodEnded(until)`: "Your free period has ended", "Choose a plan by 24 Oct to keep your booking page open".
  - `PlanEnded(until)`: "Your plan has ended", "Renew by 24 Oct to keep your booking page open".
  - `BookingPagePaused()`: "Your booking page is paused", "Guests are asked to message you on WhatsApp until you choose a plan".
  - `PlanActive(plan, until)`: "Guesthouse plan active", "Paid until 17 Nov".
- `apps/api/internal/billing/` (new package):
  - `revenuecat.go`: `RevenueCat{SecretKey, BaseURL, HTTP}` with `Subscriber(ctx, appUserID) ([]domain.Subscription, error)` reading `GET /v1/subscribers/{app_user_id}` (store subscriptions only: `app_store`, `play_store`; check field names against RevenueCat's docs), and `RecordExternal(ctx, ...)` posting a paid web period to `POST /v1/receipts/external` when enabled.
  - `paymentslk.go`: `PaymentsLK{MerchantID, MerchantKey, MerchantToken, Env, ...}` with `Checkout(order) (url string, fields map[string]string)` and `Verify(r *http.Request) (Notification, error)`, written to the confirmed docs from step 0.
  - `Provider`-style interfaces only where the server needs a fake in tests: `SubscriberSource` and `WebGateway`, one method set each.
- Tests: `billing_test.go` (table of `BillingState` cases: in trial, first direct booking, 60 days, grace, lapsed, active, cancelled but in period, store expiry then grace, paid end later than free end), `activity_test.go` cases, `billing/*_test.go` against `httptest` servers and fixture payloads.

### 5. `packages/api-spec/openapi.yaml`
- `ActivityKind` gains `billing`.
- `GET /v1/subscription` (`bearerAuth` plus `accountHeader`, every role) returns `Subscription`: `status`, `until`, `freeEndsAt` (nullable), `plan`, `period`, `source`, `willRenew`, `units`, `fits` (plan id), `canPurchase` (owner and no active subscription from another source).
- `POST /v1/subscription/refresh` (`PermBilling`) reads RevenueCat for the account and returns `Subscription`.
- `POST /v1/subscription/checkout` (`PermBilling`) takes `{plan, period}` and returns `{url, fields}` for an auto-submitting form. 409 `already_subscribed` while a store subscription is active; 503 `billing_not_configured` without Payments.lk settings.
- `POST /webhooks/revenuecat` and `POST /webhooks/payments-lk`, with no `security` (checked in the handlers), answering 200.
- `PublicProperty` gains `pausedReason` (`lapsed`, `paused`, `no_payment_method`, nullable) and `whatsapp` (the owner's number, only when `pausedReason` is `lapsed`).
- Run `bun run generate`.

### 6. Store
- `apps/api/internal/store/billing.go` (new):
  - `Subscription(ctx, t, now)`: the account's state from `GetAccountBilling` and `BillingState`, plus `BookableUnits`.
  - `ApplyStoreSubscriptions(ctx, accountID, subs, eventAt, now)`: upserts the newest store subscription, then `recomputeBilling`.
  - `RecordBillingEvent(ctx, source, eventID, accountID *uuid.UUID, type, payload) (isNew bool, err)`.
  - `CreateWebCheckout(ctx, t, plan, period, amount, currency, now)`: refuses while a store subscription gives access; the period starts at the later of now and the current web period's end.
  - `PayWebCheckout(ctx, invoiceID, providerOrderID, amount, currency, now)`: locks the checkout, ignores a second notification, checks the amount and currency, sets the web subscription's period end, then `recomputeBilling`.
  - `FailWebCheckout`.
  - `recomputeBilling(ctx, accountID, now)`: `BillingState`, `SetAccountStatus`, and on a change the matching activity line with no actor.
  - `RecomputeAllBilling(ctx, now) (changed int, err)` for the job.
- `apps/api/internal/store/public.go`: `CreateGuestBooking` calls `MarkFirstDirectBooking` when the new booking is `awaiting_payment` or `confirmed`, then `recomputeBilling`. `PublicProperty` also returns the account's status and owner phone (add them to `domain.PropertyDetail` or a small struct beside it; never to owner responses).
- `apps/api/internal/store/bookings.go`: `TransitionBooking`, when a `page` or `widget` request moves to `awaiting_payment` or `confirmed`, does the same.
- `apps/api/internal/store/postgres.go`: in `NewPostgresWithConfig`, set `cfg.PrepareConn` to run `SELECT set_config('staykey.account_id', $1, false)` with the tenant's account id when `tenant.From(ctx)` finds one, and with `''` otherwise. Keep `storetest` working (it builds its pool through the same function, or set the hook there too).
- Tests:
  - `billing_test.go`: a guest booking ends the trial into grace; a declined request doesn't; a store subscription makes the account active and writes one activity line; an older event doesn't override a newer one; a paid web checkout extends from the current end; a second notification for the same checkout changes nothing; a checkout while a store plan is active is refused.
  - `rls_test.go`: with a tenant in the context, a raw `SELECT` of another account's booking through the pool returns no rows, and an `INSERT` with another account's id fails; without a tenant both work.
  - `TestTenantIsolation` and the whole store suite still pass with RLS on.

### 7. Handlers
- `apps/api/internal/server/billing.go` (new): `GetSubscription`, `RefreshSubscription` (RevenueCat call outside any transaction, then `ApplyStoreSubscriptions`), `CreateCheckout` (amount from `domain.Plans`, fields from `WebGateway.Checkout`, return URL `STAYKEY_BILLING_RETURN_URL`).
- `apps/api/internal/server/webhooks.go` (new):
  - RevenueCat: compare `Authorization` with `STAYKEY_REVENUECAT_WEBHOOK_AUTH` in constant time (401 otherwise); parse `event.id`, `event.type`, `event.app_user_id`; `RecordBillingEvent` (a duplicate returns 200 at once); ignore ids that aren't account uuids and `TEST` events; call `Subscriber` and `ApplyStoreSubscriptions`. Answer within RevenueCat's 60 seconds.
  - Payments.lk: `Verify`, `RecordBillingEvent`, then `PayWebCheckout` or `FailWebCheckout`; after a payment, `RecordExternal` when enabled, logging (not failing) its errors.
- `apps/api/internal/server/public.go`: `takingBookings` also needs `domain.TakesBookings(status)`; set `pausedReason` and, for `lapsed`, `whatsapp`. Holds and bookings on a lapsed account get 409 `not_taking_bookings`.
- `apps/api/internal/server/limits.go` (new) and `apps/api/internal/ratelimit/` (new, fixed window counters keyed by string, with expiry of old windows):
  - a middleware in `Handler()`'s `Middlewares`, before `secure`, choosing a limit by `r.Pattern`:
    - holds: 10 per 10 minutes per client address;
    - guest bookings: 5 per 10 minutes per address;
    - guest uploads and slips: 10 per hour per booking id;
    - other `/v1/public/` routes and `/ical/`: 120 per minute per address;
    - owner routes: 300 per minute per user (read the bearer token's user after `secure`, so put this one after it);
    - webhooks and `/v1/auth/` (already limited): none.
  - 429 `rate_limited` with `Retry-After`.
  - `withClientIP`: when `X-StayKey-Forward-Key` equals `STAYKEY_BOOKING_FORWARD_SECRET` (constant time), use `X-StayKey-Guest-IP` as the client address.
- `apps/api/internal/server/server.go`: the new store methods in `Store`; `Options` gains the billing clients.
- Tests: subscription read for a caretaker; refresh and checkout are 403 for managers; RevenueCat webhook with a wrong header is 401, a duplicate is applied once; a Payments.lk notification that fails verification changes nothing; a lapsed property's hold is 409 and its public property has `whatsapp`; the sixth guest booking from one address in 10 minutes is 429; a forwarded address without the secret is ignored.

### 8. Jobs and config
- `apps/api/internal/jobs/jobs.go`: add `billing` every 15 minutes calling `RecomputeAllBilling`.
- `apps/api/internal/config/config.go`: `RevenueCat{SecretKey, WebhookAuth, ExternalPurchases}`, `PaymentsLK{MerchantID, MerchantKey, MerchantToken, Env}`, `BillingReturnURL`, `BookingForwardSecret`. All optional; the routes that need them answer 503 `billing_not_configured` without them. Production refuses `STAYKEY_PAYMENTSLK_ENV=sandbox`.
- `apps/api/cmd/api/main.go`: build the clients and pass them in.
- `apps/api/.env.example`, `turbo.json`: every new variable.
- `apps/api/cmd/seed/main.go`: leave the sample account in its trial.
- `scripts/db-backup-check.sh` (new) and `"db:backup-check"` in the root `package.json`: `pg_dump -Fc` from `DATABASE_URL`, restore into a fresh scratch database, compare row counts of `accounts`, `properties`, `bookings`, `unit_nights` and `activity`, drop the scratch database, exit non-zero on any difference.

### 9. Marketing site (`apps/marketing`)
- Add `@staykey/api-client` and `server-only` to its dependencies.
- `src/lib/api.ts` (new, `server-only`): client for `STAYKEY_API_URL`; access and refresh tokens in httpOnly, `Secure`, `SameSite=Lax` cookies scoped to `/billing`.
- `src/app/billing/page.tsx` and `actions.ts` (new): phone, then code (`/v1/auth/otp`, `/v1/auth/verify`), then the accounts the person owns, the plan that fits (`GET /v1/subscription`), monthly or yearly, then `POST /v1/subscription/checkout` and an auto-submitting form to Payments.lk.
- `src/app/billing/done/page.tsx` (new): reads `GET /v1/subscription` and shows "Paid until ..." or "We haven't heard from the bank yet" with a refresh link.
- `robots: { index: false }` on both pages. Brand tokens only.

### 10. Booking app (`apps/booking`)
- `src/lib/api.ts`: an openapi-fetch middleware that adds `X-StayKey-Forward-Key` (`STAYKEY_BOOKING_FORWARD_SECRET`) and `X-StayKey-Guest-IP` (first address in `x-forwarded-for`, else `x-real-ip`, from `headers()`), only when the secret is set.
- `src/app/p/[slug]/page.tsx` and `embed/embed-bridge.tsx`: when `pausedReason` is `lapsed`, replace the card with "Message <property> on WhatsApp" linking to `https://wa.me/<digits>` with the picked dates in the text if any; other paused reasons keep today's message.
- A 429 from the API shows "Too many tries. Please wait a few minutes." through the existing problem message.
- `.env.example` (new) with `STAYKEY_API_URL` and `STAYKEY_BOOKING_FORWARD_SECRET`.

### 11. Mobile app
- `bunx expo install react-native-purchases`.
- `src/lib/purchases.ts`: keep `PLANS`, `planFor`, `priceLabel`; replace the stubs with RevenueCat: `configurePurchases()` with `EXPO_PUBLIC_REVENUECAT_APPLE_KEY` or `EXPO_PUBLIC_REVENUECAT_GOOGLE_KEY`, `identify(accountId)` (`Purchases.logIn`), `purchase(plan, period)` finding the package by product id, `restore()`. Without a key (web, Expo Go, missing env) they return `{ ok: false, message: "Purchases aren't available in this build." }`.
- `src/api/subscription.ts` (new): `useSubscription()` (`GET /v1/subscription`, keyed by account), `useRefreshSubscription()`.
- `src/lib/session.ts`: remove `subscription`, `subscribe` and `startedAt`; bump the persist `version` and drop those keys in a `migrate`. Bump the query cache `buster` in `src/api/query-client.ts` if any cached shape changes.
- `src/features/auth/enter.ts` (`openAccount`): call `identify(accountId)`.
- `src/app/paywall.tsx`: read `useSubscription()`; buy with `purchase`, then refresh and close once `status` is `active`; restore the same way; managers and caretakers see "Ask the account owner to choose a plan"; when `canPurchase` is false because a web plan is active, say "Your plan is paid until <date>".
- `src/app/subscription.tsx`: everything from `useSubscription()` (status, until, free end, plan, units against the plan); remove `TRIAL_DAYS`; "Manage" opens the store's subscription page for store plans only; a web plan shows "Paid until <date>" with no link.
- `src/app/(tabs)/index.tsx` (`OwnerToday`): a banner for owners in `grace` or `lapsed`, or when `fits` is larger than the paid plan, opening the paywall.
- `src/app/(tabs)/more.tsx`: plan row from `useSubscription()`.
- `src/app/activity.tsx`: an icon and group for `billing`.
- `src/lib/prototype.ts`: `PROTOTYPE = __DEV__`; remove `EXPO_PUBLIC_PROTOTYPE_TOOLS` from `.env.example` and the handover.

### 12. Docs
- `docs/ROADMAP.md`: tick phase 6 items; replace PayHere with Payments.lk in the owner accounts list.
- `docs/HANDOVER.md`: commits, maps (billing, limits, RLS, marketing billing page), the calls above, gotchas, gate status, production list (RevenueCat keys and webhook secret, Payments.lk credentials, the forward secret, `NEXT_SERVER_ACTIONS_ENCRYPTION_KEY`).
- `docs/backend-plan.md`: the status line and PayHere mentions.
- `README.md`: billing and how to try it in sandbox, the backup check.

## Do not

- Take guest card payments at the booking page, or build a merchant connection for owners. `payments.cards` stays as it is.
- Build recurring web billing or web cancellation. Prepaid periods only, until Payments.lk's recurring API is confirmed.
- Link to web billing from the app or show web prices there.
- Block units, properties or bookings because of the plan size.
- Change the owner app's behaviour when the account is lapsed; only the booking page changes.
- Read entitlements from RevenueCat in the app to decide access. The server decides.
- Add a shared rate limit store (Redis or Postgres counters) or a WAF.
- Add an email provider or guest emails.
- Build the Next.js owner dashboard; the billing page is the only web surface.
- Move bookings, payments or slips between accounts, or add account deletion.

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...` -> passes on Postgres 17, including `TestTenantIsolation`, `TestOneWinnerOutOfFiftyParallelBookings` 10 times, and the RLS tests.
- `bun run generate && git diff --exit-code` -> no diff.
- `bunx biome check . && bun run typecheck` -> clean.
- `cd apps/mobile && bun test src && bunx expo export --platform ios` -> passes.
- `cd apps/booking && bun run build` and `cd apps/marketing && bun run build` -> build.
- Migrations `00010` and `00011` up, down and up on a seeded database.
- `bun run db:backup-check` on the seeded database -> counts match, exit 0.
- Server checks against the seeded API:
  - a guest booking on the page moves the sample account to `grace` with "Your first direct booking is in" in Activity;
  - with the clock past grace (or `trial_started_at` moved back in SQL), the `billing` job sets `lapsed` and the booking page shows the WhatsApp link;
  - a signed RevenueCat test webhook for the account is recorded once.
- Gate, with sandbox accounts: a StoreKit sandbox or Play test purchase in a development build turns the account `active` within a minute (webhook through a tunnel, or "Restore" calling refresh); a Payments.lk sandbox payment from `staykey.direct/billing` (marketing site on `localhost:3000`, notification URL through a tunnel) does the same, and the booking page takes bookings again.

## Risks

- **Payments.lk's API differs from Payable's IPG, or isn't documented yet.** -> Step 0 first; keep it behind `WebGateway`; test with fixtures from the real sandbox, not invented ones.
- **RevenueCat's External Purchases API is a private beta.** -> It is off unless enabled, and its failures never block a web payment from counting.
- **Webhooks arrive twice or out of order.** -> `billing_events` deduplicates by event id; the subscription upsert only applies newer `last_event_at`; a paid checkout is locked and marked once.
- **A forged webhook grants access.** -> Constant-time header check for RevenueCat, signature check for Payments.lk, and the RevenueCat path re-reads the subscriber from RevenueCat rather than trusting the body.
- **RLS hides rows a tenant request legitimately needs, or a query outside a tenant context runs with a stale setting.** -> `PrepareConn` sets the value on every acquire, empty when there is no tenant; the full suite runs with RLS on.
- **Store review rejects the app for mentioning outside payment.** -> No web billing link, price or wording in the app.
- **Owners lose bookings when a plan lapses.** -> 7 days' grace with an alert at its start, the Today banner, and the WhatsApp fallback instead of a dead page.
- **In-memory limits reset on deploy and multiply with machines.** -> Acceptable for launch; noted in the handover.

## Went-sideways flags

- `TestTenantIsolation` or the store suite starts failing with empty results or `new row violates row-level security policy` (the RLS setting isn't cleared, or a tenant path reads another account's rows on purpose).
- The same RevenueCat event or Payments.lk notification shows twice in Activity, or extends a web period twice.
- The booking page of a `grace` account refuses bookings, or a `lapsed` one still takes them.
- The app shows "active" after a purchase while `GET /v1/subscription` says otherwise (the app is reading RevenueCat instead of the server).
- A webhook body, card detail or token appears in the API log.
