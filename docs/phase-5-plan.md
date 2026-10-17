# Build phase 5: calendar sync and the booking page

Drafted 11 Oct 2026 and checked against `main` at `6e9d00e` on 17 Oct 2026. Written for the agent that executes it; the wider plan is `docs/backend-plan.md`, the checklist `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

A stay added to an owner's Airbnb calendar closes those nights on their StayKey booking page within 15 minutes, and a guest can pick dates on the booking page or widget, hold them, book, see their booking and upload a bank slip, all on live data, with the owner's calendar exported for the OTAs to import.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- **Imported stays are bookings** with the channel as their source, the open question's default. Each one is `confirmed`, with guest name "<Channel> stay", 1 adult, a total of 0, and `feed_id` and `external_uid` set. Every VEVENT counts, including "Not available" blocks.
- **Feeds sync every 10 minutes**, not 15, so the gate holds with the job's one minute tick and fetch time. Failures back off: 10 m, 20 m, 40 m and so on, capped at 6 h.
- **Feed status:** a feed shows `error` after 2 failures in a row. The team gets one "sync failed" alert, on the 3rd failure in a row.
- **First sync of a feed is quiet:** it imports silently and writes a single activity line ("Airbnb calendar connected, 12 stays imported"). Later syncs write one line per new or cancelled stay.
- **Feed to unit mapping:** a feed may name a unit (`unitId`). Without one it covers the whole property:
  - a property with a single unit uses that unit;
  - otherwise it uses the whole-house unit when there is one;
  - otherwise it holds every active unit.
- **Clashes with an imported stay:** when the stay's nights are already taken, the booking is still saved but holds no nights. Every later sync retries the hold. The team gets an alert only when the clash is with a stay that wasn't itself imported; a clash between two imports of the same dates is an OTA echo and stays quiet.
- **Stays removed from a feed:** an imported stay that leaves its feed and hasn't checked out yet is cancelled. An imported stay the owner cancelled stays cancelled even if its event remains in the feed.
- **Export:**
  - `GET /ical/{slug}-{token}.ics` lists every held night, collapsed into one event per stay or block per run of nights, titled "Booked" or "Not available", with no guest details.
  - `?unit=<id>` limits it to the nights that unit can't be sold, through `domain.LedgerUnits`.
  - Events use UIDs ending `@staykey.direct`, and import skips those UIDs so StayKey never re-imports its own export.
- **Holds:** a hold is ledger rows with a random `hold_id` and `hold_expires_at` (the property's `holdMinutes`). Booking with a `holdId` swaps the hold's rows for the booking's in one transaction. The existing `sweep_expired` job clears expired holds, and `holdNights` clears expired holds on the units it is about to take, so a stale hold never blocks a booking.
- **What a guest booking becomes:**
  - `request` mode: `requested`, expiring after `replyHours`.
  - `instant` mode with bank transfer: `awaiting_payment`, with `payment_due_at` set when the property cancels unpaid stays.
  - `instant` mode paying at the property: `awaiting_payment` with no deadline, which matches phase 3's check-in rule.
  - Cards wait for PayHere in phase 6.
- **Guest access:** the guest's way back to their booking is a token returned once at booking time. Its hash is stored on the booking, and it travels in the `X-Booking-Token` header. The booking page puts it in the stay page's URL (`?t=`) so the guest can bookmark it.
- **The booking app talks to the API only from the server** (server components and server actions), so `STAYKEY_API_URL` stays private and no browser calls the API. Slips go through a server action that uploads to the presigned URL itself.
- **Slip upload size:** the booking app's server action body limit rises to 10 MB to match `files.MaxBytes[KindSlip]`.
- **Promo codes on the booking page** must be in date, under their limit and meet their minimum nights. A booking with a code increases the promo's `used_count` in the same transaction, and a code that hit its limit in the meantime is refused.
- **Bookings from the widget** carry source `widget`: the embed adds `from=widget` to the link it opens. Bookings from the page itself carry `page`.
- **Taking bookings:** the page shows but refuses holds and bookings with 409 `not_taking_bookings` when the property's `status` is `paused`, or when it offers no way to pay (bank transfer off and paying at the property off, since cards wait for phase 6). Nothing in the API sets `paused` yet; the check reads the column so phase 6 only has to write it.
- **Guest records from the booking page:** a guest booking reuses an existing guest matched by phone or email but never changes that guest's stored name, email or country. `matchGuest` updates the record it finds, which is right for owners and wrong for anyone on the internet typing someone else's number.
- **Imported stays need a unit and a guest:** `bookings.unit_id` and `guest_id` are required. An import that holds every unit records the first active unit by position as its `unit_id`, and each import gets its own guest row ("Airbnb stay"), since it has no phone or email to match on.

## Context

- Tables that already exist:
  - `ical_feeds` (`00005_create_property_setup.sql`): `unit_id`, `status`, `last_synced_at`, `last_error`, `etag`, `consecutive_failures` and `upcoming` are unused until now, and `InsertIcalFeed` never sets `unit_id`.
  - `bookings` (`00006_create_bookings.sql`) has `feed_id`, `external_uid` with `UNIQUE (feed_id, external_uid)`, and `hold_expires_at`.
  - `unit_nights` has `hold_expires_at`, and its check is named `unit_nights_check`: `num_nonnulls(booking_id, block_id) = 1`.
- Ledger queries are in `apps/api/internal/db/queries/calendar.sql`: `InsertUnitNights` takes `booking_id` or `block_id`, `FirstHeldNight` names the clash, and `DeleteExpiredHolds` is run by `store.SweepExpired`.
- `store.holdNights` (`apps/api/internal/store/bookings.go:367`) sorts units, inserts in a savepoint and turns a `unit_nights_pkey` clash into `*domain.ConflictError` with the other stay's ref and guest name.
- `store.CreateBooking` (`bookings.go:67`) needs a `tenant.Tenant` and records `t.UserID` as creator, event actor and activity actor. Guest and import paths have no user.
- `InsertBooking` (`queries/bookings.sql`) takes `request_expires_at`, `payment_due_at`, `promo_id` and `created_by`, but not `feed_id`, `external_uid`, `hold_expires_at` or anything about guest access.
- `server.CreateBooking` (`apps/api/internal/server/bookings.go:136`) is the pattern: normalize and validate `StayInput`, price with `priceStay` (overrides through `store.RateOverrides`), `domain.RefPrefix`, `domain.LedgerUnits`, `idempotent(...)`.
- `StayInput.Validate` (`internal/domain/booking.go`) does not check stay rules (`Rules.MinNights`, `MaxNights`, `SameDayCutoff`, `WindowMonths`, `ClosedArrival`), unit `Sleeps`, rate override `closedToArrival` or promo validity. Owners may break those; guests may not.
- `pricing.Price` (`internal/domain/pricing/pricing.go:94`) applies a promo by code only. `pricing.MinNights` (`:81`) and `pricing.Deposit` (`:183`) exist. Keep `Price` unchanged: `packages/api-spec/fixtures/pricing.json` holds it level with the app.
- `GetPublicProperty` (`internal/server/properties.go:210`) returns slug, name, type, location, currency and base rate from `store.GetPropertyBySlug`, which returns `domain.Property` without the setup. `store.loadSetup` fills a `[]domain.PropertyDetail`.
- `PropertyDetail.Payments.Bank.AccountNumber` is decrypted by the store, so never put it in a public response except a guest's own bank-transfer booking.
- `files.KindSlip` and `files.MaxBytes[KindSlip]` (10 MB) exist in `internal/files/files.go`. `store.AddSlip` (`bookings.go`, end) needs a tenant.
- Activity builders are in `internal/domain/activity.go`. `BookingAdded` with status `requested` already writes "Request from X". `ActivitySync` and `ActivityImport` map to `EventSync` in `EventFor`.
- Periodic jobs live in `sweeps()` in `internal/jobs/jobs.go`, and the notify job's claim pattern is in `store.SendAlerts` (`internal/store/activity.go`): stamp in a short transaction, do network work after it commits.
- Public routes have no `security` in the spec. CORS for public routes is `*` (`internal/server/http.go:50`), though the booking app only calls the API from its server.
- `domain.Property` has no status field; `properties.status` (`live` or `paused`, from `00005`) is only in the database.
- `pricing` imports `domain`, so code in `domain` can't use `pricing.Overrides` or `pricing.MinNights`.
- `store.matchGuest` (`bookings.go:400`) updates the name, email and country of the guest it matches.
- `domain.IcalFeed` already has `Status`, `LastSync`, `Error` and `Upcoming`; only a unit is missing.
- `promos.usage_limit` is null for unlimited (`CHECK (usage_limit > 0)`), and its dates are `starts_on` and `ends_on`.
- `files.Belongs(key, accountID, files.KindSlip)` checks a key is the account's slip.
- The booking app (`apps/booking`, Next 16):
  - `src/lib/api.ts` is `server-only` and has `getPublicProperty`.
  - `src/app/p/[slug]/page.tsx` shows a placeholder cover and `BookingCard`.
  - `src/app/p/[slug]/embed/embed-bridge.tsx` opens the booking page with `checkIn`, `checkOut` and `guests`.
  - `src/components/booking-card.tsx` says availability is not wired.
  - `src/proxy.ts` maps `<slug>.<root>` onto `/p/[slug]`.
- The app's iCal screen is `apps/mobile/src/app/property/[id]/ical.tsx`:
  - it builds the export link as `https://api.staykey.direct/ical/${p.slug}-${p.icalExportToken}.ics`;
  - `refresh()` only refetches the property;
  - feeds map in `apps/mobile/src/data/from-api.ts:84` and save through `toPatch` at `:168`.
