# Build phase 8: email through AWS SES

Drafted 18 Oct 2026 on top of the phase 7 branch (`feat/phase-7-account-controls`, PR #5). Merge PR #5 before starting. Written for the agent that executes it; the checklist is `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

Guests get their booking emails from the account's message templates, people get their account emails (welcome and email confirmation, email changes, new sign-ins, deletion and restore, web plan receipts), team members can take alerts and the morning summary by email, all through AWS SES, and an address that bounces or complains is never emailed again.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- **SES v2 through the AWS SDK** already in the module (`aws-sdk-go-v2`), with explicit keys like R2: `STAYKEY_EMAIL_PROVIDER` is `log` (default, also in production until SES is set up) or `ses`, plus `STAYKEY_SES_REGION`, `STAYKEY_SES_ACCESS_KEY_ID`, `STAYKEY_SES_SECRET_ACCESS_KEY`, `STAYKEY_SES_CONFIGURATION_SET`, and `STAYKEY_EMAIL_FROM` (an address on the verified domain, for example `hello@mail.staykey.direct`).
- **One outbox for every email,** `emails`: each row is one message to one address, with optional `account_id`, `booking_id` and `user_id`, a `kind`, and a `dedupe_key` that is unique when set. A job sends due rows outside any transaction, like the alert outbox. Team alerts and the morning summary keep going straight from the alert job, like WhatsApp, since they already have their own outbox (`activity.notified_at`).
- **Sign-in is by phone code; there are no passwords.** The account emails stand in for the usual password emails: a new sign-in notice instead of a password change notice, and email changes confirmed by a link.
- **Email addresses are confirmed before StayKey relies on them.** Adding or changing an email sends a confirmation link to the new address; until it is confirmed the address gets no alerts, summaries or account emails, and isn't used as Reply-To. Links last 7 days and work once.
- **Onboarding asks for an email** (optional) on the name step, and the first one confirmed gets the welcome email instead of a plain confirmation.
- **Account emails** (to the person, `user_id` set, only to a confirmed address unless noted):
  - `welcome`: the first email address a person adds; welcomes them and carries the confirmation link (sent to the unconfirmed address, since that is the point).
  - `confirm_email`: a later added or changed address gets the confirmation link (sent to the unconfirmed address).
  - `email_changed`: notice to the previous confirmed address that it was replaced, with the time.
  - `new_sign_in`: a sign-in on a device that hasn't signed in before (new `device_name` for that person), with the device name and time, and "If this wasn't you, sign in and delete the session" wording. Not sent for the very first sign-in.
  - `deletion_scheduled`: after `POST /v1/me/deletion`, with the date it becomes final and how to undo it.
  - `account_restored`: when signing in again undoes a deletion.
  - `deletion_completed`: when the purge finishes; the address is captured before the person's row is deleted.
  - `plan_receipt`: after a paid web period, with the plan, amount, period and the Payments.lk payment id. Store purchases get their receipts from Apple and Google.
- **Guest emails** (to the guest, `booking_id` set, only when the guest has an email; imported OTA stays never get any):
  - `received`: system email, when a guest books on the page or widget (stay page link, how to pay).
  - `confirmed`: template `confirmed`, when a stay moves to confirmed or an owner creates it confirmed.
  - `declined`: system email, when a request is declined or expires.
  - `reminder`: template `reminder`, 24 hours before `payment_due_at` for unpaid bank-transfer stays.
  - `prearrival`: template `prearrival`, at 09:00 property time two days before check-in, for confirmed and awaiting-payment stays.
  - `thanks`: template `thanks`, at 10:00 property time the day after check-out, for stays that weren't cancelled.
- **Template emails only go when the template's channels include `email`;** template channels become editable (`PATCH /v1/templates/{id}` takes `channels`). System and account emails always go.
- **Event emails are queued inside the transaction that causes them** (booking change, email change, sign-in, deletion, restore, purge, web payment). **Timed guest emails** are queued by a scheduling job every 15 minutes. Dedupe keys stop repeats: `<booking>:<kind>` for guest emails, `<user>:welcome` for the welcome email.
- **The send job checks guest emails again before sending:** a stay that was cancelled, moved or paid since queuing marks the row `skipped`.
- **From and Reply-To:** guest emails come from "<property name> via StayKey" with Reply-To the account owner's confirmed email; account emails come from "StayKey" with no Reply-To.
- **Links:** guest emails link to the stay page (`<booking page>/stay/<id>?t=<token>`); the token is stored sealed (`internal/secret`, bound to the booking id) beside its hash so the server can put it in emails, and owner-created stays get a token the first time an email needs one. Stays booked before this phase have no sealed token and get a fresh one (no real guests yet). Confirmation links go to `https://staykey.direct/verify-email/<token>` on the marketing site (`STAYKEY_VERIFY_EMAIL_URL`), which posts the token to the API.
- **Plain text plus a simple HTML part:** escaped text with line breaks and links, a footer with the property's booking page or StayKey's address. No images, no open or click tracking.
- **Team email alerts:** `notification_prefs.email` (default off), sent to a confirmed `users.email`. The morning summary can go by email too.
- **Bounces and complaints** come from SES through an SNS topic subscribed to `POST /webhooks/ses`. The handler verifies the SNS signature, confirms the subscription once, and records permanent bounces and complaints in `email_suppressions`. Every send checks that table first and marks the row `suppressed`. A suppressed address also loses its confirmed status, so the person is asked to add a working one. SES's account-level suppression list stays on as a backstop.
- **Development** uses the log sender, which records that an email was skipped with the address masked, never the body; in development it also logs confirmation links, so they can be opened locally.

## Context

- People: `users.email` exists with no confirmation (`00002_create_accounts_and_people.sql`); `PATCH /v1/me` sets it through `domain.UserPatch` (`internal/domain/account.go`) and `UpdateUser` (`internal/db/queries/users.sql`). `oapi.User` has `email`. Profile edits it (`apps/mobile/src/app/profile.tsx`); onboarding's name step doesn't ask for it (`apps/mobile/src/features/onboarding/steps/you.tsx`, `flow.ts:63`).
- Sign-in: `auth.Service.VerifyCode(ctx, phone, code, deviceName)` opens a session with the trimmed device name (`internal/auth/otp.go:122,167`) and calls `RestoreUser` after `UpsertUserByPhone`. Sessions store the device name (`00004_create_auth_tables.sql`).
- Deletion: `store.DeleteUser`, `RestoreUser`, `PurgeDeletedUsers` (`internal/store/accounts.go`).
- Web payments: `store.PayWebCheckout` returns `domain.PaidPeriod` (`internal/store/billing.go`).
- Templates: `domain.Template` and `DefaultTemplates` (`internal/domain/messages.go`) with keys `confirmed`, `reminder`, `prearrival`, `thanks` and placeholders `{guest_name}`, `{property_name}`, `{check_in}`, `{check_out}`, `{balance}`, `{booking_link}`, `{map_link}`, `{check_in_time}`. `message_templates.channels` is checked against `email`, `whatsapp`. `UpdateTemplateBody` only changes the body (`internal/db/queries/messages.sql:12`).
- Alerts: `store.SendAlerts` claims and sends after commit (`internal/store/activity.go:158`); `Recipient` has `Phone`, `PushTokens`, `WhatsApp`; `jobs.Alerts{Push, WhatsApp, Logger}`; `notification_prefs` has `push` and `whatsapp`; `ActivityRecipients` and `DueSummaries` skip closed accounts and deleted people.
- WhatsApp senders show the provider interface plus a masking log sender (`internal/notify/whatsapp.go`).
- Status changes go through `store.moveTo` (`internal/store/bookings.go:354`); owner stays are created in `CreateBooking`; page stays in `CreateGuestBooking` (`internal/store/public.go`).
- Guest tokens: `newBookingToken` (`internal/server/public.go:475`) stores only the SHA-256 (`bookings.guest_token_hash`). `secret.Box` has `Seal(plaintext, context)` and `Open`; the store holds one (`store.WithSecrets`).
- Booking page URLs: `Server.bookingPageURL(slug)` (`internal/server/properties.go:209`); the booking app maps `<slug>.<domain>/<path>` onto `/p/<slug>/<path>` (`apps/booking/src/proxy.ts`).
- Marketing pages that hand a token on: `apps/marketing/src/app/join/[token]/page.tsx`; server actions call the API through `src/lib/api.ts` (`server-only`).
- R2 shows how to build an AWS client with explicit keys (`internal/files/r2.go`, `config.R2`).
- Webhooks are spec routes with no security and `application/octet-stream` bodies; rate limits skip `/webhooks/` (`internal/server/limits.go`).
- The owner app: `src/app/template/[id].tsx` edits the body only; `src/app/notifications.tsx` has push and WhatsApp columns.

## Constraints

- Work in this order, one commit per step: migration, sqlc queries, domain with tests, OpenAPI, `bun run generate`, `internal/email`, store, handlers and jobs, config and wiring, marketing, mobile, docs.
- Add migration `00013` only, with a working `Down`. Never change `00001` to `00012`.
- No em dashes anywhere: code, comments, UI copy, email text, commit messages.
- Never edit `api.gen.go`, `internal/store/queries` or `packages/api-client/src/schema.ts` by hand.
- Never hold a transaction open across an SES or SNS call.
- Never log email bodies, subjects with names, tokens or full addresses; log the outbox id, user or booking id and a masked address.
- Every new environment variable goes in the app's `.env.example` and, for the API, `passThroughEnv` for `dev` in `turbo.json`; production refuses `STAYKEY_EMAIL_PROVIDER=ses` without its settings.
- Owner store methods take a `tenant.Tenant`; person methods take the principal's user id; job and webhook methods take ids from the rows they act on.
- Tests run against Postgres 17 through `storetest.New(t)`, with a fake sender and fixed clocks; SNS signature tests use a generated key and certificate served by `httptest`.
- Next 16 for the marketing page and Expo SDK 57 for the app: read `apps/marketing/AGENTS.md` and `apps/mobile/AGENTS.md`.

## Changes

### 1. `apps/api/internal/db/migrations/00013_create_email.sql` (new)
- `emails`: `id`, `account_id` (nullable, FK cascade), `booking_id` (nullable, composite FK to bookings, cascade), `user_id` (nullable, FK `ON DELETE SET NULL` so `deletion_completed` survives the purge), `kind` (the kinds in Assumptions), `to_address`, `payload` jsonb (values captured at queue time that may be gone at send time, such as a device name, a deletion date or a receipt), `dedupe_key` (unique where not null), `status` (`pending`, `sent`, `skipped`, `suppressed`, `failed`), `attempts`, `scheduled_for`, `claimed_at`, `sent_at`, `provider_message_id`, `error`, `created_at`; index on `scheduled_for` where `status = 'pending'`.
- RLS on `emails` as in 00011, except rows with no `account_id` are hidden from tenant requests (the policy already does that).
- `email_suppressions`: `address` (lower-cased, PK), `reason` (`bounce`, `complaint`), `created_at`.
- `email_confirmations`: `id`, `user_id` (cascade), `email`, `token_hash` (unique), `expires_at`, `used_at`, `created_at`.
- `users.email_confirmed_at timestamptz`.
- `notification_prefs.email boolean NOT NULL DEFAULT false`.
- `bookings.guest_token_sealed bytea`.
- `Down` reverses each.

### 2. sqlc queries
- `internal/db/queries/email.sql` (new): `QueueEmail` (`ON CONFLICT (dedupe_key) DO NOTHING`), `ClaimDueEmails` (`FOR UPDATE SKIP LOCKED`, sets `claimed_at`, skips rows of closed accounts), `MarkEmail`, `ReleaseEmail` (bumps `attempts`), `IsSuppressed`, `Suppress` (`ON CONFLICT DO NOTHING`, also clears `email_confirmed_at` on users with that address).
- Guest email data for rendering: `GuestEmailData` (booking, guest, property, account owner's confirmed email, the account's template for the kind).
- Timed guest emails: `DueReminders`, `DuePrearrivals`, `DueThanks`, returning stays with a guest email and no row of that kind, in the property's time zone as in `DueSummaries`.
- Confirmations: `CreateEmailConfirmation`, `UseEmailConfirmation` (`UPDATE ... SET used_at WHERE token_hash AND used_at IS NULL AND expires_at > now RETURNING user_id, email`), `ConfirmUserEmail` (sets `email_confirmed_at` only when `users.email` still equals the confirmed address).
- Sign-ins: `HasSignedInOnDevice` (an earlier session of the user with the same device name); `CountSessions` (to skip the first sign-in).
- `bookings.sql`: `SetGuestToken` (hash and sealed token).
- `users.sql`: `UpdateUser` clears `email_confirmed_at` when the email changes and returns the previous email and its confirmation.
- `messages.sql`: `UpdateTemplate` sets `body` and `channels`; `ActivityRecipients` and `DueSummaries` return the confirmed email and `p.email`; `SetNotificationPref` takes `email`.
- Run `bun run generate`.

### 3. Domain
- `internal/domain/email.go` (new): `EmailKind` and its constants; `RenderTemplate(body, values)` replacing placeholders (unknown ones left as they are); `ComposeEmail(kind, values) (subject, text)` for the system and account kinds, in plain words, with no amounts except in `plan_receipt`; due times `ReminderAt`, `PrearrivalAt`, `ThanksAt`; `StillWanted(kind, booking, now)`; `NewEmailConfirmationToken` (32 random bytes, SHA-256 stored), `ConfirmationTTL` (7 days).
- `User` gains `EmailConfirmed bool`; `NotificationPref` gains `Email`.
- Tests: every placeholder, each composed kind has a subject and no em dash, due times across a time zone, `StillWanted` for a cancelled, moved and paid stay.

### 4. `packages/api-spec/openapi.yaml`
- `User` gains `emailConfirmed`.
- `POST /v1/me/email-confirmation` (`bearerAuth`): sends the confirmation again; 204; 409 `email_confirmed` when already confirmed; 400 when there is no email.
- `POST /v1/public/email-confirmation`: body `{token}`; 200 with `{email}`; 404 for an unknown, used or expired token.
- `TemplatePatch` gains `channels`; `NotificationPref` gains `email`.
- `POST /webhooks/ses` (no security, `application/octet-stream`, 200; 400 when the signature fails).
- Run `bun run generate`.

### 5. `apps/api/internal/email` (new package)
- `email.go`: `Message{To, FromName, ReplyTo, Subject, Text, HTML}`; `Sender` (`Send(ctx, Message) (messageID string, err error)`); `LogSender` (masks the address; in development also logs a confirmation link it finds, flagged by the caller).
- `ses.go`: `SES{Region, AccessKeyID, SecretAccessKey, From, ConfigurationSet}` with `github.com/aws/aws-sdk-go-v2/service/sesv2` `SendEmail` (simple content, text and HTML, configuration set, Reply-To).
- `html.go`: escape, line breaks, links, footer.
- `sns.go`: `VerifySNS(ctx, body, client)` (signature versions 1 and 2, `SigningCertURL` pinned to `https://sns.<region>.amazonaws.com/`, certificate cached), `ConfirmSubscription` (same host check).
- Tests against `httptest`: SES request fields, the log sender never printing a body, a valid and a tampered SNS message, a certificate URL on another host refused.

### 6. Store
- `internal/store/email.go` (new):
  - `queueEmail(ctx, Email)` used inside the transactions that cause emails.
  - `SendEmails(ctx, limit, now, send)`: claim in a short transaction, render each (guest emails from the template or system text with the stay link; account emails from `payload`), commit, let the caller send, then record results. Guest rows whose stay no longer wants them, or whose template has email off, become `skipped`; suppressed addresses `suppressed`; failures go back to `pending` up to 3 attempts, then `failed`.
  - `QueueTimedGuestEmails(ctx, now)`.
  - `guestLink(ctx, booking)`: opens `guest_token_sealed`, or mints and stores a token, and returns the stay URL (the base URL builder comes in through an option).
  - `RecordSuppression(ctx, address, reason)`.
  - `ConfirmEmail(ctx, tokenHash, now)`, `ResendConfirmation(ctx, userID, now)`.
- Hooks:
  - `CreateGuestBooking` queues `received` (and seals the token the server already makes, passed in `NewBooking`); `CreateBooking` queues `confirmed` for confirmed stays; `moveTo` queues `confirmed` on a move to confirmed and `declined` on requested to declined.
  - `UpdateUser`: on a new or changed email, creates a confirmation and queues `welcome` (first ever address) or `confirm_email`, and `email_changed` to the previous confirmed address.
  - Sign-in: `auth.Store` gains `NoteSignIn(ctx, userID, deviceName, now)`, called after the session opens, which queues `new_sign_in` when the person has signed in before but never on that device name.
  - `DeleteUser` queues `deletion_scheduled`; `RestoreUser` queues `account_restored`; `PurgeDeletedUsers` queues `deletion_completed` (with the address captured) before deleting the person; `PayWebCheckout` queues `plan_receipt` to the owner's confirmed email.
- `UpdateTemplate` takes channels; `SetNotificationPrefs` saves `email`; `alertFor` fills `Recipient.Email` (confirmed only); `DueSummaries` returns it.
- Tests: a page booking queues one `received`; confirming twice still queues one `confirmed`; a cancelled stay's reminder is skipped at send time; a suppressed address isn't sent and loses its confirmation; the scheduler queues one pre-arrival at 09:00 two days before in the property's time zone; a closed account sends nothing; the first email gets `welcome` and a change gets `confirm_email` plus `email_changed`; a used or expired confirmation token is refused; a second device queues `new_sign_in` and the same device doesn't; deleting, restoring and purging each queue their email; a paid web period queues one receipt.

### 7. Handlers, jobs and config
- `internal/server/webhooks.go`: `SesWebhook` verifies, confirms subscriptions, and records each recipient of `Bounce` (permanent) and `Complaint`.
- `internal/server/accounts.go`: `ResendEmailConfirmation` (limited to 3 per hour per person in `limits.go`), `ConfirmEmail` (public; 120 per minute per address like other public routes); `User.emailConfirmed`.
- `internal/server/messages.go`: template channels (each `email` or `whatsapp`); preferences with `email`.
- `internal/jobs/jobs.go`: `send_emails` every minute, `schedule_guest_emails` every 15 minutes; `Alerts` gains `Email email.Sender` for team alerts and summaries, sent after push like WhatsApp.
- `internal/config/config.go`: email settings and `STAYKEY_VERIFY_EMAIL_URL` (default `https://staykey.direct/verify-email`, `http://localhost:3000/verify-email` in development); `cmd/api/main.go` builds the sender; `.env.example` and `turbo.json`.
- Tests: a forged SNS message is 400 and records nothing; confirming through the public route marks the person confirmed; a team member with email on and a confirmed address gets the alert through the fake sender, and an unconfirmed one doesn't.

### 8. Marketing site
- `src/app/verify-email/[token]/page.tsx` (new): a server action posts the token to `POST /v1/public/email-confirmation` and shows "Your email is confirmed" or "This link has expired or was already used. Send a new one from Profile in the app." Not indexed; the token never reaches the browser's address bar after the redirect.
- `src/lib/api.ts`: `confirmEmail(token)`.

### 9. Mobile app
- Onboarding name step: an optional "Email for booking updates" field saved with the name.
- `src/app/profile.tsx`: under the email, "Not confirmed yet. Check your inbox." with "Send again"; a change shows that a confirmation was sent.
- `src/app/notifications.tsx`: an Email column, off and explained ("Confirm your email in Profile first") while the address isn't confirmed.
- `src/app/template/[id].tsx`: "Send by email automatically" bound to `channels`.
- `src/api/me.ts` and `src/api/messages.ts`: the resend call and the new fields.

### 10. Docs
- `docs/ROADMAP.md`: tick phase 8 items, adding the account emails; the AWS account item stays open until production access.
- `docs/HANDOVER.md`: commits, maps (email package, outbox, confirmations, SNS), the calls above, gotchas, gate status, production list (SES settings, the SNS subscription, DNS records for SPF, DKIM and DMARC, `STAYKEY_VERIFY_EMAIL_URL`).
- `README.md`: email in development (log sender, confirmation links in the log) and how to try SES.

## Do not

- Add passwords, password resets or email sign-in; sign-in stays by phone code.
- Send marketing or newsletters, or add unsubscribe management beyond a template's email switch and the alert preferences.
- Track opens or clicks, or add images.
- Build an editor for subjects, HTML or account email text.
- Email the phase 7 data export.
- Send WhatsApp messages to guests automatically.
- Send store purchase receipts (Apple and Google send them).
- Retry a failed email more than 3 times or send from inside a transaction.
- Add a second email provider.

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...` -> passes, including `TestTenantIsolation`.
- `bun run generate && git diff --exit-code` -> no diff.
- `bunx biome check . && bun run typecheck` -> clean.
- `cd apps/mobile && bun test src && bunx expo export --platform ios` and `cd apps/marketing && bun run build` -> pass.
- Migration `00013` up, down and up on a seeded database.
- Against the seeded API with the log sender:
  - adding an email in Profile logs a `welcome` with a confirmation link; opening it on `localhost:3000/verify-email/<token>` confirms it, and opening it again says it was used;
  - changing the email logs `confirm_email` to the new address and `email_changed` to the old one;
  - signing in from a second device name logs `new_sign_in`; signing in again from it doesn't;
  - deleting logs `deletion_scheduled`; signing in again logs `account_restored`; a purge logs `deletion_completed`;
  - a booking on the page with an email logs `received`; approving a request logs `confirmed`; declining logs `declined`; moving `scheduled_for` back sends `prearrival` once; switching the thank-you template's email off marks it `skipped`;
  - a paid web period logs `plan_receipt`;
  - a posted SNS bounce signed with a test certificate suppresses the address, clears its confirmation, and the next email to it is `suppressed`.
- With SES sandbox credentials and verified test addresses: the welcome email arrives with a working link, a page booking's email links to a working stay page, and the SES simulator's bounce address ends up suppressed through SNS.

## Risks

- **Duplicate emails when a job runs twice or a send times out.** -> Dedupe keys, `SKIP LOCKED` claims, and a row counts as sent only with an SES message id; at most 3 attempts.
- **Stale guest emails after a stay changes.** -> `StillWanted` at send time.
- **Alerts or receipts going to someone else's address.** -> Only confirmed addresses get anything beyond the confirmation itself; a bounce or complaint clears the confirmation.
- **A forged bounce suppresses a real person.** -> SNS signatures with the certificate host pinned.
- **New sign-in emails for every app reinstall.** -> Keyed on device name, not session; the first ever sign-in sends nothing.
- **Guest links stop working when tokens are re-minted.** -> Mint only when no sealed token exists.
- **SES sandbox only sends to verified addresses.** -> The gate uses verified addresses until production access; the roadmap item stays open.

## Went-sideways flags

- Two `confirmed` rows for one stay, or one `welcome` per email change (dedupe keys not applied).
- A team alert, receipt or account email logged for an address whose `email_confirmed_at` is null.
- An email body, subject with a name, token or full address in the API log.
- `POST /webhooks/ses` accepts a message whose `SigningCertURL` isn't on `amazonaws.com`.
- A guest's stay page link stops working after an email goes out.
