# Build phase 4: team, activity and messages

Drafted 11 Oct 2026 against `main` at `a301dc5`, and carried out the same day (commits `bf65c2a` to `3bf2855`; see `docs/HANDOVER.md`). Kept as the record of what phase 4 set out to do. The wider plan is `docs/backend-plan.md`, the checklist `docs/ROADMAP.md`.

## Goal

A caretaker invited from the Team screen opens a WhatsApp link, joins the owner's account limited to their property, and gets a push alert when a stay is booked there. Activity, team, templates, notification preferences and profile all come from the API, and `apps/mobile/src/data/store.ts` is deleted.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- The invite link is `https://staykey.direct/join/<token>`, a marketing page that opens `staykey://join?token=<token>`. Universal links and app links wait until hosting is decided.
- Invites last 14 days, as `cmd/seed` already does. Only owners manage the team (`domain.PermTeam`). A new invite to the same phone revokes the open one.
- Accepting needs the signed-in phone to equal the invited phone. A mismatch answers 404, like an unknown token.
- Read state is one `last_read_at` per user per account. Opening Activity marks everything read; the per-item and per-booking marks go away.
- The server writes activity text, in the third person with the actor's first name ("Nadeesha approved Sam Lee") or as the system ("Request expired").
- Caretakers see activity for their properties only, and never `payment` or `slip` items. No other kind carries an amount.
- Alerts leave through an outbox: `activity.notified_at`, drained by a river sweep every 15 seconds. Nothing is inserted into river inside the booking transaction.
- Push goes through the Expo push API without polling receipts. `DeviceNotRegistered` deletes the token.
- WhatsApp alerts use a sender with a `log` default and a `cloud` option that sends one approved template. The email sender waits for an email provider.
- The morning summary goes out at 07:00 in the time zone of the account's first property (default `Asia/Colombo`), once per user per account per day, through the channels on that user's `summary` preference.
- Push tokens belong to a session. Fan-out skips tokens whose session is revoked or expired, so logging out stops alerts on that device.
- Templates (four per account) and notification preferences (six per membership) are created as rows when the account or membership is created. Migration `00008` backfills existing rows, so reads never need defaults.
- The profile photo stays on the device, in `useSession`. Insights needs no endpoint: it already works from `useBookings`.

## Context

- These tables already exist: `invites` and `membership_properties` (`00003_scope_properties_to_accounts.sql`), `push_tokens` and `notification_prefs` (`00004_create_auth_tables.sql`). Still to create: `activity`, `activity_reads`, `message_templates`.
- `store.Postgres.AddMember`, `store.Invite` and `store.Postgres.CreateInvite` are in `apps/api/internal/store/accounts.go:121-180`. Only `cmd/seed` and the tests call them.
- `inviteToken()` in `apps/api/cmd/seed/main.go:305` makes 32 random bytes encoded base64url and stores their SHA-256. The seed prints `staykey://join?token=...` for `pendingInvite` (+94715550199, caretaker, villa only).
- `domain.Role.Can` (`apps/api/internal/domain/account.go`): owners can do everything, managers everything but `PermBilling` and `PermTeam`, caretakers nothing.
- These store functions change a stay: `CreateBooking`, `TransitionBooking` (through `moveTo` at `store/bookings.go:279`), `CancelBooking`, `RecordPayment` and `RejectSlip` in `store/bookings.go`, and `closeBySystem` in `store/jobs.go` for expired requests and unpaid stays. Each one runs in `InTx`.
- Periodic jobs live in the `sweeps` map in `apps/api/internal/jobs/jobs.go`. `jobs.Start` is called from `apps/api/cmd/api/main.go`.
- A route's security comes from the spec. Every `accountHeader` route is picked up by `TestTenantIsolation` automatically, and every new path parameter needs an entry in `aliceIDs` (`internal/server/isolation_test.go:97`). Bob's own account plus Alice's ID must give 404, so path IDs must be account-unique UUIDs.
- `auth.Principal` carries `SessionID` (`internal/auth/auth.go:215`). Get it in handlers with `principal(ctx)`, and the tenant with `currentTenant(ctx)` (`internal/server/security.go`).
- `apps/mobile/src/data/store.ts` (zustand, persisted as `staykey.data`, version 3) has 18 importers: `app/team.tsx`, `activity.tsx`, `subscription.tsx`, `templates.tsx`, `template/[id].tsx`, `notifications.tsx`, `profile.tsx`, `onboarding.tsx`, `_layout.tsx`, `(tabs)/index.tsx`, `(tabs)/more.tsx`, `booking/contact.tsx`, `booking/new.tsx`, `booking/payment.tsx`, `booking/cancel.tsx`, `features/bookings/move.ts`, `features/auth/enter.ts` and `features/prototype/sample-guesthouse.ts`.
- `app/join.tsx` shows a hard-coded `INVITE`. `useJoining` lives in `features/auth/enter.ts`.
- The default templates are in `apps/mobile/src/data/defaults.ts:100-129` and the default notification preferences in `defaults.ts:151-161`.
- `features/today/checklist.tsx:24` `checklistFor(p, team)` ticks the caretaker step from the team list.
- The query cache buster is `"phase-3"` in `apps/mobile/src/api/query-client.ts:33`. `useToday`, `sampleTeam` and `sampleData` have no importers outside `store.ts` and `data/seed.ts`.
- `expo-notifications` is not installed, and `app.json` has no `extra.eas.projectId`.
- `apps/api/internal/sms/ozone.go` and `sms_test.go` are the pattern for an HTTP sender: `URL` and `Client` fields with defaults, tested against `httptest`.