- `cmd/seed` adds two feeds with made-up Airbnb and Booking.com links (`apps/api/cmd/seed/main.go:242`). A live importer would fetch and fail on them.
- `TestTenantIsolation` needs every new owner path parameter in `aliceIDs` (`internal/server/isolation_test.go`).

## Constraints

- Work in this order, one commit per step: migration, sqlc queries, domain with tests, OpenAPI, `bun run generate`, store, handlers, jobs, booking app, mobile, docs.
- No em dashes anywhere: code, comments, UI copy, commit messages.
- Never edit `api.gen.go`, `internal/store/queries` or `packages/api-client/src/schema.ts` by hand.
- Add migration `00009` only, with a working `Down`. Never change `00001` to `00008`.
- Every business table carries `account_id`; owner store methods take a `tenant.Tenant`. Guest and import store methods take the account id from the property or feed they act on, never from the request.
- Money is integer minor units with a currency; stay dates are local `YYYY-MM-DD` in the property's time zone.
- Wrap Go errors with `%w`, never discard one with `_`, pass `context.Context` first, keep initialisms in one case (`feedID`).
- The server has no request validator. Check lengths and formats the spec states in handlers or domain code.
- Never log secrets, tokens or personal data: no booking tokens, guest phone numbers or full feed URLs (they carry OTA secrets). Log the feed id.
- Never hold a transaction open across a network call: fetch feeds, upload files and send alerts outside transactions.
- Public responses never include another guest's name or ref, bank details (except the guest's own bank-transfer booking), owner notes, promo lists or feed URLs.
- Every new environment variable goes in `apps/api/.env.example` and in `passThroughEnv` for the `dev` task in `turbo.json`.
- Next 16: read `apps/booking/AGENTS.md` and the guides it points to in `node_modules/next/dist/docs/` before writing server actions, `PageProps` or config.
- Tests check behaviour against real Postgres 17 through `storetest.New(t)`, with fixed clocks and `httptest` servers for feeds.

