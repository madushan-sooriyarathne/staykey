---
paths:
  - "**/*.ts"
  - "**/*.tsx"
---

# TypeScript Rules

These rules govern all TypeScript in this project. They are not style preferences — Biome handles formatting. They are about type safety and correctness.

## No `any`

Never introduce `any`, explicit or implicit. If a type is genuinely unknown, use `unknown` and narrow it with a runtime check or a Zod parse before use. Do not silence the compiler with `// @ts-ignore` or `// @ts-expect-error` unless you add a comment explaining the specific upstream bug being worked around. `as` casts are a code smell: prefer narrowing, type guards, or fixing the source type. A cast that lies (e.g. `as User` on unvalidated data) is forbidden.

## Infer types from the source of truth

Database types come from the Drizzle schema, never hand-written. Use `typeof table.$inferSelect` / `$inferInsert` (or `InferSelectModel`/`InferInsertModel`) and derive everything else from those. When a function returns DB data, let its return type be inferred or expressed in terms of the schema types so a column change propagates automatically.

The same principle applies elsewhere: derive types from Zod schemas with `z.infer`, from `as const` objects, and from existing function signatures. Do not maintain a parallel hand-typed interface that can drift from its source.

## Validate at external boundaries with Zod

Any data crossing into the program from outside — HTTP request bodies, query/search params, route params, environment variables, third-party API responses, webhook payloads, file contents — must be parsed with Zod at the boundary. Past that boundary, pass the inferred type around; do not re-validate internally and do not accept loosely-typed input deep in the call stack.

Internal, fully-typed data does not need Zod. Don't add validation theater between two functions you control.

## General

Prefer `type` aliases for unions and object shapes; use `interface` only when declaration merging is actually needed. Make illegal states unrepresentable with discriminated unions, and handle them exhaustively (a `never` check in the default branch). Avoid non-null assertions (`!`) — narrow instead. Keep functions returning a single, predictable shape rather than unions of unrelated results where a caller can't tell which it got.

`import type { ... }` for type-only imports. Never type a callback as `Function` — write the explicit signature. Use constructor parameter-property shorthand (`constructor(private readonly db: Database) {}`) instead of separate field declarations. Brand primitive IDs (`type UserId = Brand<string, "UserId">`) so a `UserId` can't be passed where an `OrgId` is expected. Prefer `satisfies` over `as` when validating an object against a shape without widening its literal types.

DB helper functions that may run inside or outside a transaction should type their connection parameter as `Database | Transaction<...>`, not just `Database`, so the same helper works in both contexts.

## tsconfig

`strict: true` is the floor. Also enable `noImplicitReturns`, `noUnusedLocals`, `noUnusedParameters`, `isolatedModules`.
