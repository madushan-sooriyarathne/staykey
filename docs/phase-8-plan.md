# Build phase 8: email through AWS SES

Drafted 18 Oct 2026 on top of the phase 7 branch (`feat/phase-7-account-controls`, PR #5). Merge PR #5 before starting. Written for the agent that executes it; the checklist is `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

Guests get their booking emails (received, confirmed, payment reminder, pre-arrival, thank you, declined) from the account's message templates through AWS SES, team members can take alerts and the morning summary by email, and an address that bounces or complains is never emailed again.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- **SES v2 through the AWS SDK** already in the module (`aws-sdk-go-v2`), with explicit keys like R2: `STAYKEY_EMAIL_PROVIDER` is `log` (default, also in production until SES is set up) or `ses`, plus `STAYKEY_SES_REGION`, `STAYKEY_SES_ACCESS_KEY_ID`, `STAYKEY_SES_SECRET_ACCESS_KEY`, `STAYKEY_SES_CONFIGURATION_SET`, and `STAYKEY_EMAIL_FROM` (an address on the verified domain, for example `stays@mail.staykey.direct`).
- **From and Reply-To:** the from name is the property's name ("Kingfisher Villa via StayKey"); Reply-To is the account owner's email when they have one, so guests' replies reach the owner, never StayKey.
- **Guest emails go through an outbox,** `guest_emails`, one row per booking and kind (unique), sent by a job outside any transaction, like the alert outbox. Kinds:
  - `received`: system email, when a guest books on the page or widget (stay page link, how to pay). Not a template.
  - `confirmed`: template `confirmed`, when a stay moves to confirmed or an owner creates it confirmed.
  - `declined`: system email, when a request is declined or expires.
  - `reminder`: template `reminder`, 24 hours before `payment_due_at` for unpaid bank-transfer stays.
  - `prearrival`: template `prearrival`, at 09:00 property time two days before check-in, for confirmed and awaiting-payment stays.
  - `thanks`: template `thanks`, at 10:00 property time the day after check-out, for stays that weren't cancelled.
- **Event emails** (`received`, `confirmed`, `declined`) are queued inside the booking's transaction. **Timed emails** (`reminder`, `prearrival`, `thanks`) are queued by a scheduling job that finds due stays every 15 minutes; the unique key stops repeats.
- **The send job checks again before sending:** a stay that was cancelled, moved or paid since queuing skips the email (`skipped`), so editing a stay never sends a stale one.
- **Only when the guest has an email, and only when the template's channels include `email`.** System emails (`received`, `declined`) always go when there's an address. Imported OTA stays never get emails.
- **Template channels become editable** (`PATCH /v1/templates/{id}` takes `channels`), so an owner can switch off, say, the thank-you email. WhatsApp on templates stays a manual share from the contact sheet.
- **Links in guest emails** go to the guest's stay page (`<booking page>/stay/<id>?t=<token>`). The token is stored sealed (`internal/secret`, bound to the booking id) beside its hash, so the server can put it in emails; owner-created stays get a token the first time an email needs one. Stays booked before this phase have no sealed token and get a fresh one, which replaces the old link (there are no real guests yet).
- **Plain text plus a simple HTML part:** the template body with line breaks and links, the property's name and booking page in the footer. No images, no tracking pixels, no click tracking.
- **Team email alerts:** `notification_prefs.email` (default off), sent to `users.email` when set, through the same sender as the outbox but straight from the alert job, like WhatsApp. The morning summary can go by email too.
- **Bounces and complaints** come from SES through an SNS topic subscribed to `POST /webhooks/ses`. The handler verifies the SNS signature, confirms the subscription once, and records permanent bounces and complaints in `email_suppressions`. Every send checks that table first and marks the row `suppressed`. SES's own account-level suppression list stays on as a backstop.
- **Development** uses the log sender, which records that an email was skipped with the address masked, never the body.

## Context

- Templates: `domain.Template` and `DefaultTemplates` (`apps/api/internal/domain/messages.go`) with keys `confirmed`, `reminder`, `prearrival`, `thanks`, placeholders `{guest_name}`, `{property_name}`, `{check_in}`, `{check_out}`, `{balance}`, `{booking_link}`, `{map_link}`, `{check_in_time}`. `message_templates.channels` is `text[]` checked against `email`, `whatsapp` (`00008_create_activity_and_messages.sql:40`). `UpdateTemplateBody` only changes the body (`internal/db/queries/messages.sql:12`).
- Alerts: the activity outbox claims rows and sends after commit (`store.SendAlerts`, `internal/store/activity.go:158`; `jobs.SendAlerts`, `internal/jobs/jobs.go`). `Recipient` has `Phone`, `PushTokens`, `WhatsApp` (`activity.go:146`). `jobs.Alerts{Push, WhatsApp, Logger}`. `notification_prefs` has `push` and `whatsapp`; `ActivityRecipients` and `DueSummaries` select them.
- WhatsApp senders show the pattern for a provider interface plus a log sender that masks the address (`internal/notify/whatsapp.go`).
- Status changes go through `store.moveTo` (`internal/store/bookings.go:354`), used by owner transitions, cancellations and the jobs' `closeBySystem`. Owner stays are created in `CreateBooking`; page stays in `CreateGuestBooking` (`internal/store/public.go`), which already writes the `DirectBooking` activity.
- Guest tokens: 24 random bytes from `newBookingToken` (`internal/server/public.go:475`), only the SHA-256 is stored (`bookings.guest_token_hash`, migration 00009). `secret.Box` has `Seal(plaintext, context)` and `Open` (`internal/secret/secret.go`); the store holds one (`store.WithSecrets`).
- Booking page URLs: `Server.bookingPageURL(slug)` (`internal/server/properties.go:209`). The booking app's proxy maps `<slug>.<domain>/<path>` onto `/p/<slug>/<path>` (`apps/booking/src/proxy.ts`), so the stay page is `<booking page>/stay/<id>?t=<token>`.
- R2 shows how the API builds an AWS client with explicit keys (`internal/files/r2.go`, `config.R2`).
- Webhooks are spec routes with no security, body as `application/octet-stream` (`/webhooks/revenuecat`, `/webhooks/payments-lk` in `packages/api-spec/openapi.yaml`); rate limits skip `/webhooks/` (`internal/server/limits.go`).
- The owner app: `src/app/template/[id].tsx` edits the body only; `src/app/notifications.tsx` has push and WhatsApp columns.
- Closed accounts (phase 7) must send nothing: `ActivityRecipients` and `DueSummaries` already skip them.

## Constraints

- Work in this order, one commit per step: migration, sqlc queries, domain with tests, OpenAPI, `bun run generate`, `internal/email`, store, handlers and jobs, config and wiring, mobile, docs.
- Add migration `00013` only, with a working `Down`. Never change `00001` to `00012`.
- No em dashes anywhere: code, comments, UI copy, email text, commit messages.
- Never edit `api.gen.go`, `internal/store/queries` or `packages/api-client/src/schema.ts` by hand.
- Never hold a transaction open across an SES or SNS call.
- Never log email bodies, subjects with guest names, or full addresses; log the outbox id, booking id and a masked address.
- Every new environment variable goes in `apps/api/.env.example` and `passThroughEnv` for `dev` in `turbo.json`; production refuses `STAYKEY_EMAIL_PROVIDER=ses` without its settings.
- Owner store methods take a `tenant.Tenant`; job and webhook methods take ids from the rows they act on.
- Tests run against Postgres 17 through `storetest.New(t)`, with a fake sender and fixed clocks; SNS signature tests use a generated key and certificate served by `httptest`.
- In `apps/mobile`, follow `apps/mobile/AGENTS.md`.

## Changes

### 1. `apps/api/internal/db/migrations/00013_create_email.sql` (new)
- `guest_emails`: `id`, `account_id`, `booking_id` (composite FK to bookings, cascade), `kind` (`received`, `confirmed`, `declined`, `reminder`, `prearrival`, `thanks`), `to_address`, `status` (`pending`, `sent`, `skipped`, `suppressed`, `failed`), `scheduled_for`, `claimed_at`, `sent_at`, `provider_message_id`, `error`, `created_at`; unique `(booking_id, kind)`; index on `scheduled_for` where `status = 'pending'`.
- `email_suppressions`: `address` (lower-cased, PK), `reason` (`bounce`, `complaint`), `created_at`. Not tenant data.
- `notification_prefs.email boolean NOT NULL DEFAULT false`.
- `bookings.guest_token_sealed bytea`.
- RLS on `guest_emails` like 00011 (policy `tenant_wall`).
- `Down` reverses each.

### 2. sqlc queries
- `internal/db/queries/email.sql` (new): `QueueGuestEmail` (`ON CONFLICT (booking_id, kind) DO NOTHING`), `ClaimDueGuestEmails` (`FOR UPDATE SKIP LOCKED`, sets `claimed_at`, joined to the booking, guest, property and account owner's email; skips closed accounts), `MarkGuestEmail` (status, `sent_at`, `provider_message_id`, `error`), `ReleaseGuestEmails`, `IsSuppressed`, `Suppress` (`ON CONFLICT DO NOTHING`).
- Timed emails: `DueReminders`, `DuePrearrivals`, `DueThanks`, each returning booking ids and account ids that have a guest email and no row of that kind yet, computed in the property's time zone as in `DueSummaries`.
- `bookings.sql`: `SetGuestToken` (hash and sealed token).
- `messages.sql`: `UpdateTemplate` sets `body` and `channels`; `ActivityRecipients` and `DueSummaries` also return `u.email` and `p.email`; `SetNotificationPref` takes `email`.
- Run `bun run generate`.

### 3. Domain
- `internal/domain/email.go` (new): `GuestEmailKind` and its constants; `RenderTemplate(body string, v TemplateValues) string` replacing the placeholders (unknown ones left as they are); `SystemEmail(kind, v)` for `received` and `declined` (subject and body); `EmailSubject(kind, propertyName)`; when each timed kind is due (`ReminderAt`, `PrearrivalAt`, `ThanksAt`) and `StillWanted(kind, booking, now)` for the send-time check.
- `NotificationPref` gains `Email`; `DefaultNotificationPrefs` leaves it off.
- Tests: rendering every placeholder, a missing email, each due time across a time zone, and `StillWanted` for a cancelled, moved and paid stay.

### 4. `packages/api-spec/openapi.yaml`
- `TemplatePatch` gains `channels` (array of `email`, `whatsapp`).
- `NotificationPref` gains `email`.
- `POST /webhooks/ses` (no security, `application/octet-stream`, 200; 400 when the signature fails).
- Run `bun run generate`.

### 5. `apps/api/internal/email` (new package)
- `email.go`: `Message{To, FromName, ReplyTo, Subject, Text, HTML}`; `Sender` interface (`Send(ctx, Message) (messageID string, err error)`); `LogSender` (masks the address, logs nothing else).
- `ses.go`: `SES{Region, AccessKeyID, SecretAccessKey, From, ConfigurationSet}` using `github.com/aws/aws-sdk-go-v2/service/sesv2` `SendEmail` with a simple content (text and HTML parts), the configuration set and Reply-To.
- `html.go`: escape the text, turn line breaks into `<br>` and URLs into links, add the footer.
- `sns.go`: `VerifySNS(ctx, body []byte, client *http.Client) (Notification, error)`: checks `SignatureVersion` 1 or 2, that `SigningCertURL` is `https://sns.<region>.amazonaws.com/...pem`, fetches and caches the certificate, verifies the signature over the canonical string for `Notification` and `SubscriptionConfirmation`; `ConfirmSubscription` calls `SubscribeURL` (same host check).
- Tests against `httptest` servers: SES request fields, the log sender never printing the body, a valid and a tampered SNS message, a certificate URL on another host refused.

### 6. Store
- `internal/store/email.go` (new):
  - `queueGuestEmail(ctx, accountID, bookingID, kind, at)` used inside booking transactions.
  - `SendGuestEmails(ctx, limit, now, send func(ctx, []OutgoingEmail) []Result)`: claim in a short transaction, render each from the account's template (or the system text) with a fresh or opened token link, then commit; the caller sends; results are written after. A row whose stay no longer wants it, or whose template has email switched off, becomes `skipped`; a suppressed address becomes `suppressed`. Failures go back to `pending` with the error up to 3 tries, then `failed`.
  - `QueueTimedGuestEmails(ctx, now)`: the three `Due*` queries into the outbox.
  - `guestLink(ctx, booking)`: opens `guest_token_sealed`, or mints a token, stores hash and sealed copy, and returns the stay URL (the store is given the booking page base URL builder through an option).
  - `RecordSuppression(ctx, address, reason)`.
- Hooks: `CreateGuestBooking` queues `received`; `CreateBooking` queues `confirmed` for confirmed stays with a guest email; `moveTo` queues `confirmed` on a move to confirmed and `declined` on requested to declined. Page bookings seal the token they already make (`server/public.go` passes it in `NewBooking`).
- `UpdateTemplate` takes channels; `SetNotificationPrefs` saves `email`; `alertFor` fills `Recipient.Email`; `DueSummaries` returns it.
- Tests: a page booking queues one `received`; confirming queues one `confirmed` and doing it twice still one; a cancelled stay's reminder is skipped at send time; a suppressed address is not sent; the scheduler queues a pre-arrival once at 09:00 two days before in the property's time zone; a closed account sends nothing; an owner-created stay gets a working token in its link.

### 7. Handlers, jobs and config
- `internal/server/webhooks.go`: `SesWebhook` verifies, confirms subscriptions, and for `Bounce` (permanent) and `Complaint` events records each recipient. Duplicates are harmless.
- `internal/server/messages.go`: template patch with channels (each must be `email` or `whatsapp`); preferences with `email`.
- `internal/jobs/jobs.go`: `guest_emails` every minute (send), `schedule_guest_emails` every 15 minutes; `Alerts` gains `Email email.Sender`, used for team alerts and summaries when the person chose email and has an address, after push like WhatsApp.
- `internal/config/config.go`: the email settings; `cmd/api/main.go` builds the sender.
- `.env.example`, `turbo.json`: the new variables.
- Tests: a forged SNS message is 400 and records nothing; a team member with email on gets the alert through the fake sender.

### 8. Mobile app
- `src/app/template/[id].tsx`: "Send by email automatically" switch bound to `channels` (WhatsApp stays as it is); the timing line explains when it goes.
- `src/app/notifications.tsx`: an Email column; when the person has no email, the switches are off with a link to Profile to add one.
- `src/api/messages.ts`: the template patch and preferences carry the new fields.

### 9. Docs
- `docs/ROADMAP.md`: tick phase 8 items; the AWS account item stays open until production access.
- `docs/HANDOVER.md`: commits, maps (email package, outbox, SNS), the calls above, gotchas, gate status, production list (SES settings, the SNS subscription, DNS records for SPF, DKIM and DMARC).
- `README.md`: email in development (log sender) and how to try SES.

## Do not

- Send marketing or newsletters, or add unsubscribe management beyond switching a template's email off.
- Track opens or clicks, or add images to emails.
- Build a template editor for subjects or HTML; subjects come from the code, bodies from the templates.
- Email the phase 7 data export.
- Send WhatsApp messages to guests automatically.
- Retry a failed email more than 3 times or send from inside a booking transaction.
- Add a second email provider.

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...` -> passes, including `TestTenantIsolation`.
- `bun run generate && git diff --exit-code` -> no diff.
- `bunx biome check . && bun run typecheck` -> clean.
- `cd apps/mobile && bun test src && bunx expo export --platform ios` -> passes.
- Migration `00013` up, down and up on a seeded database.
- Against the seeded API with the log sender:
  - a booking on the page with an email logs one `received` send with a masked address, and the outbox row is `sent`;
  - approving a request logs `confirmed`; declining one logs `declined`;
  - with the clock two days before a stay's check-in at 09:00 property time (or `scheduled_for` moved in SQL), `prearrival` goes once;
  - switching the thank-you template's email off in the app marks that row `skipped`;
  - a posted SNS bounce signed with a test certificate (served through `httptest`) adds the address to `email_suppressions`, and the next email to it is `suppressed`.
- With SES sandbox credentials and a verified test address: a page booking delivers the received email with the stay link working, and the SES simulator's bounce address (`bounce@simulator.amazonses.com`) ends up suppressed through SNS.

## Risks

- **Duplicate emails when a job runs twice or a send times out.** -> Unique `(booking_id, kind)`, claim with `SKIP LOCKED`, and a timed-out send counts as sent only when SES returned a message id; otherwise retried at most 3 times.
- **Stale emails after a stay changes.** -> `StillWanted` checks the current stay at send time.
- **A forged bounce suppresses a real guest.** -> SNS signature verification with the certificate host pinned to `sns.<region>.amazonaws.com`.
- **Guest links stop working when tokens are re-minted.** -> Mint only when no sealed token exists; the page booking seals the token it returns.
- **SES sandbox only sends to verified addresses.** -> The gate uses verified addresses until production access is granted; the roadmap item stays open.
- **Emails from a closed account after deletion.** -> The claim query skips closed accounts.

## Went-sideways flags

- The same booking gets two `confirmed` rows, or one email per job run (the unique key or the claim isn't working).
- A guest email body, subject or full address appears in the API log.
- `POST /webhooks/ses` accepts a message whose `SigningCertURL` isn't on `amazonaws.com`.
- A guest's existing stay page link stops working after an email goes out (a token was minted over a sealed one).