## Changes

### 1. `apps/api/internal/db/migrations/00009_create_holds_and_sync.sql` (new)
- `ALTER TABLE unit_nights ADD COLUMN hold_id uuid`.
- Drop `unit_nights_check`. Add it back as `CHECK (num_nonnulls(booking_id, block_id, hold_id) = 1)`, plus `CHECK (hold_id IS NULL OR hold_expires_at IS NOT NULL)`.
- Add a partial index on `unit_nights (hold_id) WHERE hold_id IS NOT NULL`.
- `ALTER TABLE bookings ADD COLUMN guest_token_hash bytea, ADD COLUMN payment_method text CHECK (payment_method IN ('bank', 'property'))`.
- `ALTER TABLE ical_feeds ADD COLUMN next_sync_at timestamptz`, with an index on `ical_feeds (next_sync_at) WHERE url <> ''`.
- `Down` reverses each step, deleting hold rows before restoring the old check.

### 2. sqlc queries
- `apps/api/internal/db/queries/calendar.sql`:
  - `InsertUnitNights` gains `sqlc.narg('hold_id')` and `sqlc.narg('hold_expires_at')`.
  - Add `DeleteExpiredHoldsOn`: delete rows on `unit_ids` in `[check_in, check_out)` with `hold_expires_at < now()`.
  - Add `DeleteHold`, filtered by `hold_id` and `account_id`, returning how many rows went.
  - Add `HoldMatches`, which returns the hold's units and nights for checking.
  - Add `HeldNights`: `(unit_id, night)` for a property in `[from, to)`, leaving out holds past `now`.
  - Add `ExportNights`: `(unit_id, night, booking_id, block_id)` held by bookings or blocks, not holds, for a property from a date on.
- `apps/api/internal/db/queries/bookings.sql`:
  - `InsertBooking` gains `feed_id`, `external_uid`, `payment_method` and `guest_token_hash`.
  - Add `GetBookingForGuest`: the booking with its property's slug, name, check-in time and time zone, by id, compared against `guest_token_hash` in Go.
  - Add `ListFeedBookings`: imported stays for a feed with id, `external_uid`, status, dates and whether they hold nights.
  - Add `UsePromo`: `UPDATE promos SET used_count = used_count + 1 WHERE id = @id AND account_id = @account_id AND (usage_limit IS NULL OR used_count < usage_limit) RETURNING id`. No row back means the code ran out between the quote and the booking.
  - Add `InsertGuestOnly` or reuse `FindGuest` plus `InsertGuest`, so guest bookings never run `UpdateGuest`.
