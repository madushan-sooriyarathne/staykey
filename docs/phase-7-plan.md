# Build phase 7: gaps noted in the handover

Drafted 17 Oct 2026 on top of the phase 6 branch (`feat/phase-6-billing-hardening`, PR #4). Merge PR #4 before starting. Written for the agent that executes it; the checklist is `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

An owner can delete themselves and their accounts from the app, export an account's data, erase a guest's details, rename the account, choose how a refund goes back, and keep a profile photo on the server; invite links open the app directly once `staykey.direct` is live.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- **Deletion is immediate and permanent.** `DELETE /v1/me` deletes the person and every account they own, in one transaction, after typing a confirmation. No 30-day grace: it is simpler, PDPA-friendly, and the account's database rows go by `ON DELETE CASCADE` (checked: deleting an account removes every tenant row). Files go with the hourly `sweep_uploads` job, since nothing points at them any more. The profile screen's text that promises 30 days changes to say this.
- **One deletion route.** Accounts have exactly one owner (invites only make managers and caretakers), so deleting the owner deletes their accounts. A manager or caretaker deleting themselves only removes their memberships. Apple requires in-app deletion of the person, which this covers.
- **A renewing store plan blocks deletion.** While an App Store or Google Play subscription on an owned account will renew, `DELETE /v1/me` answers 409 `store_plan_renews` ("Cancel your plan in the App Store or Google Play first"), since the server can't cancel it. Web periods don't renew and don't block.
- **Export is one JSON file per account,** built synchronously from the existing loaders: the account, its properties with full setup (bank details included; it is the owner's data), every booking with its guest, payments and slips, the team, and the plan. Owners only. The app saves it and opens the share sheet.
- **Guest erasure keeps the stays.** Erasing replaces the guest's name with "Guest removed", clears phone, email and country, clears `guest_note` on their bookings, and rewrites their name out of activity lines. Stays, prices and payments stay for the owner's records. Slip images are financial records and stay. Owners only, reached from a booking.
- **Account rename** is `PATCH /v1/account` on the account in `X-Account-Id`, owners only, the same rules as creating (`domain.NormalizeAccountName`).
- **Profile photos belong to the person,** not an account: keys `users/<userId>/avatars/<uuidv7>.<ext>`, 5 MB, uploaded through `POST /v1/me/photo` and saved with `PATCH /v1/me` `photoKey` (empty string removes it). `User` gains `photoUrl`. The replaced file is left to the sweep.
- **Refund method** is chosen on the cancel screen from bank, cash and card, defaulting to how the guest last paid. A card refund is only recorded; refunding a card through Payments.lk is phase 9.
- **Universal links** are served by the marketing site: `/.well-known/apple-app-site-association` and `/.well-known/assetlinks.json`, from `STAYKEY_APPLE_TEAM_ID` and `STAYKEY_ANDROID_CERT_SHA256` (comma-separated fingerprints). Both cover `/join/*` only. The app maps `https://staykey.direct/join/<token>` to its `join` screen. Verifying on a device waits for the domain and an EAS build; everything else is checked locally.

## Context

- Deleting an account cascades through every tenant table; `store.DeleteAccount(ctx, t)` (`internal/store/accounts.go:194`) exists and is used by `cmd/seed`. `users` cascades to memberships, sessions, push tokens, activity reads and preferences; other user references are `ON DELETE SET NULL`.
- `users.photo_key` exists (`00002_create_accounts_and_people.sql`), unused. `oapi.User` has no photo field. `GetMe` and `UpdateMe` are in `internal/server/accounts.go:14,34`; `domain.UserPatch` in `internal/domain/account.go`.
- Files: kinds `photo`, `logo`, `slip` (`internal/files/files.go:22`); `NewKey`, `ValidKey` (`keyPattern`, only `accounts/...`), `Belongs`. `R2.WrittenBefore` lists only the `accounts/` prefix (`internal/files/r2.go:92`); `Local.WrittenBefore` walks the whole folder. `FileInUse` (`internal/db/queries/jobs.sql:17`) checks property photos, logos and slips. `SweepUploads` deletes keys older than `UploadGrace` that aren't in use (`internal/jobs/jobs.go:221`).
- `CreateUpload` (`internal/server/uploads.go:14`) is tenant-scoped and needs `PermSettings`.
- `Cancellation.refund` already takes `{amount, method}` with `PaymentMethod` `bank | cash | card`; `CancelBooking` stores it. The app's cancel screen picks the method itself: `apps/mobile/src/app/booking/cancel.tsx:49` (`paidBy`, `method`). The payment screen's method pills are the pattern to copy (`apps/mobile/src/app/booking/payment.tsx:46,150`).
- The profile screen (`apps/mobile/src/app/profile.tsx`) keeps the photo in `useSession.photoUri` / `setPhoto`, and its "Delete account" button only signs out (line ~162).
- Accounts: `GET`/`POST /v1/accounts`; no rename. `accounts.name` has a length check.
- Guests have no id in the API; bookings embed the guest. Guest queries: `FindGuest`, `FindNamedGuest`, `InsertGuest`, `UpdateGuest` in `internal/db/queries/bookings.sql`.
- Subscriptions: `store.AccountBilling` (`internal/store/billing.go`) returns `Sub` with `Source`, `WillRenew`, `GivesAccess`.
- Invites: `STAYKEY_INVITE_URL` defaults to `https://staykey.direct/join`; the marketing page `apps/marketing/src/app/join/[token]/page.tsx` opens `staykey://join?token=`. The app's `src/app/join.tsx` reads `token` from the query. `app.json` has no `associatedDomains` or `intentFilters`.
- Expo Router supports `src/app/+native-intent.tsx` exporting `redirectSystemPath({ path, initial })` to rewrite incoming links (`expo-router/build/types.d.ts`). Next 16 serves `.well-known` paths from route handlers (`node_modules/next/dist/docs/01-app/02-guides/backend-for-frontend.md`).
- `TestTenantIsolation` walks every owner route; path parameters come from `aliceIDs` (`internal/server/isolation_test.go:116`), which already has `bookingId`.
- The app has no `expo-file-system` or `expo-sharing` yet.

## Constraints

- Work in this order, one commit per step: migration (none expected), sqlc queries, domain with tests, OpenAPI, `bun run generate`, store, handlers, files and jobs, marketing, mobile, docs.
- No em dashes anywhere: code, comments, UI copy, commit messages.
- Never edit `api.gen.go`, `internal/store/queries` or `packages/api-client/src/schema.ts` by hand. Never change migrations `00001` to `00011`.
- Owner store methods take a `tenant.Tenant`; person-level methods (`DeleteUser`, photo) take the user id from the principal, never the request.
- Wrap Go errors with `%w`, never discard one with `_`, pass `context.Context` first.
- The server has no request validator; check what the spec states in handlers or domain code.
- Never log phone numbers, names, emails or export contents. Log user and account ids.
- Every new environment variable goes in the app's `.env.example` and, for the API, `passThroughEnv` in `turbo.json`.
- In `apps/mobile`, add packages with `bunx expo install`; read the Expo SDK 57 docs for `expo-file-system` and `expo-sharing` first.
- Next 16: read `apps/marketing/AGENTS.md` and the route handler docs before adding `.well-known` routes.
- Tests run against Postgres 17 through `storetest.New(t)`, behind row-level security.

## Changes

### 1. `apps/api/internal/db/queries/accounts.sql`, `users.sql`, `bookings.sql`, `jobs.sql`
- `accounts.sql`: `RenameAccount :one` (`UPDATE accounts SET name = @name WHERE id = @id RETURNING *`); `OwnedAccountIDs :many` (account ids where the user's membership role is `owner`).
- `users.sql`: `DeleteUser :exec`; `SetUserPhoto :exec` (`photo_key = sqlc.narg('photo_key')`). Add `photo_key` to what `GetUser` and `UpdateUser` return if they don't use `*`.
- `bookings.sql`: `GuestOfBooking :one` (guest id and name for a booking in the account); `EraseGuest :exec` (name `Guest removed`, phone, email, country null); `ClearGuestNotes :exec` (bookings of the guest); `EraseGuestFromActivity :exec` (`replace` the name with `A guest` in `title` and `subtitle` of activity whose `booking_id` belongs to the guest).
- `jobs.sql`: `FileInUse` also checks `users.photo_key`.
- Run `bun run generate`.

### 2. Domain
- `apps/api/internal/domain/account.go`: `UserPatch` gains `PhotoKey *string`; `Validate` accepts empty or a key that `files.ValidAvatarKey` accepts (keep the check in the handler if importing `files` from `domain` would cycle).
- `ErrStorePlanRenews` (new error) in `internal/domain/billing.go`.
- `GuestRemovedName = "Guest removed"`.
- Tests for the patch validation.

### 3. `apps/api/internal/files/files.go`
- `KindAvatar Kind = "avatar"`, `MaxBytes[KindAvatar] = 5 << 20`.
- `NewAvatarKey(userID uuid.UUID, contentType string) (string, error)` returning `users/<userID>/avatars/<uuidv7>.<ext>`.
- `ValidAvatarKey(key string, userID uuid.UUID) bool`.
- `ValidKey` also accepts the avatar shape, so local serving and `Exists` work.
- `r2.go` `WrittenBefore`: list both `accounts/` and `users/`.
- Tests for the new key shapes, including a key for another user being refused.

### 4. `packages/api-spec/openapi.yaml`
- `User` gains `photoUrl` (nullable). `UserPatch` gains `photoKey` ("Empty removes the photo").
- `POST /v1/me/photo` (`bearerAuth` only): body `{contentType, size}`, returns `UploadTicket`.
- `DELETE /v1/me` (`bearerAuth`): body `{confirm}` that must equal `"DELETE"`; 204; 409 `store_plan_renews`; 400 `confirm_required`.
- `PATCH /v1/account` (`bearerAuth` + `accountHeader`): body `{name}`; returns `Account`; 403 for non-owners.
- `GET /v1/account/export` (`bearerAuth` + `accountHeader`): returns `AccountExport` (`exportedAt`, `account`, `properties: Property[]`, `bookings: Booking[]`, `team: Team`, `subscription: Subscription`); 403 for non-owners.
- `POST /v1/bookings/{bookingId}/erase-guest` (`bearerAuth` + `accountHeader`): 204; 403 for non-owners; 404 for a booking outside the tenant's scope.
- Run `bun run generate`.

### 5. Store
- `apps/api/internal/store/accounts.go`:
  - `RenameAccount(ctx, t, name)`.
  - `DeleteUser(ctx, userID, now)`: in one transaction, `OwnedAccountIDs`; for each, `AccountBilling` and refuse with `domain.ErrStorePlanRenews` when `Sub.Source.IsStore() && Sub.WillRenew && Sub.GivesAccess(now)`; delete each owned account, then the user.
  - `SetUserPhoto(ctx, userID, key *string)`; `user()` maps `photo_key`.
- `apps/api/internal/store/bookings.go`: `EraseGuest(ctx, t, bookingID)`: owners only; load the booking through the tenant's scope (reuse the existing get path so scope and 404s match); run the four queries from step 1 in one transaction.
- `apps/api/internal/store/accounts.go` or a new `export.go`: `ExportAccount(ctx, t, now)` assembling `ListProperties`, `ListBookings` over all time (`domain.BookingFilter` with the widest range it accepts, paging until done), `ListTeam`, `Subscription`.
- Tests:
  - deleting an owner removes their account's bookings and guests, and leaves another account they are a manager of intact (only their membership goes);
  - a renewing store plan refuses deletion and changes nothing;
  - erasing a guest clears their details on every one of their stays and removes their name from activity, and the stays and payments stay;
  - a caretaker can't erase or rename;
  - the export of one account contains none of another account's bookings.

### 6. Handlers
- `apps/api/internal/server/accounts.go`: `GetMe` and `UpdateMe` return `photoUrl` (`s.files.URL`); `UpdateMe` takes `photoKey`, checks `files.ValidAvatarKey(key, p.UserID)` and `s.files.Exists`, then `SetUserPhoto`. `CreateMePhoto` presigns an avatar upload. `DeleteMe` checks `confirm == "DELETE"`, calls `DeleteUser`, maps `ErrStorePlanRenews` to 409, and revokes nothing else (sessions cascade). `RenameAccount` (owner check, `NormalizeAccountName`). `ExportAccount` (owner check).
- `apps/api/internal/server/bookings.go`: `EraseBookingGuest` (owner check, maps not found to 404).
- `isolation_test.go`: the new owner routes are covered by the walk; `erase-guest` uses the existing `bookingId` entry. Confirm `DELETE /v1/me` and `POST /v1/me/photo` are not owner routes (bearer only).
- Tests: `DELETE /v1/me` without `confirm` is 400; a caretaker deleting themselves keeps the account; an avatar key for another user is refused on `PATCH /v1/me`; `GET /v1/me` returns `photoUrl` after a photo is set; a manager gets 403 on rename and export; erase-guest from another account is 404.

### 7. `apps/api/internal/jobs`
- No new job. `SweepUploads` already removes a deleted account's files and replaced avatars once `FileInUse` knows avatars and R2 lists `users/`.
- Test in `jobs_test.go`: an avatar in use survives the sweep; one that was replaced is deleted.

### 8. Marketing site (`apps/marketing`)
- `src/app/.well-known/apple-app-site-association/route.ts` (new): `GET` returns `{"applinks":{"details":[{"appIDs":["<team>.direct.staykey.app"],"components":[{"/":"/join/*"}]}]}}` as `application/json`, 404 when `STAYKEY_APPLE_TEAM_ID` is empty. `dynamic = "force-static"` is fine if the env is read at build time; otherwise read it per request.
- `src/app/.well-known/assetlinks.json/route.ts` (new): the Digital Asset Links statement for `direct.staykey.app` with `STAYKEY_ANDROID_CERT_SHA256` split on commas; 404 when empty.
- `.env.example`: both variables.
- `src/app/join/[token]/page.tsx`: unchanged; it stays the fallback when the app isn't installed.

### 9. Mobile app
- `bunx expo install expo-file-system expo-sharing`.
- `app.json`: `ios.associatedDomains: ["applinks:staykey.direct"]`; `android.intentFilters` with `autoVerify: true`, scheme `https`, host `staykey.direct`, `pathPrefix: "/join"`, categories `BROWSABLE` and `DEFAULT`.
- `src/app/+native-intent.tsx` (new): `redirectSystemPath` turns `/join/<token>` (with or without the host) into `/join?token=<token>`, returns other paths unchanged, and never throws.
- `src/api/me.ts`: `useDeleteMe()`, `uploadAvatar(uri)` (presign through `POST /v1/me/photo`, PUT, return the key; reuse the PUT part of `src/api/uploads.ts` rather than copying it).
- `src/api/accounts.ts`: `useRenameAccount()` (invalidates `me` and accounts), `exportAccount()`.
- `src/api/bookings.ts`: `useEraseGuest()` (invalidates the booking, bookings and activity).
- `src/app/profile.tsx`:
  - The photo uploads on save and shows `user.photoUrl`; remove `photoUri` and `setPhoto` from `src/lib/session.ts` (bump the persist `version`, drop the key in `migrate`).
  - Owners see an "Account name" field saved with `useRenameAccount`.
  - Owners see "Download your data", which saves the export JSON to the cache folder and opens the share sheet.
  - "Delete account" asks for `DELETE` typed in, explains that properties, bookings and team are deleted at once and a store plan must be cancelled first, calls `useDeleteMe`, then signs out and resets as today. A 409 shows its message.
- `src/app/booking/cancel.tsx`: method pills (bank, cash, card) under the refund amount, starting at the guest's last payment method, sent as `refund.method`.
- `src/app/booking/[id].tsx`: owners get "Remove guest's details" in the menu, with a confirm sheet saying the stays and payments stay.
- Anywhere the owner's avatar is drawn from `photoUri`, use `user.photoUrl`.

### 10. Docs
- `docs/ROADMAP.md`: tick phase 7 items (the universal links item stays open until checked on a device).
- `docs/HANDOVER.md`: commits, maps (deletion, export, erasure, avatars, links), the calls above, gotchas, gate status.
- `README.md`: the new account controls.

## Do not

- Add a grace period, soft delete or undo for deletion; there is no recovery flow to go with it.
- Delete or rewrite payments, slips, booking prices or `booking_events`; they are the owner's financial records.
- Cancel store subscriptions or refund web periods on deletion.
- Email the export; there is no email sender until phase 8.
- Add guest ids or a guests list to the API; erasure is reached through a booking.
- Add an account transfer, a second owner, or leaving an account as an owner.
- Refund cards through Payments.lk; that is phase 9.
- Add universal links for anything other than `/join/*`, or change `STAYKEY_INVITE_URL`.
- Resize or crop avatars on the server.

## Validate

- `cd apps/api && go vet ./... && CI=true go test -race ./...` -> passes, including `TestTenantIsolation` with the new owner routes.
- `bun run generate && git diff --exit-code` -> no diff.
- `bunx biome check . && bun run typecheck` -> clean.
- `cd apps/mobile && bun test src && bunx expo export --platform ios` -> passes.
- `cd apps/marketing && bun run build` -> builds; with `STAYKEY_APPLE_TEAM_ID=ABCDE12345` and `STAYKEY_ANDROID_CERT_SHA256=AA:BB` set, `curl -i localhost:3000/.well-known/apple-app-site-association` and `/.well-known/assetlinks.json` -> 200, `application/json`, no redirect.
- Against the seeded API:
  - rename the sample account in Profile -> the switcher and `GET /v1/accounts` show the new name;
  - set a photo -> it shows on a second signed-in device (or after reinstalling);
  - export -> the share sheet offers a JSON file with the 43 sample stays;
  - erase a guest from a booking -> every stay of that guest shows "Guest removed" and Activity no longer names them;
  - cancel a stay with a refund by cash -> the payment list shows the refund as cash;
  - delete the sample owner -> sign-in with that number starts onboarding again, the database has no rows for the account, and after `sweep_uploads` runs (or the API restarts) its photos are gone from `apps/api/.media`.
- `xcrun simctl openurl booted "https://staykey.direct/join/<token>"` on a development build lands on the join screen with the token once the domain serves the association file; until then, `bunx uri-scheme open "staykey://join/<token>" --ios` exercises `+native-intent`.

## Risks

- **Deleting the wrong thing.** -> Deletion runs only for the principal's own user and the accounts they own; tests cover a manager membership surviving. Typed confirmation in the app.
- **A store plan keeps charging after deletion.** -> Refuse with 409 while it renews; the message says where to cancel.
- **Row-level security hides rows during deletion or export.** -> `DELETE /v1/me` runs with no tenant (bearer only), so the policies let it through; export runs with the tenant and reads only that account, which is intended.
- **Activity rewrite misses lines that name the guest without a `booking_id`.** -> Only stay lines name guests, and they all carry `booking_id`; check `domain/activity.go` builders when adding new ones.
- **Universal links can't be verified before hosting.** -> Ship the files and config, test `+native-intent` with the custom scheme, and leave the roadmap item open.
- **The sweep deletes an avatar being uploaded.** -> It only removes files older than `UploadGrace` (a day) that nothing points at.

## Went-sideways flags

- `TestTenantIsolation` fails on `PATCH /v1/account` or `GET /v1/account/export` with 200 for Bob (the handlers read the account from the request instead of the tenant).
- After deleting an owner who is also a manager elsewhere, that other account is gone or has lost more than one membership.
- A guest's name still shows in Activity or on another of their stays after erasure, or a payment disappeared with it.
- `GET /v1/me` returns a `photoUrl` under `accounts/...`, or `PATCH /v1/me` accepts another user's avatar key.
