# Build phase 9: guest card payments through Payments.lk

Drafted 18 Oct 2026 on `main` after phase 8 (PR #8). Written for the agent that executes it; the checklist is `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

A guest on the booking page or widget can pay the deposit, the full amount or a later balance by card through a Payments.lk hosted checkout. The money goes straight to the owner's own Payments.lk merchant account. The payment confirms the stay once, and cancelling refunds the card from the owner app.

## Decision: each owner connects their own Payments.lk account

StayKey does not collect guest money and pay it out. Owners connect their own Payments.lk account by pasting an API key; StayKey creates checkouts and refunds with that key and never holds the funds. Owners without a Payments.lk account keep bank transfer and paying at the property, which already work.

Why not collect and pay out:

- **Payments.lk can't split or pay out.** Its API has checkouts, payments, refunds and a read-only list of Payable's settlements into the merchant's own bank. It has no connected accounts, sub-merchants, transfers or splits, so payouts to owners would be manual bank transfers from StayKey's account.
- **It is likely a regulated activity.** Taking money for someone else and paying it on is what the Central Bank of Sri Lanka licenses payment service providers and aggregators for under the Payment and Settlement Systems Act. Payable holds that position; StayKey would need its own licence or a written arrangement with Payable, and legal advice either way. Gateway terms usually forbid processing for third parties without approval.
- **The liability moves to StayKey.** Chargebacks, refunds after a payout, fraud and owners who disappear would all land on StayKey's merchant account. Every booking would also pass through StayKey's books as turnover.

The onboarding friction is real, since Payments.lk wants a registered business or a licensed professional, and many small owners have neither. The way to reduce it is a partner arrangement with Payments.lk (owners onboarded from StayKey with Payable doing the checks, or sub-merchants under StayKey with Payable holding the licence). Ask them before building anything bigger; until then the existing payment methods cover owners who can't connect.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- **One Payments.lk connection per StayKey account**, used by all its properties; each property switches cards on or off. Only owners connect, change or remove it (`PermBilling`).
- **Owners paste a secret key** from their Payments.lk dashboard. It is sealed with `internal/secret` bound to the account id; only its last four characters and mode are ever returned. If Payments.lk offers restricted keys, the app asks for one with only checkout, payment, refund and event access.
- **The key is checked when saved** by reading the account and listing one payment. Production refuses `sk_test_` keys, so a test card can never confirm a real stay; development and staging accept them, and the booking page then says "Test mode, no real charge".
- **Events are polled, not pushed.** Each owner would otherwise have to add a webhook endpoint and paste its secret. A job reads `GET /v1/events` for every live connection each minute (`payment.succeeded`, `checkout.expired`, `refund.succeeded`, `refund.failed`), and the stay page checks its checkout straight away when the guest returns. Each event id is recorded once per connection.
- **Payments.lk charges only in rupees.** For a property priced in another currency, the owner sets a rate in the card settings ("Rs 300.00 per USD"), required before cards can be switched on. The checkout amount is the booking-currency amount times the rate, rounded up to the rupee cent; the payment is recorded in the booking's currency with the rupees charged and the rate beside it. Refunds use the rate of the payment they refund.
- **What a card pays** is a property setting: `deposit` (the default, using the deposit percent; the full amount when that is 0) or `full`. On a confirmed stay with a balance before check-in, the stay page also offers "Pay the balance by card".
- **Booking by card**: the guest picks "Card" at checkout; the stay is created `awaiting_payment` with `payment_method = 'card'` and a 60 minute deadline, then the guest goes straight to the Payments.lk checkout. Paying confirms it; the `cancel_unpaid` job cancels it if the deadline passes. Request mode works as now: on approval the stay page and the booking email offer "Pay by card".
- **Card fees** (2.29% to 2.49%, plus 1% for foreign cards) are the owner's, charged by Payments.lk on their account. No surcharge to guests.
- **Refunds** from the cancel screen with method card call `POST /v1/refunds` against the stay's card payment(s), newest first, with the refund row's id as the idempotency key. The refund is `pending` until its event, and shows as such. A failed refund alerts the owner and is not retried.
- **Outbox for refunds**: the cancel transaction writes the refund row; the provider call happens after commit and again from a job for rows not yet accepted, never inside a transaction.

## Context

- Card status already exists: `payment_settings.cards_status` (`off`, `pending`, `on`) from `00005`; `domain.PaymentSettings.Cards` and its validation (`internal/domain/setup.go:76,566`); the app's payments screen shows "In review" for `pending` (`apps/mobile/src/app/property/[id]/payments.tsx:136`). `pending` meant "applied to PayHere"; it now means "switched on, waiting for a connection or a rate".
- `payments.method` already allows `card` and has `provider_ref` (`00006`). `bookings.payment_method` only allows `bank` and `property` (`00009`). `domain.Booking.Paid` sums payments minus refunds (`internal/domain/booking.go:117`).
- Guest bookings: `CreateGuestBooking` (`internal/server/public.go:200`) validates the method and sets the deadline; `GuestPaymentMethod` is `bank | property` in the spec. The booking app's checkout form is `apps/booking/src/app/p/[slug]/book/checkout-form.tsx`, the stay page `stay/[bookingId]/page.tsx` with `X-Booking-Token`.
- Payments.lk client for StayKey's own billing: `internal/billing/paymentslk.go` (`CreateCheckout`, `VerifyEvent`, LKR cents, `Idempotency-Key`). Its API: `POST /v1/checkouts`, `GET /v1/checkouts/{id}`, `GET /v1/payments/{id}` (with `refundedCents`), `POST /v1/refunds` (`paymentId`, optional `amountCents` and `reason`; partial allowed; 409 `not_refundable`), `GET /v1/events?type&since`, account retrieval.
- Deposits: `pricing.Deposit` (`internal/domain/pricing/pricing.go:183`).
- Unpaid stays: `CancelUnpaidBookings` cancels `awaiting_payment` stays past `payment_due_at` with no payment (`internal/db/queries/jobs.sql`).
- Cancel: `CancelBooking` records a refund row (`internal/store/bookings.go`); the app picks the refund method (`apps/mobile/src/app/booking/cancel.tsx:53`).
- Emails: the `received` email follows the stay's status (`domain.ComposeEmail`); `confirmed` goes on a move to confirmed.

## Constraints

- Work in this order, one commit per step: migration, sqlc queries, domain with tests, OpenAPI, `bun run generate`, the Payments.lk merchant client, store, handlers and jobs, booking app, mobile app, docs.
- Add migration `00014` only, with a working `Down`.
- No em dashes anywhere. Never edit generated files.
- Never hold a transaction open across a Payments.lk call. Never log a secret key, a checkout URL with its token, card details or guest names; log connection, checkout and payment ids.
- Owner store methods take a `tenant.Tenant`; job methods take ids from the rows they act on. New owner routes with path parameters go in `TestTenantIsolation`'s `aliceIDs`.
- Tests stand in for Payments.lk with `httptest` at the HTTP boundary, as `internal/server/billing_test.go` does.
- Next 16 for the booking app, Expo SDK 57 for the app: read `apps/booking/AGENTS.md` and `apps/mobile/AGENTS.md`.

## Changes

### 1. `00014_create_card_payments.sql`
- `card_connections`: `account_id` (PK, FK cascade), `secret_key_sealed`, `key_last4`, `mode` (`test`, `live`), `merchant_name`, `status` (`connected`, `failed`), `checked_at`, `events_since` (cursor), `created_at`, `updated_at`. RLS like 00011.
- `payment_settings`: `card_charge` (`deposit`, `full`, default `deposit`), `card_rate` (LKR cents per one unit of the property's currency, nullable; not needed for LKR).
- `card_checkouts`: `id`, `account_id`, `booking_id` (composite FK), `purpose` (`deposit`, `full`, `balance`), `amount` (booking currency), `currency`, `charged_cents` (LKR), `rate`, `provider_checkout_id`, `provider_payment_id`, `status` (`open`, `paid`, `expired`), `expires_at`, `created_at`, `paid_at`. RLS.
- `card_events`: `account_id`, `event_id`, `type`, `received_at`; unique `(account_id, event_id)`. RLS.
- `payments`: `status` (`pending`, `succeeded`, `failed`, default `succeeded`), `charged_cents`, `charged_currency`, `rate`, `card_checkout_id`; index on `provider_ref`.
- `bookings.payment_method` check gains `card`.
- `Down` reverses each.

### 2. sqlc queries (`internal/db/queries/cards.sql`, plus edits)
- Connections: get, upsert, delete, mark checked, advance `events_since`, list live ones for the job.
- Checkouts: insert, set provider id, get by id and by provider id `FOR UPDATE`, mark paid or expired, open checkout for a booking (reuse within its life).
- Events: insert `ON CONFLICT DO NOTHING` returning whether it was new.
- Refunds: pending card refunds not yet accepted (no `provider_ref`), mark accepted, succeeded or failed.
- `payment_settings` read and write the new columns; booking and payment loaders return the new payment columns.

### 3. Domain
- `internal/domain/cards.go`: `CardCharge`, `CardAmount(property, booking, purpose)` (deposit, full or balance), `ToLKR(amount, rate)` rounding up and `FromLKR` for refunds rounding down, `CardsReady(settings, connection, currency)` (connected, live in production, a rate for non-LKR), `ErrCardsOff`, key format checks (`sk_live_`, `sk_test_`).
- `Payment` gains `Status`, `Charged`, `Rate`; `Paid` counts pending refunds as refunded and leaves out failed ones.
- `GuestPaymentMethod` gains `card`; activity `CardPaid` ("Card payment from Emma Larsen", owners and managers only) and `CardRefundFailed`.
- Tests: amounts per charge mode and purpose, rounding both ways across rates, `CardsReady` for each missing piece, `Paid` with pending and failed refunds.

### 4. `packages/api-spec/openapi.yaml`
- `GET /v1/card-connection` (owners: mode, last four, merchant name, status, checked at; 404 when none), `PUT /v1/card-connection` with `{secretKey}` (checks it; 400 `invalid_key`, 400 `test_key_in_production`), `DELETE /v1/card-connection` (switches every property's cards off).
- `PaymentSettings` gains `cardCharge`, `cardRate`, and `cards` reads `on` only when ready.
- `GuestPaymentMethod` gains `card`; `PublicProperty.payments` gains `card` (`{charge, testMode}` when ready).
- `POST /v1/public/bookings/{bookingId}/card-checkout` (`X-Booking-Token`; body `{purpose}`; 201 `{url, amount, currency, charged, chargedCurrency}`; 409 `bad_status` or `cards_off`) and `POST /v1/public/bookings/{bookingId}/card-checkout/check` (reads the open checkout now; 200 with the booking).
- `Payment` gains `status`, `charged`, `chargedCurrency`.
- Run `bun run generate`.

### 5. `internal/billing/merchant.go` (new)
- `Merchant{SecretKey, BaseURL, Client}` reusing the request plumbing in `paymentslk.go`: `Account`, `CreateCheckout` (reference, LKR cents, description "Kingfisher Villa, KV-2041", success and cancel URLs), `GetCheckout`, `Refund(paymentID, amountCents, reason, idempotencyKey)`, `Events(since, types)`.
- Tests against `httptest`: request fields, idempotency keys, error shapes, event paging.

### 6. Store
- `ConnectCards(t, key, check func)`, `CardConnection(t)`, `DisconnectCards(t)`.
- `StartCardCheckout(bookingID, tokenHash, purpose, now)` returns what to send to Payments.lk (reusing an open checkout), then `SetCheckoutProvider` after the call.
- `ApplyCardEvent(accountID, event, now)`: records the event once; `payment.succeeded` for one of our checkouts adds the payment (`method card`, `provider_ref`, charged rupees and rate), writes `CardPaid`, and moves `awaiting_payment` to confirmed through `moveTo` (so the `confirmed` email goes); a payment whose amount differs from the checkout is refused and logged, like web plans. `checkout.expired` closes the checkout. `refund.succeeded` and `refund.failed` settle the refund row; failure writes `CardRefundFailed`.
- `CancelBooking` with a card refund splits it over the stay's card payments, writes `pending` refund rows; `PendingCardRefunds` and `MarkRefundSent` serve the job.
- Guest booking with `card`: `awaiting_payment`, 60 minute `payment_due_at`.
- Tests: a paid event confirms once (twice delivered, one payment); a wrong amount changes nothing; an expired checkout lets `cancel_unpaid` free the nights; the balance checkout covers only what's left; a refund over two card payments makes two rows; RLS keeps another account's events out.

### 7. Handlers, jobs and config
- Routes above; the stay page check calls `GetCheckout` and then `ApplyCardEvent` with a synthetic event id of the payment, so the event that arrives later is a repeat.
- `card_events` job every minute: for each live connection, read events since its cursor, apply them, advance the cursor; one connection failing doesn't stop the rest; a 401 marks the connection `failed`, switches its properties' cards to `pending` and alerts the owner.
- `send_card_refunds` job every minute, plus one attempt straight after a cancel.
- No new environment variables.
- Tests: connecting a test key in production is refused; a checkout for a stay at another property's token is 404; a card checkout on a property with cards off is 409; the full flow against a fake Payments.lk (book, check, confirmed, cancel, refund pending, refund event, refunded).

### 8. Booking app
- Checkout form: "Card" beside bank transfer and paying at the property when `payments.card` is set, with what will be charged ("Deposit Rs 16,740.00 now, the rest later") and the test-mode note; booking by card goes straight to the Payments.lk URL.
- Stay page: "Pay by card" for an unpaid stay and "Pay the balance by card" for a confirmed one with a balance; on return (`?paid=1`) it calls the check route and shows "Payment received" or "We're waiting for Payments.lk to confirm your payment".

### 9. Mobile app
- More, or the property payments screen: "Card payments" for owners: how to get a Payments.lk account (link to payments.lk), paste the key, see the connected merchant, mode and last four, disconnect.
- Property payments: the cards switch works only when the account is connected; charge deposit or full; the rupee rate for non-LKR properties.
- Booking detail: card payments show "Card", rupees charged, and refund status (pending, refunded, failed).
- Cancel screen: a card refund says it goes back to the guest's card through Payments.lk in a few working days.

### 10. Docs
- `docs/ROADMAP.md`: record the decision and tick phase 9 items; the gate stays open until a sandbox payment and refund go through.
- `docs/HANDOVER.md`: commits, maps, the calls above, gotchas, gate status. `README.md`: trying card payments with a sandbox key.

## Do not

- Collect guest money into StayKey's account or build payouts.
- Ask owners for webhook endpoints or secrets.
- Store card details, saved cards or customer records; use the hosted checkout only.
- Accept test keys in production.
- Add a surcharge, StayKey commission or currency other than rupees at checkout.
- Offer card payments for OTA stays or owner-entered stays (a payment link for those can come later).

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...`, including `TestTenantIsolation`.
- `bun run generate && git diff --exit-code`; `bunx biome check . && bun run typecheck`.
- `cd apps/mobile && bun test src && bunx expo export --platform ios`; `cd apps/booking && bun run build`.
- Migration `00014` up, down and up on a seeded database.
- With a stand-in Payments.lk: connect a key, switch cards on at the villa (USD, rate set), book by card on the page, pay, return: the stay is confirmed with one card payment showing rupees and the rate, Activity says "Card payment", the guest's `confirmed` email is logged. Abandon a second checkout: `cancel_unpaid` frees the nights after an hour. Cancel the first stay with a partial card refund: the refund shows pending, then refunded after the event.
- With a real Payments.lk sandbox key: the same flow with a test card, and the refund visible in the Payments.lk dashboard.

## Risks

- **A held secret key is a target.** Sealed at rest, never returned or logged, restricted keys preferred, and a disconnect that forgets it.
- **A stay confirmed without money.** Payments are matched on our checkout id, amount and mode; test keys are refused in production.
- **Double payments** (two tabs, a retry). One open checkout per stay and purpose is reused; a second payment that arrives anyway is recorded and flagged for a refund rather than lost.
- **Exchange rate drift.** The owner sets and owns the rate; the guest sees the rupee amount before paying.
- **Payments.lk terms** may not allow a platform to hold merchants' keys. Confirm with them before launch.

## Went-sideways flags

- A Payments.lk call inside `InTx`.
- A secret key, checkout URL or guest name in the log.
- One `payment.succeeded` producing two payment rows, or a stay confirmed by a `sk_test_` key in production.
- A refund sent to Payments.lk twice for one refund row.

## Open questions

| Question | Default if unanswered |
| --- | --- |
| Owners' own accounts, or StayKey collects and pays out | Owners' own accounts (above) |
| Exchange rate for non-LKR properties | Owner-set rate per property; an automatic daily rate later |
| What a card pays by default | The deposit |
| For Payments.lk: restricted key scopes, whether villa owners trading in their own name (or with SLTDA registration only) qualify, a partner or referral programme, and whether a platform may hold merchants' keys | Ask before launch; build as above meanwhile |