- `apps/api/internal/db/queries/setup.sql`:
  - `InsertIcalFeed` and `UpdateIcalFeed` take `unit_id`. A changed link or unit also sets `next_sync_at = NULL` (due now).
  - `ListIcalFeeds` returns `unit_id`.
- `apps/api/internal/db/queries/sync.sql` (new):
  - `ClaimDueFeeds`: `UPDATE ... SET next_sync_at = @lease_until WHERE id IN (SELECT ... WHERE url <> '' AND (next_sync_at IS NULL OR next_sync_at <= @now) ORDER BY next_sync_at NULLS FIRST LIMIT @row_limit FOR UPDATE SKIP LOCKED) RETURNING id, account_id, property_id, unit_id, channel, url, etag, consecutive_failures, last_synced_at`.
  - `FeedSucceeded`: status `ok`, `last_synced_at`, `etag`, failures 0, `last_error` null, `upcoming`, `next_sync_at`.
  - `FeedFailed`: failures + 1; status `error` once failures reach 2; `last_error`, `next_sync_at`; returns the new failure count.
  - `MarkFeedDue`: for the owner's "Sync now", filtered by `account_id` and `property_id`.
- Run `bun run generate`.

### 3. Domain
- `apps/api/internal/domain/pricing/stayrules.go` (new). It lives in `pricing` because it needs `Overrides` and `MinNights`, and `pricing` already imports `domain`. `GuestStay(p domain.PropertyDetail, in domain.StayInput, o Overrides, now time.Time) error` returns a `*domain.ValidationError` for:
  - check-in before today, or a same-day arrival after `SameDayCutoff` (nil means no same-day arrivals), with "today" and the hour taken in the property's `TimeZone`;
  - check-out beyond `WindowMonths`;
  - fewer nights than `MinNights`, or more than `Rules.MaxNights`;
  - arrival on a `ClosedArrival` weekday or an override's `closedToArrival`;
  - more guests (adults plus children) than the unit `Sleeps`. `ExtraGuest` only adds a charge; it isn't extra room.
- `PromoProblem(promo domain.Promo, nights int, checkIn time.Time) error` in the same file: `From` and `To` dates, `Limit` (0 is unlimited in the domain type) against `Used`, and `MinNights`.
- `apps/api/internal/domain/ical.go` (new):
  - `ImportUnits(units []Unit, feedUnitID *uuid.UUID) []uuid.UUID` implements the mapping in Assumptions, through `LedgerUnits`;
  - `ExportRuns` collapses held nights into `[start, end)` runs per stay or block;
  - `NextSync(failures int, now time.Time) time.Time` is the backoff;
  - `SyncInterval = 10 * time.Minute`.
- `apps/api/internal/domain/activity.go`, new builders, with no money in any of them:
  - `DirectBooking(b)`: kind `booking`, "New direct booking from X", subtitle with the dates and how they pay.
  - `SlipUploaded(b)`: kind `slip`, "Bank slip from X", subtitle with the ref.
  - `StayImported(b, channel)`: kind `import`, "Airbnb stay imported", subtitle with the dates.
  - `ImportedStayCancelled(b, channel)`: kind `import`.
  - `FeedConnected(channel, count)`: kind `import`.
  - `ImportClash(b, channel, other)`: kind `sync`, "Airbnb stay overlaps KV-2041".
  - `FeedFailing(channel, propertyName)`: kind `sync`, "Airbnb sync failed".
- `IcalFeed` and `IcalFeedInput` in `apps/api/internal/domain/setup.go` gain `UnitID *uuid.UUID` (`Upcoming` and `Error` exist already). `PropertyPatch.validateFeeds` (`setup.go:798`) checks the unit belongs to the property, against the units in the same patch or the stored ones.
- Tests: `pricing/stayrules_test.go`, `ical_test.go` and new cases in `activity_test.go`, all table-driven.

### 4. `apps/api/internal/ical/` (new package)
- `parse.go`: `Parse(r io.Reader, loc *time.Location) ([]Event, error)`, where `Event{UID, Summary string; Start, End time.Time}` are local dates.
  - Use `github.com/emersion/go-ical` (it has no tagged releases; `go get github.com/emersion/go-ical@latest` gives `v0.0.0-20250609112844-439c63cef608` and pulls in `rrule-go`). Decode with `ical.NewDecoder(r).Decode()`, then read each `Event`'s `DateTimeStart(loc)` and `DateTimeEnd(loc)`.
  - Date-time values convert to the property's time zone; a missing `DTEND` means one night.
  - Skip events with UIDs ending `@staykey.direct`, events with `STATUS:CANCELLED`, events that ended before today, and empty runs.
  - Clip to 24 months ahead.
