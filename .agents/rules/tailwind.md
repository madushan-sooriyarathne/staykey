---
paths:
  - "apps/booking/src/**"
  - "apps/marketing/src/**"
  - "packages/widget/**"
---

# Tailwind CSS v4 Rules

## CSS-first config

v4 has no `tailwind.config.js`. Declare custom tokens, colors, fonts, and keyframes directly in the global CSS file inside `@theme`. Don't generate or look for a JS/TS config file.

## Conditional classes

Wrap conditional/merged class logic in a `cn()` helper (`clsx` + `tailwind-merge`), never raw template-string ternaries in `className`. For components with more than a couple of style variants, use `tailwind-variants` (`tv`) instead of nested ternaries, and export the variant prop types from the schema.

## Responsive & container queries

Mobile-first: base classes are the smallest breakpoint, scale up with `sm:`/`md:`/`lg:` in ascending order. For a self-contained widget that needs to adapt to its container rather than the viewport (a card that can sit in a sidebar or a wide column), mark the parent `@container` and use `@md:`/`@lg:` variants instead of viewport breakpoints.

## Class order

Layout/position → box model/spacing → typography → visual (background/border) → interactive state (`hover:`/`focus-visible:`) → animation/transition. Prefer `focus-visible:` over `focus:` so outlines only show for keyboard users.

## Avoid div-soup

When a class list gets too long to scan, extract a child component — don't compress it into a global class via `@apply`. Reserve `@apply` for true global resets (typography defaults, base form elements), not component-level styling.
