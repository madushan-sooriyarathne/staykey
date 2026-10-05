# StayKey repository notes

StayKey (staykey.direct) is a direct booking SaaS for small Sri Lankan villa and guesthouse owners. Go API, Expo owner app, Next.js booking page and widget.

## Start of a session

- Read `docs/HANDOVER.md` (current state, maps of the API and app, gotchas, open questions), `docs/ROADMAP.md` (what's left, as checkboxes) and the relevant phase in `docs/backend-plan.md`.
- Read `apps/mobile/AGENTS.md` before touching the app.
- First time on a machine: `bun run setup` installs dependencies, starts Postgres, migrates and seeds.

## Working style

- No em dashes anywhere: code comments, docs, UI copy, commit messages. Restructure the sentence instead.
- Smooth sentence flow, concise and direct, understated professional tone.
- Commit after each meaningful step (migration, store, handlers, app area, docs). Small commits with a plain summary line.
- Inside a phase, work in this order: migration, sqlc queries, domain rules with tests, OpenAPI routes, handlers, generated client, app hooks, screens.
- When an open question in `docs/HANDOVER.md` would block the work, ask. Otherwise take the listed default and record the call in the handover's "Calls made without asking".
- At the end of a phase: tick `docs/ROADMAP.md`, update `docs/HANDOVER.md` (commits, state, maps, gotchas, next step, gate status), the status line in `docs/backend-plan.md` and the README, then commit.

## Tooling

- Bun is the package manager and script runner. Use `bun` and `bunx`, never npm, pnpm or yarn. In `apps/mobile`, add packages with `bunx expo install <pkg>` so versions match the Expo SDK.
- Turborepo runs tasks across workspaces: `bun run dev`, `build`, `typecheck`, `test`, `generate`. It runs in strict env mode, so a new environment variable an API task reads goes in that task's `passThroughEnv` in `turbo.json`.
- The API contract is `packages/api-spec/openapi.yaml`. Change it first, run `bun run generate`, then implement handlers in `apps/api/internal/server`. Never edit `api.gen.go`, `internal/store/queries` or `packages/api-client/src/schema.ts` by hand.
- SQL lives in `apps/api/internal/db/migrations` (goose) and `apps/api/internal/db/queries` (sqlc). Owner store methods take a `tenant.Tenant`; every business table carries `account_id`.
- Every owner route with path parameters needs an entry in `TestTenantIsolation`'s `aliceIDs` fixture.
- Go code is formatted with `gofmt` and checked with `go vet`. TypeScript, JSON and CSS use Biome from the repo root.
- App-specific agent notes live in `apps/booking/AGENTS.md`, `apps/marketing/AGENTS.md` and `apps/mobile/AGENTS.md`. Next.js 16 and Expo SDK 57 differ from older versions, so read the docs those files point to before changing framework code.

## Rules of the domain

- Money is always an integer in minor units with an ISO currency code.
- Stay dates are local `YYYY-MM-DD`; events are `timestamptz`.
- Linked units (a whole house and its rooms) block each other.
- Brand tokens come from `@staykey/tokens`. Ember marks direct bookings and actions that need the owner; Magenta Spark marks wins and live status. Primary buttons stay obsidian.

## Verification

Run `/verify`, or by hand:

```sh
bunx biome check . && test -z "$(gofmt -l apps/api)"
bun run typecheck
bun run generate && git diff --exit-code   # generated code is up to date
cd apps/api && go test ./...               # needs Postgres
cd apps/mobile && bun test src && bunx expo export --platform ios
```