- `fetch.go`: `Fetcher{Client *http.Client; AllowPrivate bool}` with `Fetch(ctx, url, etag string) (body []byte, newETag string, notModified bool, err error)`:
  - `webcal://` becomes `https://`; only http and https;
  - 20 s timeout, at most 5 MB read;
  - sends `If-None-Match`, with `User-Agent: StayKey/1.0 (+https://staykey.direct)`;
  - the dialer refuses loopback, private, link-local, unspecified and multicast addresses unless `AllowPrivate`, so owner links can't reach internal services. This covers redirects too.
- `export.go`: `Write(w io.Writer, runs []domain.ExportRun, prodName string) error` writes VCALENDAR with `PRODID:-//StayKey//Calendar//EN` and all-day VEVENTs.
- Tests: `parse_test.go` with fixture files in `testdata/` (an Airbnb feed with "Reserved" and "Not available", a Booking.com "CLOSED - Not available", a date-time event, a folded line, an echoed `@staykey.direct` UID), and `fetch_test.go` (304 with ETag, size cap, private address refused when `AllowPrivate` is false).

### 5. Store
- `apps/api/internal/store/bookings.go`:
  - Split `CreateBooking` into an unexported `createBooking(ctx, accountID uuid.UUID, actor *uuid.UUID, in domain.NewBooking, now time.Time) (uuid.UUID, error)`, used by the tenant method, guest bookings and imports.
  - `domain.NewBooking` gains `FeedID`, `ExternalUID`, `PaymentMethod`, `GuestTokenHash`, `PaymentDueAt` and `HoldID *uuid.UUID`.
  - When `HoldID` is set, delete that hold's rows in the same transaction before `holdNights`.
  - `holdNights`:
    - calls `DeleteExpiredHoldsOn` first;
    - takes optional hold arguments for `InsertUnitNights`;
    - when the clash is a hold, returns a `ConflictError` with neither id, so guests see "just taken".
  - `TransitionBooking`: when approving, skip `payment_due_at` for `payment_method = 'property'`.
- `apps/api/internal/store/public.go` (new):
  - `PublicProperty(ctx, slug)`: the full `PropertyDetail` through `loadSetup`, plus `properties.status`, which `domain.Property` doesn't carry.
  - `Availability(ctx, propertyID, accountID, from, to, now)`: held nights per unit.
  - `PublicOverrides(ctx, accountID, propertyID, from, to)`: the same rows as `RateOverrides`, without a tenant.
  - `CreateHold(ctx, accountID, propertyID, units, from, to, expiresAt) (uuid.UUID, error)` with a random v4 `hold_id`, and `ReleaseHold`.
  - `CreateGuestBooking(ctx, accountID, nb, now)`: `createBooking` with a nil actor and guest matching that never updates an existing guest; then `UsePromo` when there is a promo; then `DirectBooking` or `BookingAdded` activity with a nil actor.
  - `GuestBooking(ctx, bookingID, tokenHash)`: constant-time hash compare; `domain.ErrNotFound` on mismatch.
  - `AddGuestSlip(ctx, bookingID, tokenHash, key, amount, now)`: checks the key belongs to the booking's account (`files` key prefix); writes `SlipUploaded` activity.
- `apps/api/internal/store/sync.go` (new):
  - `ClaimDueFeeds(ctx, now, lease, limit)`.
  - `ApplyFeed(ctx, feed, events, now)`, in one transaction per feed:
    - create, re-date, retry holding or cancel imported stays per Assumptions, with the unit and guest rules there;
    - write activity, or the single `FeedConnected` line on a first sync;
    - set `upcoming` and `FeedSucceeded`.
  - `FeedNotModified` and `RecordFeedFailure` (returns failures; writes `FeedFailing` activity when it reaches 3).
  - `MarkFeedDue(ctx, t, feedID)`.
  - `ExportNights(ctx, slug, token, unitID *uuid.UUID, from)`, which compares the token in constant time.
- `apps/api/internal/store/setup.go`: write `unit_id` for feeds; load `unit_id`, `upcoming` and `last_error` into `domain.IcalFeed`.
- Tests:
  - `public_test.go`: a hold blocks a second hold; an expired hold doesn't block a booking; booking with a hold keeps the nights; a used-up promo is refused; a wrong token is not found; a guest booking with an existing guest's phone leaves that guest's name unchanged.
  - `sync_test.go`: new, changed, removed and echoed events; a clash with a direct stay alerts and holds nothing, then holds once that stay is cancelled; an owner-cancelled import stays cancelled; a first sync writes one activity line.
  - `TestOneWinnerOutOfFiftyParallelBookings` must still pass.

