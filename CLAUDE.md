# StayKey repository notes

- Bun is the package manager and script runner. Use `bun` and `bunx`, never npm, pnpm or yarn. In `apps/mobile`, add packages with `bunx expo install <pkg>` so versions match the Expo SDK.
- Turborepo runs tasks across workspaces: `bun run dev`, `build`, `typecheck`, `test`, `generate`.
- The API contract is `packages/api-spec/openapi.yaml`. Change it first, run `bun run generate`, then implement handlers in `apps/api/internal/server`. Never edit `api.gen.go` or `packages/api-client/src/schema.ts` by hand.
- Money is always an integer in minor units with an ISO currency code.
- Brand tokens come from `@staykey/tokens`. Ember marks direct bookings and actions that need the owner; Magenta Spark marks wins and live status. Primary buttons stay obsidian.
- App-specific agent notes live in `apps/booking/AGENTS.md`, `apps/marketing/AGENTS.md` and `apps/mobile/AGENTS.md`. Next.js 16 and Expo SDK 57 differ from older versions, so read the docs those files point to before changing framework code.
- Go code is formatted with `gofmt` and checked with `go vet`. TypeScript, JSON and CSS use Biome from the repo root.
