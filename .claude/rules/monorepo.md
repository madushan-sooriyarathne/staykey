---
paths:
  - "apps/**"
  - "packages/**"
  - "turbo.json"
---

# Monorepo Rules (Turborepo + pnpm workspaces)

Applies when the project is a Turborepo with pnpm workspaces. For a single-app repo, only the "no barrel files" and import-hygiene points apply.

## Respect package boundaries

Each package owns a clear responsibility (`apps/*` are deployables; `packages/*` are shared libraries). Apps depend on packages; packages do not depend on apps. Avoid circular dependencies between packages — if two packages need the same thing, extract it into a third. A package's public surface is what it explicitly exports; don't reach into another package's internals.

## No cross-package relative imports

Never import across package boundaries with `../` relative paths (e.g. `import { x } from "../../packages/ui/src/..."`). Import by the workspace package name instead (`import { x } from "@acme/ui"`). Relative imports are fine _within_ a single package. Add the dependency to that package's `package.json` (using the `workspace:*` protocol) so the boundary is explicit and the build graph is correct.

## Dependencies & tooling

Install dependencies into the specific package that uses them, not the root — the root holds only repo-wide dev tooling. Use pnpm (`pnpm add <pkg> --filter <package>`), never npm or yarn, so the lockfile and workspace links stay consistent. Keep shared dev config (Biome, TypeScript base config) at the root or in a shared config package and extend it, rather than copying config into every package.

## Turborepo

Define tasks in `turbo.json` with correct `dependsOn` (`^build` for upstream builds) and `outputs` so caching works. Don't bypass the task graph by running package scripts ad hoc in a way that skips dependencies. Keep tasks cacheable — declare inputs/outputs accurately and avoid side effects that defeat caching.

## No barrel files

Avoid `index.ts` barrel files that re-export a whole directory. They hurt tree-shaking, create import cycles, and pull unrelated code into bundles. Import from the specific module path. A package's single top-level entry point defined in its `package.json` `exports` is fine; sprawling internal barrels are not.