### 6. `packages/api-spec/openapi.yaml`
- Extend `PublicProperty`, adding fields only:
  - `description`, `amenities`, `photos` (URLs), `checkIn`, `checkOut`, `houseRules`, `policy`, `depositPercent`;
  - `units` (id, name, sleeps, beds, `linkedUnitIds`), `extras` (id, name, price, per);
  - `rules` (min and max nights, window, closed arrival), `booking` (mode, holdMinutes);
  - `payments` (`bank: boolean`, `atProperty: boolean`), `branding` (color, logo URL), `takingBookings: boolean`.
- Public routes:
  - `GET /v1/public/properties/{slug}/availability?from&to` (at most 13 months) returns `{units: [{unitId, taken: Day[], closedArrival: Day[]}]}`.
  - `POST /v1/public/properties/{slug}/quote` takes `{unitId, checkIn, checkOut, adults, children, extras, promo}` and returns `Quote`, or 400 with a field.
  - `POST /v1/public/properties/{slug}/holds` takes `{unitId, checkIn, checkOut, replaces?}` and returns 201 `{holdId, expiresAt}`, or 409 `dates_taken` with no clash details.
  - `POST /v1/public/properties/{slug}/bookings` needs `Idempotency-Key` and takes `{holdId?, unitId, checkIn, checkOut, adults, children, extras, promo, guest, note, paymentMethod: bank|property, source: page|widget}`. It returns 201 `{booking: GuestBooking, token}`, or 409 `dates_taken` or `not_taking_bookings`.
  - `GET /v1/public/bookings/{bookingId}` with header `X-Booking-Token` returns `GuestBooking`:
    - ref, status, property name and check-in time, unit name, dates, guests, lines, total, deposit, paid, balance;
    - `paymentMethod`, `paymentDueAt`, `requestExpiresAt`;
    - `bank` (bank name, account name, number) only for bank transfer;
    - `slips` (amount, status, uploadedAt).
  - `POST /v1/public/bookings/{bookingId}/uploads` takes `X-Booking-Token` and `{contentType, size}`, and returns `UploadTicket`.
  - `POST /v1/public/bookings/{bookingId}/slips` takes `X-Booking-Token` and `{key, amount}`, and returns `GuestBooking`.
  - `GET /ical/{file}` with optional `?unit=` returns `text/calendar`, or 404.
- Owner route (`bearerAuth` plus `accountHeader`): `POST /v1/ical-feeds/{feedId}/sync` (`PermSettings`) returns `IcalFeed`.
- `IcalFeed` and `IcalFeedInput` gain `unitId` (nullable). `Booking` gains `paymentMethod`.
- Run `bun run generate`.

### 7. Handlers
- `apps/api/internal/server/public.go` (new) holds every public route above.
  - **Reads:** use a `systemTenant(accountID)` (owner role, nil user) only to reuse `RateOverrides`, `GetProperty`-style loaders and `priceStay`, with a comment that it must never reach a write.
  - **Writes:**
    - go through the account-scoped store methods in step 5;
    - map `*domain.ConflictError` to 409 `dates_taken` with the message "Those dates were just taken. Pick other dates." and no clash;
    - wrap the booking create in `idempotent(...)` with the system tenant, so a retry replays the same reply and token.
  - Hash booking tokens with SHA-256; compare in constant time.
- `apps/api/internal/server/ical.go` (new):
  - `GetIcalExport` splits `{file}` at its last `-`, strips `.ics` and checks slug and token;
  - sets `Content-Type: text/calendar; charset=utf-8` and `Cache-Control: private, max-age=300`.
  - `SyncIcalFeed` handles the owner route.
- `apps/api/internal/server/property_map.go`: map `unitId`, `upcoming` and `error` for feeds, and `paymentMethod` on bookings.
- `apps/api/internal/server/server.go`: add the new store methods to `Store`.
- `apps/api/internal/server/isolation_test.go`: give Alice's villa a feed with a `PATCH` of `icalFeeds` in the fixture, and add its id as `feedId` to `aliceIDs`.
- Tests:
  - `public_test.go`: quote rules; hold then book; double hold is 409 without a guest name in the body; wrong token 404; paused property 409; slip upload writes activity.
  - `ical_test.go`: export lists a stay and a block and no guest name; a wrong token 404; `?unit=` for a room includes a whole-house booking.

### 8. Jobs and config
- `apps/api/internal/jobs/jobs.go`: add `ical_import` every minute.
  - Claim up to 20 due feeds with a 5 minute lease.
  - Fetch each outside any transaction; on 304 call `FeedNotModified`.
  - Otherwise parse in the property's time zone and `ApplyFeed`.
  - On error call `RecordFeedFailure` with a message fit for the owner, never the raw URL.
