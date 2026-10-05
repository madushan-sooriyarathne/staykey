---
description: Run every StayKey check and fix what fails
---

Run these from the repo root and report the result of each. Postgres must be running (`bun run db:up`).

1. `bunx biome check .`
2. `test -z "$(gofmt -l apps/api)"`
3. `bun run typecheck`
4. `bun run generate && git diff --exit-code` (generated code is up to date)
5. `cd apps/api && go test ./...`
6. `cd apps/mobile && bun test src`
7. `cd apps/mobile && bunx expo export --platform ios --output-dir /tmp/staykey-export`

If something fails, fix the cause rather than the check, then run the failing step again. Never edit generated files by hand.
