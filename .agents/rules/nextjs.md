---
paths:
  - "apps/booking/src/**"
  - "apps/marketing/src/**"
  - "apps/*/next.config.*"
---

# Next.js Rules (App Router)

## Server components by default

Components are Server Components unless they demonstrably need the client. Do not add `"use client"` reflexively. A component needs `"use client"` only when it uses state/effects, browser-only APIs, event handlers, or a client-only library. When it does, push the directive as far down the tree as possible — wrap the small interactive leaf in a client component and keep its parents on the server. Never put `"use client"` at the top of a page or layout to make a single button work.

When you do add `"use client"`, the implicit justification should be obvious from the code; if it isn't, add a short comment saying why.

## Data fetching

Fetch data in Server Components and server actions, close to where it's used. Don't build client-side fetch waterfalls for data that could be fetched on the server and passed down. Parallelize independent fetches with `Promise.all` rather than awaiting in sequence. Be deliberate about caching and revalidation — choose static, `revalidate`, or dynamic based on how fresh the data must be, and don't accidentally force a whole route dynamic by reading `headers()`/`cookies()` where you don't need to.

Keep server-only modules server-only. Never import server code (DB client, secrets, server env) into a client component. Use `server-only` / `client-only` packages to enforce the boundary where it matters.

## Routing & structure

Use the App Router conventions: `page.tsx`, `layout.tsx`, `loading.tsx`, `error.tsx`, `route.ts`, `not-found.tsx`. Co-locate route-specific components, but lift genuinely shared UI into a shared location/package. Use route groups `(group)` for organization without affecting the URL. Provide `loading.tsx`/Suspense boundaries for slow segments and `error.tsx` for graceful failures.

## Server actions

Validate inputs with Zod at the top of every server action — actions are public endpoints. Check the BetterAuth session and authorization before doing work. Revalidate (`revalidatePath`/`revalidateTag`) after mutations rather than over-fetching.

## Data access layer

Centralize DB queries, CMS calls, and third-party mutations in a dedicated DAL (`lib/dal/`, `features/*/queries.ts`). Add `import "server-only"` at the top of every DAL file so an accidental import into a Client Component fails the build instead of leaking server code to the client.

Identical `fetch` calls (same URL + options) inside one render pass are automatically deduped — don't thread data through props or a top-level provider just to avoid a second call. For mutable/dynamic data, tag fetches (`next: { tags: ["products"] }`) and purge with `revalidateTag` from a route handler or server action rather than guessing a time-based revalidate window.

## Misc

Use `next/image` for images and `next/font` for fonts — always set `sizes` when the image isn't a fixed layout, and `priority` on above-the-fold LCP images. Use `next/link` for internal navigation. Reach for `next/dynamic` to keep heavy client-only widgets out of the initial bundle. Set route `metadata` for pages that need it. Wrap components reading `cookies()` or `searchParams` in `<Suspense>` so they don't block the rest of the page. Validate dynamic route params early and call `notFound()` immediately on a miss rather than letting bad data flow downstream.