- `apps/api/internal/jobs/sync_test.go` (new) is the gate below the app:
  - an `httptest` feed with a stay;
  - one `ical_import` run;
  - `Availability` shows those nights taken;
  - a hold on them returns a conflict.
- `apps/api/internal/config/config.go`: `ICalAllowPrivate` from `STAYKEY_ICAL_ALLOW_PRIVATE`, default true outside production and refused in production. Pass it to `jobs.Start`. Add it to `.env.example` and `turbo.json`.
- `apps/api/cmd/seed/main.go`: set the sample feeds' `next_sync_at` to 2100-01-01 and their status to `ok`, so the importer never fetches the made-up links. Print the export link for the villa.

### 9. Booking app (`apps/booking`)
- `src/lib/api.ts`: add `getAvailability`, `quoteStay`, `holdStay`, `bookStay`, `getGuestBooking`, `uploadSlip` (presign, PUT the bytes, record the slip), all server-only and mapping API errors to `{field?, message}`.
- `src/app/p/[slug]/actions.ts` (new, `"use server"`): `quote`, `hold` (then `redirect` to the checkout), `book` (then `redirect` to the stay page), `sendSlip`.
- `src/components/availability-calendar.tsx` (new): a two-month grid.
  - Taken nights and closed arrivals are struck through, and past days are off.
  - It selects check-in and check-out and won't select a range crossing a taken night.
  - Keyboard reachable, with `aria-label`s on days. Brand tokens only.
- `src/components/booking-card.tsx`:
  - takes `units`, `availability` and `rules`, with a unit select when there is more than one;
  - shows the calendar and guests;
  - on submit calls the `hold` action, or an `onCheckAvailability` override in the embed;
  - shows the quote total once dates are picked.
- `src/app/p/[slug]/page.tsx`:
  - loads the property and 12 months of availability;
  - shows photos (cover plus grid), description, amenities, rooms with sleeps and beds, check-in and out times, house rules and the policy;
  - when not taking bookings, shows "not taking bookings right now".
- `src/app/p/[slug]/book/page.tsx` (new):
  - reads `hold`, `unit`, `checkIn`, `checkOut`, `guests` and `from`;
  - shows the summary, the hold's countdown and the form.
- `src/app/p/[slug]/book/checkout-form.tsx` (new, client):
  - name, phone, email, country, adults and children, extras, promo (re-quotes), payment method radios from `payments`, note;
  - submits to `book` with a form-instance idempotency key;
  - when the hold has lapsed, says so and offers to pick dates again.
- `src/app/p/[slug]/stay/[bookingId]/page.tsx` (new):
  - reads `t` from `searchParams` and loads the booking;
  - shows status, dates, totals, the deadline, bank details for bank transfer and slips;
  - sets `Referrer-Policy: no-referrer` through `generateMetadata` or a header.
- `src/app/p/[slug]/stay/[bookingId]/slip-form.tsx` (new, client): file plus amount, posts to `sendSlip`.
- `src/app/p/[slug]/embed/page.tsx` and `embed-bridge.tsx`: pass units and availability to the compact card, and add `from=widget` and `unit` to the link it opens.
- `next.config.ts`: add `experimental: { serverActions: { bodySizeLimit: "10mb" } }` (the key `node_modules/next/dist/docs/01-app/03-api-reference/05-config/01-next-config-js/serverActions.md` gives; the default is 1 MB).

### 10. Mobile app
- `src/data/types.ts`: add `unitId?: string` to `IcalFeed` and `paymentMethod?: "bank" | "property"` to `Booking`.
- `src/data/from-api.ts`: map them both ways (`:84`, `:168`, `toBooking`).
- `src/api/properties.ts`: add `useSyncFeed()`. It calls `POST /v1/ical-feeds/{feedId}/sync` and refetches the property after about 5 s.
- `src/app/property/[id]/ical.tsx`:
  - build the export link from `API_URL` (`src/api/client.ts`) rather than the hardcoded `https://api.staykey.direct`, so it works against a local API;
  - a unit picker per feed when the property has more than one unit ("Whole property" first);
  - one export link per unit plus the whole-property link when there is more than one unit;
  - "Sync now" uses `useSyncFeed`.
- `src/app/booking/[id].tsx`: one line for bookings from the page or widget, "Pays by bank transfer" or "Pays at the property".

### 11. Docs
- `docs/ROADMAP.md`: tick the phase 5 items.
- `docs/HANDOVER.md`: commits, API and app maps, the calls above, gotchas, gate status.
- `docs/backend-plan.md`: the status line.
- `README.md`: try the booking page, the export link, and a local feed with `python3 -m http.server` and `STAYKEY_ICAL_ALLOW_PRIVATE=true`.

## Do not