## Constraints

- Work in this order, one commit per step: migration, sqlc queries, domain with tests, OpenAPI, `bun run generate`, store, handlers, jobs, client hooks, screens, docs.
- No em dashes anywhere: code, comments, UI copy, commit messages.
- Never edit `api.gen.go`, `internal/store/queries` or `packages/api-client/src/schema.ts` by hand.
- Add migration `00008` only, with a working `Down`. Never change `00001` to `00007`.
- Every business table carries `account_id`. Child rows use composite foreign keys `(id, account_id)`. Owner store methods take a `tenant.Tenant`.
- Wrap Go errors with `%w`, never discard one with `_`, pass `context.Context` first, keep initialisms in one case (`userID`).
- Every new environment variable goes in the API tasks' `passThroughEnv` in `turbo.json` and in `apps/api/.env.example`.
- Mobile: add packages with `bunx expo install`. Never return a new array or object from a zustand selector. TanStack `select` functions are module level.
- Tests check behaviour, against real Postgres through `storetest.New(t)`, with a fixed clock for jobs.
- Any text a caretaker can see carries no money.

## Changes

### 1. `apps/api/internal/db/migrations/00008_create_activity_and_messages.sql` (new)
- `activity`: `id`, `account_id` (FK accounts, cascade), `property_id` (nullable), `booking_id` (nullable), `kind` (CHECK in `request`, `booking`, `slip`, `payment`, `cancellation`, `sync`, `import`), `title`, `subtitle`, `actor_user_id` (FK users, set null), `notified_at timestamptz`, `created_at`. Composite FKs `(property_id, account_id)` and `(booking_id, account_id)` with cascade.
- Indexes: `(account_id, created_at DESC, id DESC)`, plus a partial index on `created_at` `WHERE notified_at IS NULL`.
- `activity_reads`: PK `(account_id, user_id)`, `last_read_at timestamptz NOT NULL`, both FKs cascade.
- `message_templates`: `id`, `account_id`, `key` (CHECK in `confirmed`, `reminder`, `prearrival`, `thanks`), `name`, `channels text[]` (each `email` or `whatsapp`), `timing`, `body` (1 to 1000 chars), timestamps, `UNIQUE (account_id, key)`, `UNIQUE (id, account_id)`, an `updated_at` trigger using `set_updated_at()`.
- `ALTER TABLE notification_prefs ADD COLUMN last_sent_on date` (used by the `summary` row).
- `ALTER TABLE push_tokens ADD COLUMN session_id uuid REFERENCES sessions (id) ON DELETE CASCADE`.
- Backfill the four templates per existing account and six preference rows per existing active membership, with the values from `defaults.ts`. Use `gen_random_uuid()` for these rows only, and say so in a comment.
- `Down` drops the new tables and columns.

