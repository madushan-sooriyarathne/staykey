# Build phase 12: end-to-end and load tests

Drafted 21 Oct 2026 on `feat/phase-11-observability` (PR #15). Written for the agent that executes it. The checklist is in `docs/ROADMAP.md` and the state of the code is in `docs/HANDOVER.md`.

## Goal

StayKey's main journeys are tested the way people use them, on every change, and the API is known to hold up under more traffic than launch will bring:

- **Web:** Playwright drives the booking page, checkout, stay page and slip, the widget and web billing against a seeded API, on every pull request.
- **App:** Maestro drives sign-in, onboarding, a new booking, approving a request and recording a payment on Android in CI. The same flows run on iOS.
- **Load:** a load test of booking creation and the public availability route has written targets. It runs on the production machine sizes, and its results are kept in the repo.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

### Web end to end

- **Playwright with `@playwright/test`,** in a new workspace, `tests/web` (`@staykey/e2e-web`). The root `workspaces` gains `tests/*`.
- **Production builds:** the suite tests `next build` output of the booking app and marketing site, as guests get them. It doesn't test `next dev`.
- **Browsers:**
  - the booking page and widget run in Chromium at desktop size and in WebKit with an iPhone profile, since most guests book on phones and Safari has broken hydration before (the `Intl` date gotcha);
  - billing runs in Chromium only.
- **A fresh, seeded stack per run:**
  - Playwright's `webServer` starts the API, the Payments.lk stand-in, the booking app and the marketing site, and waits on each `/healthz`;
  - `globalSetup` migrates, runs `cmd/seed -reset`, and sets up through the API what the seed doesn't: an own Payments.lk connection to the stand-in and cards switched on for Kingfisher Villa.
- **Every guest is a different visitor.** Each test's browser context sends its own `X-Forwarded-For`, and the API and booking app share an e2e `STAYKEY_FORWARD_SECRET`. The holds and bookings rate limits then count per test, as they would per guest, and stay on.
- **Dates come from the API.** A helper picks a free range from the public availability route inside the property's booking window, using a different week for each test. Tests never hard-code a date.
- **No retries.** Retries stay at 0 in CI, per `.claude/rules/testing.md`. A flaky test is fixed or deleted. On failure, Playwright keeps the trace, screenshots and video as a CI artifact.
- **One behaviour per test.** A journey test may check its steps along the way, but it is named for, and ends on, one outcome a guest or owner sees. This is how the "one assertion per test" rule applies to end-to-end tests.

### Payments.lk stand-in

- **`apps/api/cmd/paymentslk-standin`** promotes the stand-in used by hand in phases 6 and 9 into the repo. It refuses to start when `APP_ENV=production`.
- **What it serves:**
  - the merchant API the API calls: checkouts, payments, refunds and events, with keys that start `sk_test_`;
  - a hosted checkout page with "Pay" and "Cancel" buttons, which returns to the checkout's return URL;
  - for plan checkouts, a `payment.succeeded` webhook to `/webhooks/payments-lk`, signed with `STAYKEY_PAYMENTSLK_WEBHOOK_SECRET`.
- **Selected by setting, not code:** the API and booking app find it through `STAYKEY_PAYMENTSLK_API_URL`, which already exists.

### App end to end

- **Maestro,** with flows in `apps/mobile/.maestro/`:
  - `sign-in.yaml`, `onboarding.yaml`, `new-booking.yaml`, `approve-request.yaml` and `record-payment.yaml`;
  - shared steps in `.maestro/subflows/`.
- **A release build against a development API.** The API returns `devCode` outside production and the app already shows it, so flows read the sign-in code from the screen. Android builds with `expo prebuild` and Gradle's release variant, which the Expo template signs with the debug keystore. `EXPO_PUBLIC_API_URL=http://10.0.2.2:8080` reaches the host.
- **Android in GitHub Actions:**
  - on an emulator (API 35, Pixel profile) with `reactivecircus/android-emulator-runner`;
  - on pushes to `main`, nightly, and on demand, but not on every pull request, since the build and emulator take 20 to 30 minutes.
- **iOS** runs the same flows on a simulator in a weekly and on-demand macOS workflow, and locally on a Mac with `expo run:ios --configuration Release`. EAS Workflows can take over once the EAS project exists (see open questions).
- **Fresh data each run:** each flow starts with `launchApp: clearState: true`, and the run starts from `cmd/seed -reset`. Onboarding signs up a new number made from the run id. `approve-request.yaml` approves one of the seed's requested stays. `record-payment.yaml` records a payment on a seeded awaiting-payment stay.
- **Selectors:** text first, as owners read it. Add a `testID` only where text is ambiguous or changes, and add one to the dev code line so its digits can be read.

### Load

- **k6 (v1),** with scripts in TypeScript in `tests/load`. k6 is a standalone binary, like Go and Maestro, not an npm package.
- **Scenarios and targets:**

| Scenario | Shape | Targets |
| --- | --- | --- |
| Availability | `GET /v1/public/properties/{slug}/availability` for a two-month window, spread over the load properties. Ramp to 50 requests a second over 1 minute, then hold for 5 minutes | p95 under 300 ms, p99 under 800 ms, failures under 0.1% |
| Bookings, spread | Guest bookings (bank transfer, no hold) on free dates across the load properties, at 5 a second for 3 minutes | p95 under 1 s, no 5xx, every 201 has its nights in the ledger |
| Bookings, contention | 20 rounds. In each, 50 guests book the same nights at once, on a room and its whole-house unit | Exactly one 201 per round and the rest 409 `dates_taken`; p95 under 2 s; no 5xx and no deadlocks in the Postgres log |
| Smoke | Each scenario with 1 virtual user for 30 seconds | Runs in CI against the local stack, so the scripts can't rot |

- **Guests look like the booking app.** k6 sends `X-StayKey-Forward-Key` and a random `X-StayKey-Guest-IP` per virtual user, so rate limits count guests and not the load generator. The rate limits themselves aren't load tested; Go tests cover them.
- **Load data:** `cmd/seed -load <n>` adds a "Load test" account with n instant-booking properties of four units each, one of them a whole house over two rooms. The default is 50.
- **Where it runs:** a temporary stack (`staykey-load`) on the machine sizes chosen for production, in the production region, with its own Postgres of the production size. It runs in development mode, so the seed can run; nothing on these routes behaves differently in production. k6 runs from a temporary Fly machine in the same region, so the numbers measure StayKey rather than the distance to it. The stack is torn down after the run.
- **Results:** `tests/load/RESULTS.md` keeps each full run with its date, commit, machine sizes, pool size and the numbers against the targets. The gate's run is the first entry.

## Context

- **CI:** `.github/workflows/ci.yml` has one `check` job on Ubuntu, with a Postgres 17 service, Bun, Node 22 and Go. It runs generate, lint, typecheck, test, migrate and build.
- **Seed:** `apps/api/cmd/seed` creates the sample owner (+94770000001), the manager and caretaker, Kingfisher Villa (`kingfisher`, whole place) and Coral Bay House (`coralbay`, rooms with a whole-house unit), 43 stays (including requests and awaiting-payment stays), slips, a block and a pending invite. `-reset` deletes the sample account first. It refuses `APP_ENV=production`.
- **Booking app:**
  - property pages answer on `<slug>.localhost:3001`, which Chromium and WebKit resolve to 127.0.0.1;
  - the checkout, stay page, slip form and card form are server actions against the public API;
  - `src/lib/visitor.ts` reads `X-Forwarded-For`, and `forwardVisitor` passes it to the API with the forward secret;
  - `/healthz` exists since phase 11.
- **Widget:** `packages/widget` builds `dist/widget.js`. `<div data-staykey="kingfisher" data-staykey-origin="...">` mounts the embed in an iFrame that resizes itself. `example.html` shows the use.
- **Marketing billing:** `/billing` signs in by phone, shows the owner's accounts and plans, and starts a Payments.lk checkout. `/billing/done` reads the plan back.
  - Its sign-in form shows the API's `devCode` only when `NODE_ENV` isn't `production`, so a production build never shows it.
  - The API returns `devCode` only outside production (`ExposeDevCodes: !cfg.IsProduction()`).
- **API limits:**
  - holds 10 per 10 minutes per address, guest bookings 5 per 10 minutes, other public routes 120 a minute;
  - a request carrying the forward key is counted by `X-StayKey-Guest-IP`;
  - `holdId` is optional on guest bookings.
- **App:**
  - 46 files already carry `testID`s;
  - the onboarding "You" step shows "Development server: your code is <code>." whenever the API returns one;
  - `bunx expo export` works, and there is no `eas.json` and no EAS project yet.

## Constraints

- Work in this order, one commit per step:
  1. the Payments.lk stand-in with its test;
  2. `cmd/seed -load`;
  3. the small testability changes in the apps;
  4. the Playwright suite;
  5. the CI job for it;
  6. the Maestro flows and the Android build;
  7. their workflows;
  8. the k6 scripts;
  9. the load run and its results;
  10. docs.
- Read `apps/booking/AGENTS.md`, `apps/marketing/AGENTS.md` and `apps/mobile/AGENTS.md` first. Before writing the suites, read the current documentation for each tool:
  - Playwright: `webServer` arrays, projects and devices, `globalSetup`;
  - Maestro: flows, `runScript` with `http`, `evalScript`, running on Android emulators in CI;
  - k6 v1: scenarios, thresholds, TypeScript support.
- Bun is the package manager. Add Playwright with `bun add -d @playwright/test` in `tests/web`, and install its browsers in CI with `bunx playwright install --with-deps chromium webkit`. Locally, if the cached Chromium build doesn't match the Playwright version, point at it with `launchOptions.executablePath`, and never run `playwright install` here.
- Production code changes only where a test can't reach something otherwise, and each one is listed in the handover.
- The suites never call real third parties: no Payments.lk, RevenueCat, SES, oZone, Expo push or Sentry. Development defaults (log senders, no DSN) and the stand-in cover them.
- Tests own their data. Nothing depends on the order tests run in, except inside one file marked `serial`.
- No em dashes. No secrets in the repo. The e2e forward secret, webhook secret and stand-in keys are fixed test values in the Playwright config and CI, named as such, and refused in production by the existing checks (`sk_test_` keys and a missing `STAYKEY_DATA_KEY`).

## Steps

### 1. Payments.lk stand-in

- `apps/api/cmd/paymentslk-standin/main.go`. It listens on `STANDIN_ADDR` (default `:8099`) and keeps its state in memory.
  - **Merchant API:** the routes and shapes `internal/billing/merchant.go` and the plan checkout client use: create and read checkouts, read payments, create refunds, list events with a cursor, and 401 for keys that aren't `sk_test_`.
  - **Hosted checkout:** `GET /pay/{checkoutId}` shows the amount with "Pay" and "Cancel".
    - "Pay" records the payment, adds a `payment.succeeded` event, and for plan checkouts posts the signed webhook to `STANDIN_WEBHOOK_URL`.
    - Either button redirects to the return URL.
  - **Refunds** succeed straight away and add their event.
- `main_test.go` drives the API's real `billing` clients against the stand-in: create, pay, see the event, refund. This keeps the stand-in in step with what the API expects.

### 2. Load data

- `cmd/seed -load <n>` adds the "Load test" account, owned by `+94770009999`. Its properties are `load-001` to `load-<n>`, each with a room pair, a whole-house unit over them and one more room, instant booking with bank transfer, and a 12-month window. `-reset` removes it too.

### 3. Testability changes

- **Marketing:** the billing sign-in form shows `devCode` whenever the API returns one, and drops the `NODE_ENV` check. The API is the gate: it never returns a code in production. Production builds can then run end to end.
- **App:**
  - a `testID="dev-code"` on the dev code line, holding only the digits;
  - `testID`s on the controls the flows can't find by text (the approve button on a request, the payment amount field, the save on new booking) if they're missing.
- Nothing else in the apps changes for testing.

### 4. Playwright suite (`tests/web`)

- `package.json` (`@staykey/e2e-web`):
  - scripts `e2e` (`playwright test`) and `typecheck`;
  - dev dependencies `@playwright/test`, `@staykey/api-client` and `@staykey/config`.
- `playwright.config.ts`:
  - projects `chromium` and `webkit-iphone`;
  - `retries: 0`, `fullyParallel: true`, and 2 workers in CI;
  - `trace`, `screenshot` and `video` kept on failure;
  - `webServer` entries for:
    - the API binary on `:8080`, with `STAYKEY_FORWARD_SECRET`, `STAYKEY_PAYMENTSLK_API_URL` and `STAYKEY_PAYMENTSLK_WEBHOOK_SECRET`;
    - the stand-in on `:8099`;
    - `next start` for booking on `:3001` and marketing on `:3000`;
    - each waits on its `/healthz`, with `reuseExistingServer` locally.
- `global-setup.ts`:
  - runs migrate and `seed -reset`;
  - signs in as the owner through the API with the returned `devCode`;
  - connects an own Payments.lk key (`sk_test_e2e_owner`) to the stand-in, and switches cards on for Kingfisher Villa.
- `fixtures.ts`:
  - a `guest` page whose context sends a unique `X-Forwarded-For`;
  - an `owner` API client;
  - `freeStay(property, testInfo)`, which returns a free check-in and check-out from the availability route, a week per test index.
- **Specs**, each named for its outcome:
  - `booking-page.spec.ts`:
    - a link with dates opens priced, with the nightly total;
    - taken nights can't start or cross a range.
  - `checkout-bank.spec.ts`:
    - a guest holds dates, books with bank transfer, and lands on the stay page with the ref, the bank details and the deadline;
    - the guest uploads a slip and the owner's API shows it on the booking.
  - `checkout-card.spec.ts` (Kingfisher): booking by card goes to the stand-in, "Pay" returns to the stay page, and it says "Confirmed" with "Payment received".
  - `dates-taken.spec.ts` (serial): two guests on the same dates; the second sees "Those dates were just taken".
  - `widget.spec.ts`:
    - serves a host page on `http://owner-site.localhost:4100` with `dist/widget.js` and the embed pointed at `http://kingfisher.localhost:3001`;
    - the iFrame loads and grows to its content;
    - picking dates opens the booking page with those dates and `from=widget`.
  - `billing.spec.ts` (Chromium):
    - the owner signs in on `/billing` with the code on screen, picks the plan that fits and continues to the stand-in;
    - "Pay" leads to `/billing/done` showing "Paid until" a date a month on.
  - `stay-page.spec.ts`: a stay link without its token, or with a wrong one, shows the not-found page and no booking details.
- `tsconfig.json` extends `@staykey/config`. Biome covers the folder.

### 5. CI for the web suite

- A new `e2e-web` job in `ci.yml`, beside `check`, with the same Postgres service and toolchain. It:
  - builds the API binary, the stand-in, the widget, and the booking and marketing apps;
  - installs Playwright's Chromium and WebKit;
  - runs `bun run e2e:web`;
  - uploads `tests/web/test-results` and the HTML report when it fails.
- Root scripts `e2e:web`, `e2e:mobile` and `load:smoke`, and a turbo `e2e` task (`cache: false`) whose `passThroughEnv` lists the test settings.
- Target: under 10 minutes.

### 6. Maestro flows and the Android build

- `apps/mobile/.maestro/`:
  - `config.yaml` (flow order, `appId: direct.staykey.app`);
  - `subflows/sign-in.yaml` (phone, "Send code", read `dev-code`, type it);
  - the five flows:
    - `sign-in.yaml`: the seeded owner lands on Today with the sample stays.
    - `onboarding.yaml`: a new number, through every step with no photos, to "You're live" and Today.
    - `new-booking.yaml`: the plus on Today, dates, a guest, save; the stay opens with its ref.
    - `approve-request.yaml`: a seeded request, Approve; it shows "Awaiting payment".
    - `record-payment.yaml`: an awaiting-payment stay, record the balance; it shows "Confirmed".
- `apps/mobile/scripts/build-e2e-android.sh`: `expo prebuild -p android --clean` and `./gradlew assembleRelease` with `EXPO_PUBLIC_API_URL=http://10.0.2.2:8080`. It prints the APK path.
- `apps/mobile/.maestro/README.md`: running the flows locally on Android and iOS, and on a phone against a LAN API.

### 7. Mobile workflows

- **`.github/workflows/mobile-e2e.yml`** runs on push to `main`, nightly at 02:00 Colombo, and `workflow_dispatch`. Its `android` job:
  - starts Postgres and the API (migrate, `seed -reset`);
  - builds the APK and caches Gradle;
  - runs `maestro test apps/mobile/.maestro` on the emulator;
  - uploads Maestro's screenshots and logs.
- **An `ios` job** on `macos-15` runs weekly and on dispatch only. It builds a Release simulator app with `expo run:ios`, runs the API on the runner, and runs the same flows.

### 8. Load scripts (`tests/load`)

- `lib.ts`: the base URL, the forward key, guest IPs, and free-range picking per property and unit.
- `availability.ts`, `bookings-spread.ts` and `bookings-contention.ts`, with the targets above as k6 thresholds. A missed threshold fails the run.
- `check-ledger.sql` runs after a run and checks two things: no unit night is held twice, and every booking created has its nights.
- `README.md` covers:
  - creating and tearing down the `staykey-load` stack;
  - seeding with `-load 50`;
  - running k6 from a Fly machine in the region, and the smoke runs locally.
- **CI:** a `load-smoke` step in `e2e-web` runs each script with 1 user for 30 seconds against the local API.

### 9. The load run

- Create the load stack on the production sizes, seed it, and run the three scenarios.
- Write `tests/load/RESULTS.md`. If a target is missed, find the cause first: query plans, the pool size, locks. Fix it in its own commit, then run again. Lower a target only with the reason written beside it.
- Tear the stack down.

### 10. Docs

- `docs/ROADMAP.md`: tick the items as they land.
- `docs/HANDOVER.md`:
  - a map of `tests/web`, `.maestro`, `tests/load` and the stand-in;
  - how to run each suite;
  - the calls above, gotchas and the gate status.
- `README.md`: the test commands. Update the status line in `docs/backend-plan.md` and its testing section.

## Do not

- Retry a failing test to make CI green, or add `test.fixme` to hide one without saying so in the handover.
- Turn off rate limits, CSRF checks, token checks or any other rule for the test runs. Vary the visitor instead.
- Call real Payments.lk, RevenueCat, SES, oZone, Expo push or Sentry from a test.
- Load test production with real owners on it, or point the load test at anything but the load stack or a local API.
- Add test-only routes to the API or the OpenAPI contract.
- Commit an APK, a simulator build, Playwright reports or k6 output other than `RESULTS.md`.

## Validate

- `bunx biome check . && bun run typecheck`, and `bun run generate && git diff --exit-code`.
- `cd apps/api && go vet ./... && CI=true go test -race ./...`, including the stand-in's test.
- `bun run e2e:web` passes locally three times in a row from a clean database, in both projects. The CI job passes on the pull request.
- **Break things on purpose, once each:**
  - make the booking page's total wrong, and `booking-page.spec.ts` fails;
  - stop the stand-in returning to the stay page, and `checkout-card.spec.ts` fails;
  - drop the `dates_taken` message, and `dates-taken.spec.ts` fails.

  Revert each.
- Maestro: all five flows pass on a local Android emulator against a local API, and in `mobile-e2e.yml` on dispatch. iOS passes on a Mac or in the macOS job.
- `bun run load:smoke` passes against the local stack, and `check-ledger.sql` returns no rows.
- **Gate:** the web suite passes in CI on the pull request; the Android flows pass in CI on dispatch; the iOS flows pass once on a simulator; and the load run on the production sizes meets its targets, recorded in `tests/load/RESULTS.md`.

## Risks

- **Flaky end-to-end tests:**
  - dates come from the API, every visitor is unique, data is reset per run, and waits are on what's on screen, never on fixed sleeps;
  - jobs that run every minute (cancelling unpaid stays, expiring requests) must not land on a test's stay, so tests use stays that are minutes old.
- **Next overwriting `X-Forwarded-For`.** If `next start` replaces the incoming header with the socket address, every test is one visitor again. Check this first in step 4. The fallback is a booking app setting that trusts a named header only when the forward secret is set outside production, recorded as a call.
- **Slow mobile CI.** Gradle and emulator boot dominate; cache Gradle and the AVD snapshot. If the Android job still passes 40 minutes, run it nightly only and say so.
- **WebKit differences.** Safari quirks found by the WebKit project are real bugs; fix them rather than skipping the test there.
- **Load results that don't transfer.** The load stack uses production sizes but development mode and fresh data. Write down what differs in `RESULTS.md`, and run again once real data exists.

## Went-sideways flags

- A test passes only on a retry, or only when run alone.
- A test or setup step reaches a real third-party service.
- A rule in the API is relaxed so a test can pass.
- The load test's 409s aren't all `dates_taken`, or a unit night is held twice.
- The web job takes more than 10 minutes, or the Android job more than 40.

## Open questions

| Question | Default if unanswered |
| --- | --- |
| iOS flows in CI | A weekly and on-demand macOS job on GitHub Actions; move to EAS Workflows (Maestro on EAS) once the EAS project exists |
| Mobile flows on every pull request | No; on `main`, nightly and on demand, since they take 20 to 30 minutes |
| WebKit for the booking page | Yes, with an iPhone profile, beside desktop Chromium |
| Load targets | Availability p95 under 300 ms at 50 a second; bookings p95 under 1 s at 5 a second; one winner per contended round with p95 under 2 s |
| Where the load test runs | A temporary stack on the production sizes in Singapore, torn down after; never production |
| Production machine sizes | Whatever hosting settles on. The load run records them, and a change of size means a new run |
| Contract tests (responses against `openapi.yaml`) | Not in this phase; the generated types cover the clients |