- Take card payments or connect PayHere. Phase 6 does.
- Email or message guests (confirmations, reminders, templates sent automatically). There's no email provider yet. The stay page is the guest's record.
- Add rate limits, captchas or client-IP forwarding from the booking app to the API. Phase 6 does. Note the risk in the handover instead.
- Turn the booking page into "message the owner on WhatsApp" for lapsed subscriptions. Phase 6 does.
- Change `pricing.Price` or `packages/api-spec/fixtures/pricing.json`. Guest rules sit beside pricing, not inside it.
- Set `accounts.first_direct_booking_at`. Phase 6 can backfill it from page and widget bookings.
- Add a channel manager, OTA APIs, or two-way sync beyond iCal.
- Change `packages/widget` (the loader). Everything new lives in the embed page.
- Show holds on the owner's calendar.
- Call the API from the browser, or expose `STAYKEY_API_URL` as a `NEXT_PUBLIC_` variable.

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...` -> passes on Postgres 17.
  - `TestTenantIsolation` lists `POST /v1/ical-feeds/{feedId}/sync`.
  - `TestOneWinnerOutOfFiftyParallelBookings` passes 10 runs in a row.
- `bun run generate && git diff --exit-code` -> no diff.
- `bunx biome check . && bun run typecheck` -> clean.
- `cd apps/mobile && bun test src && bunx expo export --platform ios` -> passes.
- `cd apps/booking && bun run build` -> builds.
- Migration `00009` up, down and up again on a seeded database. After down, `\d unit_nights` shows the old check.
- Gate, local:
  1. Serve `ical/testdata/airbnb.ics` with `python3 -m http.server`.
  2. Point the villa's Airbnb feed at it in the app and tap "Sync now".
  3. Within a minute the feed shows "Synced, 1 upcoming stay", Activity shows "Airbnb calendar connected", and those nights are struck through on `http://kingfisher.localhost:3001`.
  4. Add a second event to the file. Within 10 minutes it shows on the page too.
- Booking page, in a browser against the seeded API:
  - pick free dates, continue (hold), fill the form with bank transfer and book;
  - the stay page shows the ref, bank details and deadline;
  - upload a slip;
  - the owner's app shows the stay as a direct booking and "Bank slip from ..." in Activity;
  - a second browser holding the same dates gets "just taken".
- `curl "localhost:8080/ical/kingfisher-<token>.ics"` -> a VCALENDAR with the seeded stays as "Booked" and no guest names.

## Risks

- **SSRF through owner feed links.** -> The dialer check in `ical/fetch.go` with `AllowPrivate` false in production. Test that a feed pointing at `127.0.0.1` fails in production config.
- **Echo loops between OTAs and StayKey's export.** -> Skip `@staykey.direct` UIDs, export only held nights, and stay quiet on import-versus-import clashes. Watch Activity for repeated "overlaps" lines after connecting two channels.
- **An OTA keeps a removed reservation in its export for a while, or drops past events.** -> Cancel only stays that haven't checked out, and never re-create ones the owner cancelled.
- **Holds as a denial-of-service with no rate limits yet.** -> Holds last `holdMinutes` (default 15), a new hold may replace the client's previous one (`replaces`), and holds never show as bookings. Record the gap for phase 6.
- **Guest tokens in stay page URLs can leak through referrers or shared links.** -> Set `Referrer-Policy: no-referrer`. A token only shows one booking and accepts slips for it.
- **The idempotent reply holds the guest token in `idempotency_keys.response` for 24 hours.** -> Acceptable: a replay needs the same random key and body. Don't log replies.
- **Strangers editing guest records.** -> Guest bookings never call `UpdateGuest`; the `public_test.go` case above guards it.
- **Server actions across several booking app machines.** -> Next encrypts action ids per build; set `NEXT_SERVER_ACTIONS_ENCRYPTION_KEY` the same on every instance when hosting more than one. Note it in the handover's production list.
- **Time zones:** DATE-TIME events from Expedia or Agoda land a day off. -> Parse in the property's `TimeZone`, with a fixture for it.
- **A first sync of a busy Booking.com feed holds hundreds of nights in one transaction.** -> Clip to 24 months, which is fine for a villa; check the transaction time in the test with a 200-event fixture.

## Went-sideways flags

- `TestTenantIsolation` fails on `{feedId}`, or Bob gets 200 syncing Alice's feed.
- A public 409 body contains another guest's name or a `KV-` ref.
- After a sync, the same Airbnb stay appears twice in Bookings, or a new "imported" activity line appears on every sync (the feed id or UID isn't the match key).
- The booking page shows nights free that the owner's Calendar shows taken, especially for a room inside a booked whole house (availability isn't read through `LedgerUnits`).
- The API log shows a feed URL, a booking token or a guest's phone number.
- An owner's existing guest gets a new name after someone books on the page with that guest's phone number.
