# Better Stack uptime

StayKey's uptime checks, job heartbeats and status page run on Better Stack Uptime's free plan. They are set up by hand in the dashboard; this file is the record of what exists, so it can be rebuilt or moved to Terraform later. What to do when one fires is in `docs/RUNBOOK.md`.

## Alert policy

- **Who:** hello@madushan.dev, by email. SMS and phone calls need a paid plan.
- **When:** after 2 failed checks in a row ("Confirmation period" 1 minute), with a recovery email when it passes again.
- **Escalation:** none. One person is on call.

## Monitors

Each is an HTTP monitor checked every minute from at least two regions, with a 10 second request timeout and "Keyword" matching where noted.

| Name | URL | Passes when |
| --- | --- | --- |
| API | `https://api.staykey.direct/healthz` | Status 200 and the body contains `"database":"up"` |
| Booking pages | `https://uptime.staykey.direct/healthz` | Status 200. It answers 503 when it can't reach the API within 2 seconds. |
| Marketing site | `https://staykey.direct/healthz` | Status 200 |

The wildcard `*.staykey.direct` sends any subdomain to the booking app, which answers `/healthz` on every host, so the booking monitor needs no real property.

## Heartbeats

The API pings a heartbeat after a job runs without error, at most once a minute per heartbeat. A heartbeat alerts when it hears nothing for its period plus the grace.

| Name | Job | Period | Grace |
| --- | --- | --- | --- |
| `jobs` | the one-minute `heartbeat` sweep, which proves the scheduler runs | 5 minutes | 5 minutes |
| `send_emails` | sends queued email | 5 minutes | 5 minutes |
| `card_events` | reads card payments and refunds from Payments.lk | 5 minutes | 5 minutes |
| `ical_import` | imports Airbnb and Booking.com calendars | 5 minutes | 5 minutes |
| `notify` | sends push alerts | 5 minutes | 5 minutes |

That is 8 of the free plan's 10 monitors and heartbeats.

Each heartbeat has its own URL, shown when it's created. Give them all to the API in one setting, as `name=url` pairs separated by commas:

```sh
fly secrets set -a staykey-api STAYKEY_HEARTBEAT_URLS="jobs=https://uptime.betterstack.com/api/v1/heartbeat/<id>,send_emails=https://uptime.betterstack.com/api/v1/heartbeat/<id>,card_events=https://uptime.betterstack.com/api/v1/heartbeat/<id>,ical_import=https://uptime.betterstack.com/api/v1/heartbeat/<id>,notify=https://uptime.betterstack.com/api/v1/heartbeat/<id>"
```

A job without a URL in the setting simply isn't pinged. Treat the URLs as secrets: anyone holding one can keep a dead heartbeat green.

## Status page

- **Address:** status.staykey.direct, with a CNAME to the address Better Stack gives.
- **Shows:** API, Booking pages and Marketing site, under the names "API", "Booking pages" and "Website". The heartbeats stay off the page; owners don't need to see background jobs.
- **Branding:** the StayKey mark and obsidian on paper, from `@staykey/tokens`.
