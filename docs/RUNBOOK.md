# StayKey runbook

What to do when something breaks in production. The monitors behind the alerts are listed in `infra/better-stack.md`; the log shipper is `infra/log-shipper/fly.toml`. Fly app names below (`staykey-api`, `staykey-booking`, `staykey-marketing`, `staykey-db`) are the planned ones; change them here if the deploy picks others.

## Where to look

| Tool | What it has | Kept |
| --- | --- | --- |
| Sentry | Unexpected errors from the API, booking pages, marketing site and app, with release, request id and account id | 30 days |
| Better Stack | Uptime of the API, booking pages and site; heartbeats from the API's jobs | Free plan history |
| Grafana Cloud (Loki) | Every log line from the Fly apps | 14 days |
| `fly logs -a <app>` | The last few minutes of an app's output | Minutes |

Sentry only hears from production. Development, staging, CI and tests send nothing, even with a DSN set, so an empty Sentry outside production is expected.

## Alerts and first checks

### API is down

Better Stack's `API` monitor failed twice: `/healthz` didn't answer 200 with `"database":"up"`.

1. `fly status -a staykey-api`: are machines running? If they're crashing, `fly logs -a staykey-api` shows why, usually a missing or wrong secret right after a deploy.
2. If the body said `"database":"down"`, check the Postgres cluster: `fly status -a staykey-db` and its dashboard.
3. A bad deploy: `fly releases -a staykey-api`, then `fly deploy --image <previous image>`.
4. The booking pages monitor fails too while the API is down. That is expected.

### Booking pages are down

