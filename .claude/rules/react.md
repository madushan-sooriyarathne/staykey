---
paths:
  - "**/*.tsx"
  - "**/*.jsx"
---

# React Rules

## Composition over configuration

Don't pile boolean props onto a component to switch its layout (`showHeader`, `showFooter`). Use sub-component composition (`<Card><Card.Header/>...`) so the call site reads like the structure it renders.

## Lists

Keys must be stable, unique identifiers (`item.id`). Never use the array index as a key if the list can be filtered, sorted, or reordered — it desyncs React's reconciliation from the actual item identity.

## Effects

Treat `useEffect` as a last resort, only for syncing with something outside React (a subscription, an imperative DOM API, a global event listener). Every subscription/listener/timeout created in an effect must return a cleanup function. Don't use `useEffect` for data fetching or for deriving a value that could be computed directly during render.

## State

Derive, don't duplicate: if a value can be computed from existing props/state, compute it during render instead of mirroring it into a second state variable. Colocate state near where it's consumed; don't lift to a parent or global store until more than one distant component actually needs it.

For async UI (form submissions, optimistic updates), prefer `useActionState`, `useTransition`, and `useOptimistic` over hand-rolled loading booleans.

## Memoization

Don't reach for `React.memo`/`useMemo`/`useCallback` by default — only when profiling shows a real cost (a heavy computation, or a dependency of an unstable third-party API). Premature memoization adds a dependency array to get wrong and rarely pays for itself.

## Safety

Wrap independent layout sections (sidebar, main panel, nav) in separate error boundaries so one broken fragment doesn't take down the whole page. Never use `dangerouslySetInnerHTML` on unsanitized input — run it through a sanitizer (e.g. DOMPurify) first.

## Testing

Query through accessible roles (`screen.getByRole`), not test-id soup or implementation details. Test what the user sees and does, not internal state.
