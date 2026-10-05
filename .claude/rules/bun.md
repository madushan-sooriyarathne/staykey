---
paths:
  - "**/*.ts"
  - "bunfig.toml"
  - "**/*.test.ts"
---

# Bun Rules

Applies when the project's runtime is Bun. Skip if it's plain Node.

## Tooling

Use the `bun` CLI exclusively — `bun add`, `bun add -d`, `bun run`. Never mix in `npm`/`yarn`/`pnpm` commands within a Bun project; that produces a second lockfile and drifting dependency trees. Pin the exact Bun version in `Dockerfile`/CI (`oven/bun:1.x.x-alpine`), never `latest`.

## Native APIs over Node polyfills

- HTTP server: `Bun.serve({ fetch(req) {...} })`, not `node:http`. WebSockets via the `websocket` option on `Bun.serve`, not a separate `ws` package.
- Files: `Bun.file(path).text()/.json()` and `Bun.write(path, data)`, not `node:fs` streams — fall back to `node:fs` only for low-level file-descriptor operations Bun doesn't expose.
- Env: `process.env` is populated from `.env` automatically. Never install `dotenv`. Validate `process.env` through Zod at startup.
- SQLite: `bun:sqlite`, not `better-sqlite3` or similar.

## Testing

Use `bun:test` (`import { test, expect, describe, mock } from "bun:test"`), not Jest or Vitest, for anything running under the Bun runtime. Run via `bun test`, coverage via `bun test --coverage`.

## Production

Catch `SIGTERM`/`SIGINT`, call `.stop()` on the running server, and give in-flight requests a grace period before `process.exit(0)` — don't let a deploy sever live connections mid-request.
