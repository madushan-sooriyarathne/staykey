# Build phase 11: error tracking, uptime and logs

Drafted 21 Oct 2026 on `feat/phase-10-marketing` (PR #12). Written for the agent that executes it; the checklist is `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

When something breaks, StayKey's owner hears about it before an owner or a guest does, and can see what happened:

- Unexpected errors in the API, the booking app, the marketing site and the owner app reach Sentry, tagged with their release and environment. They never carry a guest's or owner's personal details.
- Better Stack checks the API, booking pages and marketing site every minute, and hears from the API's jobs. It emails the owner of StayKey when one stops.
- The API's logs are kept for 14 days and can be searched, and every request and error carries a request id that joins the two.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

### Error tracking

- **Sentry, with four projects** (`staykey-api`, `staykey-booking`, `staykey-marketing`, `staykey-app`), starting on the free Developer plan (5,000 errors a month, one user, 30 days of history). Use Sentry's EU data region if the account allows it.
- **Errors only.** No performance tracing, profiling, session replay or Sentry logs to begin with (`tracesSampleRate: 0`), so the free quota goes on errors and no screen recordings exist. They can be turned on later.
- **What counts as an error:**
  - API: a panic, a response that ends as a 500, a job run that returns an error, and an email or card refund that fails for good.
  - Web: an unhandled error in a server component, route handler, server action or the browser.
  - App: a crash, an unhandled promise rejection, a render error, and an API answer of 500 or above.
  - Expected outcomes are never reported: 4xx answers, validation, "dates taken", being offline.
- **No personal data in reports**, on every surface:
  - `sendDefaultPii: false` (`SendDefaultPII: false` in Go);
  - no request bodies, cookies or headers except the user agent;
  - query strings removed from every URL;
  - token path segments replaced: `/join/<token>` becomes `/join/:token` and `/verify-email/<token>` becomes `/verify-email/:token`, and the stay page's `?t=` goes with the query;
  - the user is the person's and account's ids only, never a phone, email or name;
  - a scrubber replaces anything shaped like a phone number, email address, Payments.lk key (`sk_live_`, `sk_test_`), bearer token or NIC in messages, exception values and breadcrumbs, as a last line of defence. Postgres errors can echo the values in a unique key, for example.
- **Releases:**
  - API, booking and marketing take the git commit (`STAYKEY_RELEASE`, set at deploy; `dev` locally);
  - the app takes `direct.staykey.app@<version>+<build>` with a dist, which the Sentry Expo plugin sets;
  - source maps upload during the Next builds and EAS builds when `SENTRY_AUTH_TOKEN` is present, and are skipped quietly when it isn't.
- **Environments:** `development`, `staging` and `production`, from `APP_ENV` and its equivalents. With no DSN set, the SDKs are off, which is the default in development and in CI.
- **Floods are capped.** A periodic job that fails every minute would use the month's quota in days, so each job reports a given error at most once an hour, grouped by job and error.
- **Web events go through the sites' own tunnel** (`/monitoring`), so ad blockers don't hide guests' errors. The marketing proxy's matcher already excludes it; the booking app has no proxy.

### Uptime and alerts

- **Better Stack Uptime**, free plan: 10 monitors and heartbeats, alerts by email (and Slack if wanted), one status page.
  - **Monitors, checked every minute:**
    - the API's `/healthz` (expects 200 with `"database":"up"`);
    - the booking app's new `/healthz`, which also reaches the API within 2 seconds;
    - the marketing site's `/healthz`.
  - **Heartbeats, which the API pings after a successful run:** `jobs` (a new one-minute sweep that proves the scheduler runs), `send_emails`, `card_events`, `ical_import` and `notify`. A heartbeat alerts when it misses 5 minutes, with a 5 minute grace.
- **Alerts go to hello@madushan.dev,** after 2 failed checks in a row, with a recovery email. SMS and phone calls need a paid plan and are an open question.
- **A public status page** at status.staykey.direct, showing the API, booking pages and site, but not the job heartbeats.
- Sentry's own uptime and cron monitors aren't used: only one of each is free.

### Logs

- **The API already logs structured JSON-ready lines with slog** to stdout. On Fly.io they are kept only briefly. The official `fly-log-shipper` app ships them to **Grafana Cloud Logs (Loki)**, free plan: 50 GB a month, 14 days of retention, search with LogQL. The booking and marketing apps' logs go the same way.
- **The API logs in JSON in production** (`slog.NewJSONHandler`) and text in development, so fields can be searched. Nothing logged changes; the rules on keys, tokens, names and addresses stay as they are.
- **Every request gets an id:** `X-Request-Id`, kept from the booking or marketing server when the forward secret matches, otherwise generated. It goes on the request log line, every log line written while handling the request, the response header, and the Sentry event (`request_id` tag). Support can ask an owner for it from an error screen later.

### Testing on purpose

- **API:** `POST /internal/test-error` raises an error and a panic report, and answers 500. It exists only when `STAYKEY_OPS_SECRET` is set, and checks the `X-Ops-Secret` header in constant time; without a match it is a 404. It is outside the OpenAPI contract, like `/healthz`.
- **Booking and marketing:** `/internal/test-error` route handlers with the same secret and header, which throw on the server.
- **App:** tapping the version line in Help seven times sends a test error, and says "Test error sent" with its event id. It works in release builds, so the gate can run on a TestFlight build.

## Context

- **API:**
  - `internal/server/http.go`: `recoverPanics` logs panics; `logRequests` writes one slog line per request; `writeAPIError` turns anything that isn't an `apiError` into a 500 and `ResponseErrorHandlerFunc` logs it.
  - `cmd/api/main.go` builds the logger and shuts down on SIGTERM.
  - `internal/jobs/jobs.go`: the `sweeper` worker runs each periodic job and logs its error; river retries.
  - `GET /healthz` returns `{"status":"ok","database":"up"}`.
- **Web:** Next 16 apps with `src/` folders. The marketing site has `src/proxy.ts` (matcher `/billing`); the booking app has server actions and subdomain routing. Neither has `instrumentation.ts`, `global-error.tsx` or a health route.
- **App:** Expo SDK 57; root `src/app/_layout.tsx` exports `RootLayout`; there is no `metro.config.js` yet. `src/api/errors.ts` has `ApiError` and `OfflineError`. Help shows the version with `Constants.expoConfig?.version`.
- **Privacy:** `apps/marketing/src/app/privacy/page.tsx` lists processors; Sentry, Better Stack and Grafana Labs join that list.
- **Hosting** is still the default, Fly.io in Singapore, and staging doesn't exist yet. The gate runs on the first staging deploy, or locally with real DSNs and monitors pointed at a tunnel.

## Constraints

- Work in this order, one commit per step:
  1. `internal/observe` with tests;
  2. API wiring;
  3. the booking app;
  4. the marketing site;
  5. the app;
  6. log shipping and monitor setup as files and a runbook;
  7. privacy policy;
  8. docs.
- Read `apps/booking/AGENTS.md`, `apps/marketing/AGENTS.md` and `apps/mobile/AGENTS.md` first. Before writing SDK code, read Sentry's current guides:
  - Next.js manual setup for the App Router with Turbopack;
  - Expo with `@sentry/react-native`;
  - `sentry-go` with `net/http`.
  Install with `bun add` in each web app, `bunx expo install @sentry/react-native` in the app, and `go get` in the API.
- Reporting must never break a request: Sentry, heartbeats and log shipping run outside the request path or time out within 2 seconds, and failures are logged once, not retried.
- No em dashes. Never log or report a secret, token, phone, email, name or address.
- New settings go in each app's `.env.example` and turbo's `passThroughEnv` (dev) and `env` for builds where the Next apps read them at build time.

## Steps

### 1. `internal/observe` (API)
- `observe.go`: `Init(Config{DSN, Environment, Release})` sets up `sentry-go` with:
  - `SendDefaultPII: false` and `AttachStacktrace: true`;
  - a `BeforeSend` that drops request data except method, route pattern and user agent, removes query strings, and runs the scrubber.
  `Flush(timeout)` and `Enabled()`. Off without a DSN.
- `scrub.go`: `Scrub(string) string` replaces phone numbers (`+94...` and international forms), emails, `sk_live_`/`sk_test_` keys, bearer and refresh tokens, and NIC numbers with `[removed]`.
- `report.go`:
  - `Report(ctx, err, tags)` adds the request id and the tenant's account and user ids when the context has them.
  - `JobReporter` reports a job's error at most once an hour per job and error fingerprint.
- `heartbeat.go`: `Heartbeats{URLs map[string]string}` with `Ping(ctx, name)`. A GET with a 2 second timeout, at most once a minute per name, failures logged once per hour.
- `requestid.go`: middleware that sets and returns `X-Request-Id`, and `RequestID(ctx)`. It keeps an incoming id only when the forward secret matches, so the booking and marketing servers can pass theirs on, and otherwise makes a new UUIDv7.
- Tests:
  - the scrubber on each shape (a Sri Lankan phone, an international phone, an email, both key prefixes, a bearer token, old and new NICs);
  - query strings and token segments removed from a URL;
  - `JobReporter` reports once in an hour and again after;
  - heartbeats ping once a minute and never block past their timeout (against `httptest`).

### 2. API wiring
- `config.go`: `STAYKEY_SENTRY_DSN`, `STAYKEY_RELEASE` (default `dev`), `STAYKEY_HEARTBEAT_URLS` (`name=url` pairs, comma separated), `STAYKEY_OPS_SECRET` and `STAYKEY_LOG_FORMAT` (`json` in production by default, `text` otherwise).
- `cmd/api/main.go`: build the slog handler by format; `observe.Init`; flush on shutdown after the server and jobs stop.
- `http.go`:
  - request ids go outermost, so the request log line and every handler log line carry `request_id`;
  - `recoverPanics` reports the panic;
  - `ResponseErrorHandlerFunc` reports the error it logs;
  - `/internal/test-error` sits beside `/healthz`.
- `jobs.go`:
  - `sweeper` reports errors through `JobReporter` and pings the job's heartbeat after a successful run;
  - a new `heartbeat` sweep pings `jobs` every minute;
  - final email failures (`SendEmails`) and refused card refunds report a warning with the outbox or refund id.
- Tests:
  - a handler error that becomes a 500 is reported once with the request id;
  - a 409 isn't reported;
  - a panic is reported and answers 500;
  - the test route is 404 without the secret, 404 with a wrong one, and 500 and reported with the right one;
  - a reported event carries no phone, email or token from a request that had them in its query, headers and body (a fake Sentry transport collects events).

### 3. Booking app
- `@sentry/nextjs`:
  - `instrumentation.ts` with `register()` and `onRequestError`;
  - `instrumentation-client.ts`;
  - `sentry.server.config.ts`;
  - `app/global-error.tsx` in the booking page's style;
  - `withSentryConfig` in `next.config.ts` (`tunnelRoute: "/monitoring"`, `silent: !process.env.CI`, the auth token from the environment, no telemetry).
- `src/lib/sentry-scrub.ts`, shared by client and server configs:
  - `beforeSend` and `beforeBreadcrumb` drop query strings, and replace `/stay/<id>` tokens and anything the scrubber matches;
  - user context is never set.
  - Unit tests mirror the Go scrubber's cases.
- `src/app/healthz/route.ts`: 200 `{status:"ok", api:"up"}` when the API's `/healthz` answers within 2 seconds, else 503 `{status:"degraded", api:"down"}`; `dynamic`, never cached.
- `src/app/internal/test-error/route.ts`: throws when `X-Ops-Secret` matches `STAYKEY_OPS_SECRET`, else 404.
- Settings:
  - `NEXT_PUBLIC_SENTRY_DSN`, read on both sides;
  - `SENTRY_ORG`, `SENTRY_PROJECT` and `SENTRY_AUTH_TOKEN`, at build only;
  - `STAYKEY_RELEASE` and `STAYKEY_ENVIRONMENT`, exposed to the client through `next.config.ts` `env`;
  - `STAYKEY_OPS_SECRET`.

### 4. Marketing site
- The same as step 3, with its own project and `src/lib/sentry-scrub.ts` covering `/join/<token>` and `/verify-email/<token>`.
- The proxy's matcher covers only `/billing`, so the `/monitoring` tunnel needs no exclusion.
- `/healthz` answers 200 without calling the API, since the site works when the API is down.
- Keep the Lighthouse scores: the Sentry client loads after hydration, and the home and pricing pages still score 90 or more for performance on mobile.

### 5. App
- `bunx expo install @sentry/react-native`; add the `@sentry/react-native/expo` plugin to `app.json` (organization and project from settings), and `metro.config.js` with `getSentryExpoConfig`.
- `src/lib/sentry.ts`:
  - `Sentry.init` with `EXPO_PUBLIC_SENTRY_DSN` and the environment (`EXPO_PUBLIC_APP_ENV`);
  - `sendDefaultPii: false`, `tracesSampleRate: 0`, no replay integration;
  - `beforeSend` and `beforeBreadcrumb` scrub as on the web, including `staykey://join?token=` and the `?t=` of stay links.
  - `setUser({ id })` with the user id after sign-in, and cleared on sign-out; the account id goes on as a tag.
- `_layout.tsx` exports `Sentry.wrap(RootLayout)`; `src/api/errors.ts` reports `ApiError`s with status 500 or above (never 4xx or offline), with the request id from the response header.
- `help.tsx`: seven taps on the version line send a test error and show its event id.
- Expo Go keeps working, without native crash reports; a development build has them. Note in the handover that EAS builds need `SENTRY_AUTH_TOKEN` as an EAS secret for source maps.

### 6. Log shipping and monitors
- `infra/log-shipper/fly.toml` for `flyio/log-shipper` with the Loki sink settings (`LOKI_URL`, `LOKI_USERNAME`, `LOKI_PASSWORD`, `ORG`, `ACCESS_TOKEN` as Fly secrets; none committed).
- `infra/better-stack.md`: the monitors and heartbeats to create (names, URLs, expected bodies, intervals, grace, alert policy), the status page, and how to put each heartbeat URL into `STAYKEY_HEARTBEAT_URLS`. Better Stack's API or Terraform provider can come later.
- `docs/RUNBOOK.md` (new):
  - what each alert means and its first checks;
  - finding a request by id in Grafana and Sentry;
  - the useful LogQL queries (errors in the last hour, one request id, one account's requests, slow requests);
  - rotating the DSNs and the ops secret.

### 7. Privacy policy
- Add to the processors in `/privacy`: Sentry (error reports, with no personal details), Better Stack (uptime checks) and Grafana Labs (server logs, kept 14 days). Update its date. The policy is still a draft until reviewed.

### 8. Docs
- `docs/ROADMAP.md`: tick the items as they land. The gate stays open until it runs on a deployed environment.
- `docs/HANDOVER.md`:
  - maps for `internal/observe` and the Sentry files in each app;
  - the calls above, gotchas and the gate status;
  - the accounts to create: Sentry, Better Stack and Grafana Cloud.
- `README.md` and the status line in `docs/backend-plan.md`.

## Do not

- Send personal data to Sentry, Better Stack or Grafana: no PII defaults, bodies, cookies, auth headers, query strings or token segments.
- Turn on session replay, tracing or profiling in this phase.
- Report expected outcomes (4xx, validation, conflicts, offline) as errors.
- Let reporting, heartbeats or logging slow or fail a request or a job.
- Commit Sentry auth tokens, the ops secret or Grafana credentials. DSNs are public by design, but they still come from settings.
- Add the test routes to the OpenAPI contract, or let them answer without the secret.

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...`; `bunx biome check . && bun run typecheck`; `bun run generate && git diff --exit-code`; `cd apps/booking && bun test src && bun run build`; `cd apps/marketing && bun test src && bun run build`; `cd apps/mobile && bun test src && bunx expo export --platform ios`.
- Locally with real DSNs for a development environment:
  - each test route and the app's seven taps create one event in the right Sentry project, with the release and environment;
  - the API's event carries the request id, which matches the API log line.
- Send a request with a phone, an email and a `?t=` token in the query and a bearer header to the test route. The Sentry event shows none of them.
- `/healthz` on the booking app answers 503 when the API is stopped, and 200 again when it starts.
- With heartbeat URLs pointed at a local `httptest` stand-in, each heartbeat is pinged after its job runs, at most once a minute.
- Lighthouse on mobile for the marketing home and pricing pages stays at 90 or more for performance.
- **Gate (on staging):** a test error from each surface shows up in Sentry with its release; stopping the API raises Better Stack's alert email within 3 minutes and the recovery email when it starts; the API's logs for the last hour can be searched by request id in Grafana.

## Risks

- **Personal data leaking into reports.** Default PII off, query and token stripping, and the scrubber as a backstop, with tests that send PII on purpose.
- **Quota exhaustion.** Jobs report once an hour per error, expected outcomes aren't reported, tracing is off. If a release still floods, Sentry's spike protection and per-project rate limits cap it.
- **Alert fatigue.** Two failures before an alert, heartbeats with grace, and job errors in Sentry rather than email.
- **Vendor sprawl.** Three free accounts (Sentry, Better Stack, Grafana Cloud). Better Stack's logs could replace Grafana's (one vendor, but 3 days of free retention); see the open questions.
- **Bigger app builds.** The Sentry native SDK adds to the binary and needs a development build for native crashes; Expo Go still runs.

## Went-sideways flags

- A Sentry event with a phone, an email, a token, a guest's name or a request body in it.
- An expected 4xx or "dates taken" in Sentry.
- A request or job that waits on Sentry, a heartbeat or the log shipper.
- The test route answering without the secret, or appearing in the OpenAPI spec.
- The marketing site's performance on mobile dropping below 90.

## Open questions

| Question | Default if unanswered |
| --- | --- |
| Error tracking service | Sentry, free Developer plan, EU region if offered |
| Uptime monitoring service | Better Stack, free plan; alerts by email to hello@madushan.dev |
| SMS or phone call alerts for the API being down | No; email only until Better Stack's paid plan is worth it |
| Log service | Grafana Cloud Logs, free, 14 days; Better Stack logs (3 days free) if one fewer vendor matters more |
| Performance tracing | Off; consider 5% of API requests later |
| A public status page | Yes, status.staykey.direct, without job heartbeats |
| Slack for alerts | Not set up; email only |