`/healthz` on the booking app answered 503 (it couldn't reach the API within 2 seconds) or nothing at all.

- If the API monitor is green, the booking app can't reach it: check `STAYKEY_API_URL` and the private network, and `fly logs -a staykey-booking`.
- If nothing answered, check the booking app's machines and its last deploy, as for the API.

### Marketing site is down

`/healthz` only proves the site's server answers, so this is the site itself: machines, last deploy, DNS.

### A heartbeat stopped

The API pings each heartbeat after its job runs without error. A missed heartbeat means the job is failing or not running.

- `jobs` stopped: the scheduler itself isn't running. Check that the API is up; if it is, look for "River" errors in the logs. Every other heartbeat stops with it.
- One job stopped, `jobs` still green: that job returns errors. Sentry has an issue tagged with the job's name (reported at most once an hour), and River logs each failed run as an error line for the `sweep` kind, with the job's name in its args.
  - `send_emails`: Amazon SES refusing or throttling. Check the SES console and `STAYKEY_SES_*`.
  - `card_events`: Payments.lk unreachable or a key revoked. Owners' card payments aren't being confirmed until it recovers.
  - `ical_import`: one bad feed shouldn't stop the job, so this usually means the database or the network.
  - `notify`: Expo's push service refusing, or `STAYKEY_EXPO_ACCESS_TOKEN` wrong.

### A new issue in Sentry

1. Read the release and the first-seen time: did it start with a deploy?
2. Take the `request_id` tag and find the request's log lines in Grafana (below).
3. The `account_id` tag says whose account it happened in; the `user` is an id only. Look the account up in the database if the owner needs to hear from us.
4. A flood from one job is capped at one event an hour per error, so a high count means many different requests, not one stuck job.

## Finding a request by id

Every API response carries an `X-Request-Id` header. The same id is on the API's request log line, on every other line written while handling it, and on the Sentry event as the `request_id` tag. The API keeps an incoming `X-Request-Id` only from callers that hold the forward secret; the booking and marketing servers don't send one yet, so their calls get a fresh id.

- **From Sentry:** copy `request_id` from the event's tags and search for it in Grafana.
- **From an owner:** ask for the time and what they tapped, then search that minute's errors in Grafana.

## Useful LogQL

Grafana Cloud, Explore, the Loki data source. The shipper labels each stream with the Fly app; check the label browser for the exact label name (`fly_app_name` below).

Errors in the last hour, newest first:

```logql
{fly_app_name="staykey-api"} | json | level="ERROR"
```

Everything about one request:

```logql
{fly_app_name="staykey-api"} |= "01a15eee-e517-7395-87e8-a9b6afcb83fb"
```

Server errors by route, over the selected range:

```logql
sum by (path) (count_over_time({fly_app_name="staykey-api"} | json | msg="request" | status >= 500 [$__range]))
```

Slow requests (over one second; `duration` is in nanoseconds):

```logql
{fly_app_name="staykey-api"} | json | msg="request" | duration > 1000000000
```

The API never logs names, phone numbers, email addresses, tokens or keys, so none of these queries can find one.

## Sending a test error

Each surface has a way to raise a test error on purpose, to check reporting after a deploy. All of them need the ops secret, `STAYKEY_OPS_SECRET`, except the app.

```sh
curl -X POST https://api.staykey.direct/internal/test-error -H "X-Ops-Secret: $STAYKEY_OPS_SECRET"
curl -X POST https://uptime.staykey.direct/internal/test-error -H "X-Ops-Secret: $STAYKEY_OPS_SECRET"
curl -X POST https://staykey.direct/internal/test-error -H "X-Ops-Secret: $STAYKEY_OPS_SECRET"
```

Each answers 500 and creates one Sentry event. Without the secret, or with a wrong one, the routes answer 404. In the app, tap the version line at the bottom of Help seven times; a production build shows the event id.

## Rotating secrets

| Secret | Where it lives | To rotate |
| --- | --- | --- |
| API DSN | `STAYKEY_SENTRY_DSN` on `staykey-api` | Create a new key in the Sentry project, `fly secrets set`, then delete the old key |
| Booking and marketing DSNs | `NEXT_PUBLIC_SENTRY_DSN` at build time | New key in Sentry, update the build setting, redeploy, delete the old key |
| App DSN | `EXPO_PUBLIC_SENTRY_DSN` in the EAS production environment | New key, then a new build or EAS Update. Keep the old key until most owners have updated |
| Sentry auth token | `SENTRY_AUTH_TOKEN` in the web builds and as an EAS secret | Create a new organization token with release and source map scopes, replace it, revoke the old one |
| Ops secret | `STAYKEY_OPS_SECRET` on the API, booking and marketing apps | `openssl rand -base64 32`, then `fly secrets set` on all three |
| Heartbeat URLs | `STAYKEY_HEARTBEAT_URLS` on `staykey-api` | Recreate the heartbeat in Better Stack and set the new URL |
| Grafana token | `LOKI_PASSWORD` on `staykey-log-shipper` | New access policy token with `logs:write`, `fly secrets set`, revoke the old one |

DSNs are public by design (the browser and the app carry them), so rotating one is about stopping abuse of the quota, not about a leak.

## Setting up the accounts

- **Sentry:** one organization, EU data region if offered, and four projects: `staykey-api` (Go), `staykey-booking` and `staykey-marketing` (Next.js), `staykey-app` (React Native). Turn on spike protection, and keep the default "new issue" email alert. Leave tracing, profiling, replay and logs off.
- **Better Stack:** as `infra/better-stack.md`.
- **Grafana Cloud:** a free stack, then an access policy with `logs:write` for the shipper. The Loki URL and user id are on the stack's Loki details page.

## Database role

The API and `cmd/migrate` connect as `staykey`, an ordinary role that owns the database. Superusers and `BYPASSRLS` roles skip row-level security without an error, so the API refuses to start in production on one ("connects as a superuser or BYPASSRLS role"). Create the role once with the provider's admin user, as `scripts/db-app-role.sql` describes, and build `DATABASE_URL` from it. If a deploy fails with that message, `DATABASE_URL` points at the admin user.

## Store review sign-in

App Store and Google Play reviewers can't receive a text, so the API can let one number sign in with a fixed code and no SMS. Set `STAYKEY_REVIEW_PHONE` and `STAYKEY_REVIEW_CODE` together on `staykey-api`: a fictional number such as `+1 201 555 0142` (555 01xx numbers are reserved for fiction, so the number never belongs to a real person) and a 6-digit code that isn't one digit repeated. The code goes through the usual limits (5 codes an hour, 5 tries each), and each request writes "store reviewer sign-in code requested" to the log. Give the number and code to the reviewers in App Store Connect and the Play Console, and unset both once the apps are approved if you prefer.