### 2. sqlc queries
- `apps/api/internal/db/queries/team.sql` (new): list members with user name, phone and `property_ids` (same aggregate as `GetMembership`), list open invites, get an invite by token hash with the account name, the inviter's name and the property names, accept an invite (`UPDATE ... WHERE accepted_at IS NULL AND revoked_at IS NULL AND expires_at > @now RETURNING *`), revoke one invite, revoke the open invites for a phone, update a membership's role, delete a membership's properties, delete a membership. Every write filters by `account_id`.
- `apps/api/internal/db/queries/activity.sql` (new): insert; list newest first with a `(created_at, id)` cursor, an optional `property_ids` filter (rows with a null `property_id` are always visible) and an excluded-kinds array; unread count since `last_read_at`; upsert `last_read_at`; claim unnotified rows `FOR UPDATE SKIP LOCKED LIMIT 100`; mark notified; recipients for an activity (active members of the account except the actor, in property scope, with that event's push and whatsapp flags and their push tokens on live sessions).
- `apps/api/internal/db/queries/messages.sql` (new): insert, list and update a template body; list, insert and upsert notification preferences; upsert a push token (`ON CONFLICT (token) DO UPDATE` moves `user_id`, `session_id` and `platform`); delete push tokens by token; due summaries; mark a summary sent.
- Run `bun run generate`.

### 3. Domain
- `apps/api/internal/domain/team.go` (new): `InviteTTL = 14 * 24 * time.Hour`, `NewInviteToken() (token string, hash []byte, err error)` (moved from the seed's `inviteToken`), `HashInviteToken(token string) []byte`, `TeamMember`, `Invite`, `InvitePreview`, and `MemberPatch{Role *Role; PropertyIDs *[]uuid.UUID}` whose `Validate` allows only `manager` and `caretaker`. Add `ErrOwnerMembership` for edits aimed at an owner.
- `apps/api/internal/domain/activity.go` (new): `ActivityKind` constants, `Activity`, `NotificationEvent` constants matching the CHECK in `00004`, `EventFor(ActivityKind) NotificationEvent` (`slip` goes to `payment`, `import` to `sync`), `HiddenFromCaretakers` = `payment` and `slip`, and text builders for: booking created, request approved or declined, request expired, cancelled by a person or for non-payment, payment recorded, slip rejected.
- `apps/api/internal/domain/messages.go` (new): `Template`, `DefaultTemplates()` with the four bodies copied verbatim from `defaults.ts:100-129`, `DefaultNotificationPrefs` copied from `defaults.ts:151-161`, and body validation.
- `apps/api/internal/domain/activity_test.go` and `team_test.go` (new): table tests for `EventFor`, the text builders (no amount outside payment and slip) and `MemberPatch.Validate`.

### 4. Store
- `apps/api/internal/store/accounts.go`
  - `CreateAccount`: insert `DefaultTemplates()` and the owner's six preference rows in the same transaction.
  - `AddMember`: insert the new member's preference rows.
  - `CreateInvite`: revoke the open invites for the same phone first, and return the new invite's id.
- `apps/api/internal/store/team.go` (new)
  - `ListTeam(ctx, t)`: members and open invites.
  - `PreviewInvite(ctx, tokenHash, now)`: no tenant.
  - `AcceptInvite(ctx, userID, phone, tokenHash, now) (domain.AccountMembership, error)`: lock the invite and check the phone; an existing member is returned as is and the invite marked accepted; anyone else becomes a member through `AddMember`'s inserts.
  - `UpdateMember(ctx, t, membershipID, patch)`, `RemoveMember(ctx, t, membershipID)`, `RevokeInvite(ctx, t, inviteID)`: return `domain.ErrOwnerMembership` for owner rows and `domain.ErrNotFound` outside the account.
- `apps/api/internal/store/activity.go` (new)
  - Unexported `addActivity(ctx, accountID uuid.UUID, a domain.Activity) error`, called inside the existing transactions: `CreateBooking` (`booking`, or `request` when requested), `TransitionBooking` (approve and decline only; check-in and check-out log nothing), `CancelBooking`, `RecordPayment`, `RejectSlip` and `closeBySystem`.
  - `ListActivity(ctx, t, cursor, limit)`: applies `t.PropertyScope()` and, when `!t.Can(domain.PermPrices)`, excludes `HiddenFromCaretakers`.
  - `UnreadActivity(ctx, t)` and `MarkActivityRead(ctx, t, now)`.
  - For the jobs: `ClaimNotifications(ctx, limit)`, `MarkNotified(ctx, ids, now)`, `DeletePushTokens(ctx, tokens)`.
- `apps/api/internal/store/messages.go` (new): `ListTemplates(ctx, t)`, `UpdateTemplate(ctx, t, id, body)`, `NotificationPrefs(ctx, t)` and `SetNotificationPrefs(ctx, t, prefs)` (the caller's own rows), `SavePushToken(ctx, userID, sessionID, token, platform)`, `DueSummaries(ctx, now)`, `MarkSummarySent(ctx, membershipID, day)`.
- `apps/api/internal/store/team_test.go` and `activity_test.go` (new), covering:
  - the wrong phone, an expired invite and a revoked invite each give not found;
  - accepting twice gives one membership;
  - a caretaker's listing has no payment items and no other property;
  - approving a request writes exactly one activity row;
  - `MarkActivityRead` zeroes the unread count.

### 5. `apps/api/internal/notify/` (new package)
- `expo.go`: `Expo{URL, Client, AccessToken}` with `Send(ctx, []Push) (dead []string, err error)`. It posts batches of 100 to `https://exp.host/--/api/v2/push/send` and returns the tokens answered with `DeviceNotRegistered`. No retries in the sender; river retries the job.
- `whatsapp.go`: `LogWhatsApp` and `CloudWhatsApp{Token, PhoneNumberID, Template, Language, URL, Client}`. `CloudWhatsApp` sends the approved template with the alert title and subtitle as body parameters, on the Graph API version pinned from Meta's current docs at implementation time.
- `expo_test.go` and `whatsapp_test.go`: `httptest` servers, following `sms_test.go`.

### 6. `apps/api/internal/jobs/jobs.go`
- Pass a notify dependency (`Expo`, the WhatsApp sender, a logger) into `Start`.
- Add a `notify` sweep every 15 seconds:
  - claim rows, send each recipient's push and WhatsApp messages per their preferences, delete dead tokens, then mark the rows notified;
  - if a send fails, return the error and leave the rows unmarked.
- Add a `morning_summary` sweep every 15 minutes. For each due membership, send "Today: N arrivals, N departures, N requests waiting" for its property scope, then call `MarkSummarySent`.
- Extend `jobs_test.go` to run both sweeps once with a fixed clock against `httptest` senders.

### 7. Config and wiring
- `apps/api/internal/config/config.go` adds:
  - `STAYKEY_EXPO_ACCESS_TOKEN` (optional);
  - `STAYKEY_WHATSAPP_PROVIDER` (`log` or `cloud`, default `log`), plus `STAYKEY_WHATSAPP_TOKEN`, `STAYKEY_WHATSAPP_PHONE_NUMBER_ID` and `STAYKEY_WHATSAPP_TEMPLATE`, which `cloud` requires;
  - `STAYKEY_INVITE_URL` (default `https://staykey.direct/join`).
- `apps/api/cmd/api/main.go`: build the senders and pass them to `jobs.Start`; pass the invite URL to `server.Options`.
- `apps/api/.env.example`, `turbo.json`: add the variables.

### 8. `packages/api-spec/openapi.yaml`
- Owner routes (`bearerAuth` plus `accountHeader`):
  - `GET /v1/team` returns `{members, invites}`;
  - `POST /v1/team/invites` takes `{phone, name?, role, propertyIds}` and answers 201 with `{invite, link}`; the link is only returned here;
  - `DELETE /v1/team/invites/{inviteId}`;
  - `PATCH /v1/team/members/{membershipId}` takes `{role?, propertyIds?}`;
  - `DELETE /v1/team/members/{membershipId}`;
  - `GET /v1/activity?cursor&limit` returns `{items, unread, nextCursor}`;
  - `POST /v1/activity/read`;
  - `GET /v1/templates` and `PATCH /v1/templates/{templateId}` with `{body}`;
  - `GET` and `PUT /v1/notification-prefs`.
- Signed-in routes without an account (`bearerAuth` only):
  - `POST /v1/invites/accept` takes `{token}` and returns `Account`;
  - `POST /v1/push-tokens` takes `{token, platform}`.
- Public route (no security): `POST /v1/public/invites/preview` takes `{token}` and returns `{accountName, propertyNames, role, invitedByName, expiresAt}`, or 404.
- Tokens travel in bodies, never in paths.
- Team writes need `PermTeam` (403 otherwise), and template writes need `PermSettings`. Preferences are the caller's own, for any role.
- Run `bun run generate`.

### 9. Handlers
- `apps/api/internal/server/server.go`: add the new store methods to `Store`, and `InviteURL` to `Options`.
- `apps/api/internal/server/team.go`, `activity.go`, `messages.go` (new): thin handlers that map domain errors the way `bookings.go` does. `POST /v1/push-tokens` takes the session from `principal(ctx).SessionID`.
- `apps/api/internal/server/isolation_test.go`: add `inviteId`, `membershipId` and `templateId` to `aliceIDs`, with Alice's rows created in the fixture.
- `apps/api/internal/server/team_test.go` and `messages_test.go` (new), covering:
  - the full flow: invite, preview, accept with the matching phone, then the caretaker lists only the villa;
  - a mismatched phone gets 404;
  - a manager gets 403 on `POST /v1/team/invites`;
  - a caretaker gets 403 on `PATCH /v1/templates/{templateId}`.

### 10. `apps/api/cmd/seed/main.go`
- Use `domain.NewInviteToken` and delete `inviteToken`. Print both `https://staykey.direct/join/<token>` and `staykey://join?token=<token>`.

### 11. `apps/marketing/src/app/join/[token]/page.tsx` (new)
- Read `apps/marketing/AGENTS.md` first (Next 16).
- A static page with "Open StayKey to join", linking to `staykey://join?token=<token>`, plus one line telling people to install the app first. No API call.

### 12. Mobile API layer
- `bunx expo install expo-notifications`, and add `"expo-notifications"` to `plugins` in `apps/mobile/app.json`.
- `apps/mobile/src/api/team.ts` (new): `useTeam`, `useInvite` (returns the link), `useUpdateMember`, `useRemoveMember`, `useRevokeInvite`, `previewInvite`, `acceptInvite`.
- `apps/mobile/src/api/activity.ts` (new): `useActivity` (infinite), `useUnreadCount`, `useMarkActivityRead`.
- `apps/mobile/src/api/messages.ts` (new): `useTemplates`, `useUpdateTemplate`, `useNotificationPrefs`, `useSetNotificationPrefs` (optimistic).
- `apps/mobile/src/api/bookings.ts`: the existing mutations' `onSuccess` also invalidates the activity keys.
- `apps/mobile/src/lib/push.ts` (new): after sign-in, ask for permission, call `getExpoPushTokenAsync` with `Constants.expoConfig?.extra?.eas?.projectId`, and send `POST /v1/push-tokens`. Skip on web or when there is no project id, warning once. Add a response listener that routes to `/booking/[id]` when the data has `bookingId`, otherwise to `/activity`.
- `apps/mobile/src/api/query-client.ts`: buster `"phase-4"`.

### 13. Mobile screens
- `app/team.tsx`: members and invites from `useTeam`. Inviting calls `useInvite`, then `openWhatsApp(phone, message)` with the returned link. Pending invites can be revoked. "You" is the row whose user is the session's user.
- `app/join.tsx`: read `token` with `useLocalSearchParams`, preview it from the API, and show the account, inviter, role and properties. Accepting stores the token in `useJoining` and runs the existing code steps.
- `features/auth/enter.ts` `afterSignIn`: when joining with a token, call `acceptInvite` first and pick that account. Remove every `useData` call.
- `app/activity.tsx`: `useActivity`; mark read on focus with `useFocusEffect`.
- `app/templates.tsx`, `app/template/[id].tsx`, `app/booking/contact.tsx`: `useTemplates` and `useUpdateTemplate`.
- `app/notifications.tsx`: `useNotificationPrefs` and `useSetNotificationPrefs`.
- `app/profile.tsx`: `useMe` and `useUpdateMe` for name, email and language; photo in `useSession`.
- `app/(tabs)/more.tsx`:
  - unread from `useUnreadCount`;
  - when `useMe().data.accounts.length > 1`, an account row that opens `Menu`; switching calls `useSession.getState().setAccount(id, role)`, clears the query cache and replaces to the tabs;
  - remove `resetData`.
- `app/(tabs)/index.tsx` and `features/today/checklist.tsx`: unread from `useUnreadCount`. The caretaker step is done when `useTeam` has a caretaker member or invite; change `checklistFor`'s `team` parameter to the API type.
- `app/subscription.tsx`: the member count comes from `useTeam`.
- `app/booking/new.tsx`, `payment.tsx`, `cancel.tsx`, `features/bookings/move.ts`: delete the `log` and `markRead` calls.
- `app/onboarding.tsx`, `features/prototype/sample-guesthouse.ts`, `app/_layout.tsx`: drop `useData`. In `_layout.tsx`, drop `useHydrated(useData)` and remove the `staykey.data` storage key once on boot.
- Delete `apps/mobile/src/data/store.ts`, then `sampleTeam`, and `sampleData` if nothing else uses it.
- `apps/mobile/src/data/types.ts`: delete `Activity`, `TeamMember`, `Template` and `Account` once nothing imports them; screens use `@staykey/api-client` types.

### 14. Docs
- `docs/ROADMAP.md`: tick the phase 4 items.
- `docs/HANDOVER.md`: commits, API and app maps, the assumptions above as calls, gotchas, gate status.
- `docs/backend-plan.md`: the status line.
- `README.md`: how to try invites and push.

## Do not

- Add `GET /v1/insights`. Insights already computes from `useBookings`.
- Set up universal links, app links or associated domains. They need production hosting.
- Poll Expo push receipts, or add retries inside the senders.
- Build the email sender or automatic guest messages from templates (confirmation, reminders, pre-arrival). Templates are edited and used from the contact sheet only, and no provider is chosen yet.
- Add account rename, account deletion, ownership transfer or a second owner.
- Add per-item read state.
- Rate-limit the public invite preview. That belongs to phase 6.
- Upload profile photos.
- Split river into a worker mode, or insert river jobs inside store transactions.
- Touch `apps/booking` or `packages/widget`. They belong to phase 5.

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...` -> passes on Postgres 17. `TestTenantIsolation` lists the new team, activity, template and preference routes, and `TestOneWinnerOutOfFiftyParallelBookings` still passes.
- `bun run generate && git diff --exit-code` -> no diff.
- `bunx biome check . && bun run typecheck` -> clean.
- `cd apps/mobile && bun test src && bunx expo export --platform ios` -> passes.
- `grep -rn "data/store" apps/mobile/src` -> no matches.
- `bun run db:seed` -> prints a join link. Opening `staykey://join?token=...` on a simulator and signing in as +94715550199 lands on Today with Kingfisher Villa only.
- Gate, on two phones with a development build:
  - the owner invites a caretaker from Team, and WhatsApp opens with the link;
  - the caretaker taps it, sees the right property and inviter, and signs in with a code;
  - the owner creates a stay at that property, and the caretaker's phone shows a push within about 15 seconds;
  - the owner's own phone gets none for that stay.

## Risks

- Remote push needs a development build and an EAS project id. Expo Go doesn't deliver remote push on Android. -> Run `bunx eas-cli init` and make a development build before working on push; until then the app skips registration with one warning.
- WhatsApp Cloud only sends business-initiated messages through an approved template, and there is no Business number yet. -> Keep `log` as the default; the gate relies on push alone.
- A send that succeeds followed by a failed `MarkNotified` sends the alert twice when the job retries. -> Accepted; a duplicate alert is better than a lost one. Mark the rows right after the send.
- Writing activity inside booking transactions lengthens them a little. -> Rerun `TestOneWinnerOutOfFiftyParallelBookings` under `-race` ten times.
- An amount could leak into a caretaker-visible subtitle. -> A store test lists activity as a caretaker after a payment and checks that no subtitle contains the currency code.
- The persisted query cache may hold old shapes, or the old `staykey.data` key may linger. -> Bump the buster and remove the key on boot.

## Went-sideways flags

- `TestTenantIsolation` fails on an unknown path parameter (missing `aliceIDs` entry), or Bob gets 200 on one of Alice's IDs (a new query without `account_id`).
- A caretaker's Activity shows a payment line or an amount.
- Approving a request produces two activity rows or none (the app still logs locally, or the store call sits outside the transaction).
- After signing in, the app still shows the sample team or old alerts (the store was not removed, or the buster was not bumped).
- A push reaches the phone of the person who made the change.
