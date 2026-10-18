# Build phase 9: guest card payments through Payments.lk

Drafted 18 Oct 2026 on `main` after phase 8 (PR #8). Written for the agent that executes it; the checklist is `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

A guest on the booking page or widget can pay the deposit, the full amount or a later balance by card through a Payments.lk hosted checkout. Each owner chooses where the money goes:

- **Own account (bring your own gateway):** the owner connects their own Payments.lk merchant account; guest money goes straight to them, and StayKey never holds it.
- **StayKey payouts:** for owners without a registered business, StayKey takes the payment on its own Payments.lk account and pays the owner by bank transfer every two weeks, less the card fee.

Either way, the payment confirms the stay once, and cancelling refunds the card from the owner app.

## Before StayKey payouts go live

Collecting guest money for owners and paying it on is likely to need the Central Bank's approval as a payment service provider, or a written arrangement with Payable, which holds that position (Payment and Settlement Systems Act; StayKey is not licensed). Payments.lk's terms may also forbid taking payments for third parties without approval. It also makes StayKey answerable for chargebacks, refunds after a payout and owners who can't be reached.

So the payouts option is built in full but sits behind `STAYKEY_PAYOUTS_ENABLED`, which defaults to off and must stay off in production until there is legal advice and written approval from Payable. The own-account option doesn't depend on it and can ship alone.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

### Shared

- **Card mode is per account:** `off`, `own` or `payouts`. Each property then switches cards on or off. Only owners change the mode, the connection or the payout details (`PermBilling`).
- **Payments.lk charges only in rupees.** For a property priced in another currency, the owner sets a rate ("Rs 300.00 per USD"), required before cards switch on. The checkout amount is the booking-currency amount times the rate, rounded up to the cent. The payment is recorded in the booking's currency with the rupees charged and the rate beside it, and refunds use the rate of the payment they refund.
- **What a card pays** is a property setting: `deposit` (the default, using the deposit percent; the full amount when that is 0) or `full`. On a confirmed stay with a balance before check-in, the stay page also offers "Pay the balance by card".
- **Booking by card:** the guest picks "Card" at checkout. The stay is created `awaiting_payment` with `payment_method = 'card'` and a 60 minute deadline, and the guest goes straight to the Payments.lk checkout. Paying confirms the stay; the `cancel_unpaid` job cancels it if the deadline passes. Request mode works as now: on approval, the stay page and the booking email offer "Pay by card".
- **Events are polled, not pushed.** A job reads `GET /v1/events` each minute for every connection (owner keys and StayKey's own collection key): `payment.succeeded`, `checkout.expired`, `refund.succeeded` and `refund.failed`. The stay page also checks its checkout straight away when the guest returns. Each event id is recorded once per connection. Owners never set up webhooks.
- **Refunds** from the cancel screen with method card call `POST /v1/refunds` against the stay's card payments, newest first, with the refund row's id as the idempotency key. A refund is `pending` until its event. The provider call is made after commit and again by a job for refund rows not yet accepted, never inside a transaction. A failed refund alerts the owner and is not retried.
- **Test keys** are refused in production, so a test card can never confirm a real stay. In development and staging they work, and the booking page says "Test mode, no real charge".

### Own account

- **One connection per account,** used by all its properties. The owner pastes a secret key from their Payments.lk dashboard. It is sealed with `internal/secret`, bound to the account id, and only its last four characters and mode are ever returned. If Payments.lk offers restricted keys, the app asks for one limited to checkouts, payments, refunds and events.
- **The key is checked when saved** by reading the account and listing one payment.
- **Card fees** (2.29% to 2.49%, plus 1% for foreign cards) are charged by Payments.lk on the owner's account.

### StayKey payouts

- **A separate Payments.lk merchant account** for guest payments, not the one for StayKey's plans, so the two never mix: `STAYKEY_COLLECT_PAYMENTSLK_SECRET_KEY`.
- **Payout details:** the owner's legal name, NIC number and a bank account in their own name (bank, branch, account number), plus acceptance of the payout terms (time and version stored). The NIC and account number are sealed. Changing the bank account holds the next payout for 72 hours, and the owner is notified by email and in Activity.
- **A ledger in rupee cents** (`payout_entries`), one row per money movement, each with the date it becomes available:
  - a guest payment, credited in full;
  - the Payments.lk fee, debited (the payment's `feeCents`, read from `GET /v1/payments/{id}`);
  - a StayKey commission, if one is set (`STAYKEY_PAYOUT_COMMISSION_BPS`, default 0);
  - refunds, debited when sent;
  - adjustments (chargebacks, corrections), entered from the payouts command;
  - payouts, debited when paid.
- **Money becomes payable once the stay starts:** a payment's entries become available the day after check-in, and a cancellation before then refunds out of money StayKey still holds. A refund after the payout leaves a negative balance that the next payouts make up. Any cancellation refund that would take the balance below zero beyond what future stays cover is still sent, since the guest's refund comes first.
- **Payouts run every other Monday**, anchored on Monday 2 Nov 2026, covering everything available up to the Sunday before. A job prepares a payout per owner whose available balance is at least Rs 1,000; smaller balances roll over. Payments.lk has no transfer API, so StayKey's team pays the batch by bank transfer and then marks it paid.
- **Payouts are operated from a command, not a web admin:** `go run ./cmd/payouts` with `list`, `export <batch>` (a CSV for the bank's bulk upload: name, bank, branch, account, amount, reference), `paid <batch> --ref <bank ref>`, `failed <payout> --reason` (returns the money to the balance and tells the owner) and `adjust <account> <amount> --reason`. It runs against the production database with the operator's own role.
- **Owners see** their balance, upcoming payout and past payouts with a statement (each stay, fee, refund and adjustment) in the app, and get a `payout_sent` email when a payout is marked paid.

## Context

- Card status already exists: `payment_settings.cards_status` (`off`, `pending`, `on`) from `00005`; `domain.PaymentSettings.Cards` and its validation (`internal/domain/setup.go:76,566`); the app's payments screen shows "In review" for `pending` (`apps/mobile/src/app/property/[id]/payments.tsx:136`). `pending` meant "applied to PayHere"; it now means "switched on, waiting for the card mode, a connection or a rate".
- `payments.method` already allows `card` and has `provider_ref` (`00006`). `bookings.payment_method` only allows `bank` and `property` (`00009`). `domain.Booking.Paid` sums payments minus refunds (`internal/domain/booking.go:117`).
- Guest bookings: `CreateGuestBooking` (`internal/server/public.go:200`) validates the method and sets the deadline; `GuestPaymentMethod` is `bank | property` in the spec. The booking app's checkout form is `apps/booking/src/app/p/[slug]/book/checkout-form.tsx`, the stay page `stay/[bookingId]/page.tsx` with `X-Booking-Token`.
- Payments.lk client for StayKey's plans: `internal/billing/paymentslk.go` (`CreateCheckout`, `VerifyEvent`, LKR cents, `Idempotency-Key`). Its API: `POST /v1/checkouts`, `GET /v1/checkouts/{id}`, `GET /v1/payments/{id}` (with `feeCents`, `netCents`, `refundedCents`), `POST /v1/refunds` (`paymentId`, optional `amountCents` and `reason`; partial refunds allowed; 409 `not_refundable`), `GET /v1/events?type&since`, account retrieval. It has no transfers, splits or connected accounts.
- Deposits: `pricing.Deposit` (`internal/domain/pricing/pricing.go:183`). Unpaid stays: `CancelUnpaidBookings` (`internal/db/queries/jobs.sql`). Cancel: `CancelBooking` records a refund row (`internal/store/bookings.go`); the app picks the refund method (`apps/mobile/src/app/booking/cancel.tsx:53`).
- Email outbox from phase 8: `emails.kind` is checked against a list in `00013`; account emails go through `queueAccountEmail`.

## Constraints

- Work in two parts, each in this order with one commit per step: migration, sqlc queries, domain with tests, OpenAPI, `bun run generate`, Payments.lk client, store, handlers and jobs, booking app, mobile app, docs.
  - **Part A**, shared plus own account (migration `00014`), complete and verified before Part B.
  - **Part B**, StayKey payouts (migration `00015`), plus `cmd/payouts`.
- Each migration has a working `Down`. No em dashes anywhere. Never edit generated files.
- Never hold a transaction open across a Payments.lk call. Never log a secret key, an NIC or account number, a checkout URL or a guest name; log connection, checkout, payment and payout ids.
- Owner store methods take a `tenant.Tenant`; job and command methods take ids from the rows they act on. New owner routes with path parameters go in `TestTenantIsolation`'s `aliceIDs`.
- Money in the payout ledger is integer rupee cents. Balances are always sums of entries, never stored totals.
- Tests stand in for Payments.lk with `httptest` at the HTTP boundary, as `internal/server/billing_test.go` does.
- Next 16 for the booking app, Expo SDK 57 for the app: read `apps/booking/AGENTS.md` and `apps/mobile/AGENTS.md`.

## Part A: card payments, own account

### A1. `00014_create_card_payments.sql`
- `accounts.card_mode` (`off`, `own`, `payouts`; default `off`).
- `card_connections`: `account_id` (PK, FK cascade), `secret_key_sealed`, `key_last4`, `mode` (`test`, `live`), `merchant_name`, `status` (`connected`, `failed`), `checked_at`, `events_since`, `created_at`, `updated_at`. RLS like 00011.
- `payment_settings`: `card_charge` (`deposit`, `full`; default `deposit`) and `card_rate` (rupee cents per unit of the property's currency, nullable; not needed for LKR).
- `card_checkouts`: `id`, `account_id`, `booking_id` (composite FK), `collected_by` (`owner`, `staykey`), `purpose` (`deposit`, `full`, `balance`), `amount` and `currency` (booking), `charged_cents` (LKR), `rate`, `provider_checkout_id`, `provider_payment_id`, `status` (`open`, `paid`, `expired`), `expires_at`, `created_at`, `paid_at`. RLS.
- `card_events`: `source` (`owner:<account>` or `staykey`), `event_id`, `type`, `received_at`; unique `(source, event_id)`.
- `payments`: `status` (`pending`, `succeeded`, `failed`; default `succeeded`), `charged_cents`, `charged_currency`, `rate`, `card_checkout_id`, `collected_by`; index on `provider_ref`.
- `bookings.payment_method` check gains `card`.

### A2. sqlc queries (`internal/db/queries/cards.sql`, plus edits)
- Connections: get, upsert, delete, mark checked or failed, advance `events_since`, list live ones.
- Checkouts: insert, set provider id, get by id and by provider id `FOR UPDATE`, mark paid or expired, the open checkout for a stay and purpose.
- Events: insert `ON CONFLICT DO NOTHING`, reporting whether it was new.
- Refunds: pending card refunds not yet accepted, mark accepted, succeeded or failed.
- `payment_settings` and `accounts.card_mode` read and write; booking and payment loaders return the new payment columns.

### A3. Domain
- `internal/domain/cards.go`: `CardMode`, `CardCharge`, `CardAmount(property, booking, purpose)` (deposit, full or balance), `ToLKR(amount, rate)` rounding up and `FromLKR` rounding down, `CardsReady(mode, settings, connection, currency, production)` saying what is missing, `ErrCardsOff`, and key checks (`sk_live_`, `sk_test_`).
- `Payment` gains `Status`, `Charged`, `Rate`, `CollectedBy`. `Paid` counts pending refunds as refunded and leaves out failed ones.
- `GuestPaymentMethod` gains `card`. New activity lines: `CardPaid` ("Card payment from Emma Larsen", owners and managers only) and `CardRefundFailed`.
- Tests: amounts per charge mode and purpose, rounding both ways across rates, `CardsReady` for each missing piece, `Paid` with pending and failed refunds.

### A4. `packages/api-spec/openapi.yaml`
- `GET /v1/card-payments`: owners only. Returns the mode, the connection (mode, last four, merchant name, status, checked at), whether payouts are available on this server, and the payout profile's status (Part B).
- `PUT /v1/card-payments/mode` with `{mode}`.
- `PUT /v1/card-payments/connection` with `{secretKey}`. It checks the key; errors are 400 `invalid_key` and 400 `test_key_in_production`.
- `DELETE /v1/card-payments/connection`: switches the mode off and every property's cards to `pending`.
- `PaymentSettings` gains `cardCharge` and `cardRate`, and `cards` reads `on` only when ready.
- `GuestPaymentMethod` gains `card`. `PublicProperty.payments` gains `card` (`{charge, testMode}` when ready).
- `POST /v1/public/bookings/{bookingId}/card-checkout` (`X-Booking-Token`; body `{purpose}`): 201 with `{url, amount, currency, charged, chargedCurrency}`, or 409 `bad_status` or `cards_off`.
- `POST /v1/public/bookings/{bookingId}/card-checkout/check`: reads the open checkout now; 200 with the booking.
- `Payment` gains `status`, `charged` and `chargedCurrency`.
- Run `bun run generate`.

### A5. `internal/billing/merchant.go` (new)
- `Merchant{SecretKey, BaseURL, Client}`, sharing the request plumbing with `paymentslk.go`: `Account`, `CreateCheckout` (reference, LKR cents, description "Kingfisher Villa, KV-2041", success and cancel URLs), `GetCheckout`, `GetPayment`, `Refund(paymentID, amountCents, reason, idempotencyKey)` and `Events(since, types)`.
- Tests against `httptest`: request fields, idempotency keys, error shapes, event paging.

### A6. Store
- Connections: `SetCardMode`, `ConnectCards(t, key, check)`, `CardPayments(t)`, `DisconnectCards(t)`.
- `StartCardCheckout(bookingID, tokenHash, purpose, now)`: picks the collector from the account's mode, reuses an open checkout, and returns what to send to Payments.lk. `SetCheckoutProvider` records the provider's id after the call.
- `ApplyCardEvent(source, event, now)` records the event once, then acts on it:
  - `payment.succeeded` for one of our checkouts adds the payment (method card, `provider_ref`, rupees charged, rate, collector) and writes `CardPaid`. It moves an `awaiting_payment` stay to confirmed through `moveTo`, so the `confirmed` email goes. A payment whose amount differs from its checkout is refused and logged.
  - `checkout.expired` closes the checkout.
  - `refund.succeeded` and `refund.failed` settle the refund row; a failure writes `CardRefundFailed`.
  - Part B hooks in here for ledger entries.
- `CancelBooking` with a card refund spreads it over the stay's card payments and writes `pending` refund rows. `PendingCardRefunds` and `MarkRefundSent` serve the refund job.
- A guest booking with `card` is created `awaiting_payment` with a 60 minute `payment_due_at`.
- Tests:
  - an event delivered twice gives one payment and confirms the stay once;
  - a wrong amount changes nothing;
  - an expired checkout lets `cancel_unpaid` free the nights;
  - a balance checkout covers only what is left;
  - a refund over two card payments makes two rows;
  - RLS keeps another account's checkouts out.

### A7. Handlers, jobs and config
- The routes above. The stay page's check calls `GetCheckout`, then `ApplyCardEvent` keyed on the payment id, so the polled event that arrives later is a repeat.
- `card_events` job every minute: for StayKey's collection key, then for each live owner connection, it reads events since the cursor, applies them and advances the cursor. One connection failing doesn't stop the rest. A 401 marks the connection `failed`, switches its properties' cards to `pending` and alerts the owner.
- `send_card_refunds` job every minute, plus one attempt straight after a cancel.
- Tests:
  - a test key in production is refused;
  - a checkout with another stay's token is 404;
  - a card checkout on a property with cards off is 409;
  - the full flow against a fake Payments.lk: book, check, confirmed, cancel, refund pending, refund event, refunded.

### A8. Booking app
- Checkout form: "Card" beside bank transfer and paying at the property when `payments.card` is set. It shows what will be charged ("Deposit Rs 16,740.00 now, the rest later") and the test-mode note. Booking by card goes straight to the Payments.lk URL.
- Stay page: "Pay by card" for an unpaid stay, and "Pay the balance by card" for a confirmed one with a balance. On return (`?paid=1`) it calls the check route and shows "Payment received" or "We're waiting for Payments.lk to confirm your payment".

### A9. Mobile app
- "Card payments" (More, and linked from the property payments screen), owners only: choose own account or StayKey payouts (the second hidden when the server says payouts aren't available). For own account: how to get a Payments.lk account, paste the key, then see the merchant name, mode and last four, with "Disconnect".
- Property payments: the cards switch works only when ready, and says what is missing. It also sets deposit or full, and the rupee rate for non-LKR properties.
- Booking detail: card payments show "Card", the rupees charged and the refund status (pending, refunded, failed). The cancel screen says a card refund goes back to the guest's card through Payments.lk in a few working days.

## Part B: StayKey payouts

### B1. `00015_create_payouts.sql`
- `payout_profiles`: `account_id` (PK, FK cascade), `legal_name`, `nic_sealed`, `bank_name`, `branch`, `account_name`, `account_number_sealed`, `account_last4`, `terms_version`, `accepted_at`, `bank_changed_at`, `created_at`, `updated_at`. RLS.
- `payouts`: `id`, `account_id`, `batch_id`, `period_end`, `amount` (rupee cents), `status` (`prepared`, `paid`, `failed`), `bank_ref`, `reason`, `prepared_at`, `paid_at`. RLS.
- `payout_entries`: `id`, `account_id`, `booking_id` (nullable), `payment_id` (nullable), `kind` (`payment`, `fee`, `commission`, `refund`, `adjustment`, `payout`, `payout_returned`), `amount` (signed rupee cents), `available_on` (date), `payout_id` (nullable), `note`, `created_at`. RLS. Unique on (`payment_id`, `kind`) where set, so one payment is entered once.
- `emails.kind` check gains `payout_sent` and `payout_bank_changed`.

### B2. Queries, domain and spec
- Queries: profile get and upsert; entries insert; available balance up to a date; entries for a payout; prepare a batch (`FOR UPDATE` on unpaid entries, `payout_id` set in one statement per account); mark paid, failed or returned; payouts and statements for an account.
- `internal/domain/payouts.go`:
  - `PayoutCycle(anchor, now)` gives the next run date and the period end;
  - `AvailableOn(checkIn)` is the day after check-in;
  - `Commission(amount, bps)` rounds down;
  - `PayoutMinimum` is Rs 1,000;
  - profile validation: an NIC in the old or new format, account numbers, names.
- Tests: cycles across months and years, availability, commission rounding, NIC formats.
- Spec:
  - `GET` and `PUT /v1/card-payments/payout-profile` (sealed fields come back as last four only);
  - `GET /v1/payouts` (balance, available, pending, next payout date, past payouts);
  - `GET /v1/payouts/{payoutId}` (the statement).
  - The `payouts` mode is refused with 409 `payouts_unavailable` unless `STAYKEY_PAYOUTS_ENABLED` is true and the profile is complete. Add `{payoutId}` to `aliceIDs`.

### B3. Store, jobs, command and config
- `ApplyCardEvent` on a StayKey-collected payment reads `GET /v1/payments/{id}` (after commit, through the event job) and writes the payment, fee and commission entries, available the day after check-in. A sent refund on a StayKey-collected payment writes a refund entry available at once.
- `prepare_payouts` job daily: on a payout Monday, it prepares one batch with a payout per account whose available balance is at least the minimum. Accounts whose bank changed in the last 72 hours, or whose account is closed, are skipped.
- `cmd/payouts`: `list`, `export <batch>` (CSV for the bank's bulk upload), `paid <batch> --ref`, `failed <payout> --reason` and `adjust <account> <amount> --reason`. `paid` queues `payout_sent` emails; `failed` writes `payout_returned` and alerts the owner.
- New settings, each in `.env.example` and turbo `passThroughEnv`:
  - `STAYKEY_PAYOUTS_ENABLED` (default false);
  - `STAYKEY_COLLECT_PAYMENTSLK_SECRET_KEY`, required when payouts are enabled; production refuses a test key;
  - `STAYKEY_PAYOUT_COMMISSION_BPS` (default 0).
- Tests:
  - a StayKey-collected payment writes payment and fee entries once;
  - a cancellation before check-in nets to zero;
  - a refund after a payout leaves a negative balance that the next payout absorbs;
  - a batch is prepared once per Monday and never includes an entry twice;
  - a bank change holds the next payout;
  - `failed` returns the money;
  - the mode can't be chosen with payouts disabled.

### B4. Apps
- App: the payout profile form in Card payments (legal name, NIC, bank, branch, account, terms). Also a Payouts screen under More: available and pending balances, next payout date, past payouts, and each payout's statement.
- Booking app: nothing new; a StayKey-collected checkout looks the same to the guest, with the property's name on it.

## Docs (after each part)
- `docs/ROADMAP.md`: tick items as they land; the gate stays open until a sandbox payment and refund go through, and the payouts item until legal sign-off.
- `docs/HANDOVER.md`: commits, maps, the calls above, gotchas, gate status, and the operator's payout routine. `README.md`: trying card payments with a sandbox key, and running a payout locally.

## Do not

- Turn on `STAYKEY_PAYOUTS_ENABLED` in production, or describe payouts to owners as available, before legal advice and Payable's approval.
- Ask owners for webhook endpoints or secrets.
- Store card details, saved cards or customer records; use the hosted checkout only.
- Accept test keys in production.
- Build a web admin; payouts run from `cmd/payouts`.
- Pay out before check-in, or pay out a balance below the minimum.
- Offer card payments for OTA stays or owner-entered stays (a payment link for those can come later).

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...`, including `TestTenantIsolation`.
- `bun run generate && git diff --exit-code`; `bunx biome check . && bun run typecheck`.
- `cd apps/mobile && bun test src && bunx expo export --platform ios`; `cd apps/booking && bun run build`.
- Migrations `00014` and `00015` up, down and up on a seeded database.
- Part A, with a stand-in Payments.lk:
  - connect a key, switch cards on at the villa (USD, rate set), book by card on the page, pay and return;
  - the stay is confirmed with one card payment showing rupees and the rate, Activity says "Card payment", and the guest's `confirmed` email is logged;
  - abandon a second checkout: `cancel_unpaid` frees the nights after an hour;
  - cancel the first stay with a partial card refund: the refund shows pending, then refunded after the event.
- Part B, with payouts enabled in development:
  - fill the payout profile, switch to payouts, and book and pay two stays by card;
  - move one check-in into the past and run the job on a payout Monday: one payout covers that stay less its fee;
  - `cmd/payouts export` writes the CSV, `paid` marks it and logs `payout_sent`;
  - cancel the other stay before check-in: its refund nets its entries to zero.
- With real Payments.lk sandbox keys: the Part A flow with a test card, and the refund visible in the dashboard.

## Risks

- **Regulatory exposure of payouts.** Behind a switch that stays off until approved; see the section above.
- **Owner keys are a target.** They are sealed at rest, never returned or logged, restricted keys are preferred, and disconnecting forgets the key.
- **A stay confirmed without money.** Payments are matched on our checkout id, amount and mode, and production refuses test keys.
- **Double payments** (two tabs, a retry). One open checkout per stay and purpose is reused. A second payment that arrives anyway is recorded and flagged for a refund, not lost.
- **Negative balances** after refunds or chargebacks. They roll into later payouts; an owner who stops taking bookings leaves StayKey out of pocket, so payouts start only after check-in.
- **Paying the wrong bank account.** Changes are held for 72 hours with an email to the owner, and the CSV is checked by a person before upload.
- **Exchange rate drift.** The owner sets and owns the rate, and the guest sees the rupee amount before paying.

## Went-sideways flags

- A Payments.lk call inside `InTx`.
- A secret key, NIC, account number, checkout URL or guest name in the log.
- One `payment.succeeded` producing two payment rows or two ledger credits.
- A payout entry in two payouts, or a payout prepared before the stay's check-in.
- A refund sent to Payments.lk twice for one refund row.
- The payouts mode selectable while `STAYKEY_PAYOUTS_ENABLED` is false.

## Open questions

| Question | Default if unanswered |
| --- | --- |
| Commission on StayKey payouts | None; the owner bears only the Payments.lk fee (`STAYKEY_PAYOUT_COMMISSION_BPS=0`) |
| When collected money becomes payable | The day after check-in |
| Payout day and minimum | Every other Monday from 2 Nov 2026, Rs 1,000 minimum |
| Exchange rate for non-LKR properties | Owner-set rate per property; an automatic daily rate later |
| What a card pays by default | The deposit |
| For Payments.lk and Payable: approval for collecting on behalf of owners, restricted key scopes, whether villa owners trading in their own name qualify for their own account, a partner programme, and whether a platform may hold merchants' keys | Ask before launch; build as above meanwhile |
