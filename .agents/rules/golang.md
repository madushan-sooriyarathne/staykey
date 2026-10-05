---
paths:
  - "**/*.go"
---

# Go Rules

## Layout

`/cmd/<binary>/main.go` for entrypoints, `/internal` for private application code (unimportable outside the module), `/pkg` only for code explicitly meant to be imported by other projects. Read config in `main.go` (or a dedicated loader) and pass it down explicitly via structs — don't read env vars deep in business logic.

## Naming

Short names for short-lived scope (`i`, `ctx`, `r`); descriptive names for package-level vars. Initialisms stay one case: `userID`, `apiURL` — never `userId`. No `this`/`self` receivers — use a short abbreviation of the type (`func (srv *Server) Start()`). Package names: single lowercase word, no underscores, no `util`/`common`/`shared` grab-bags — group by domain instead.

## Errors

Never discard an error with `_`. Check `if err != nil` immediately and keep the happy path at minimal indentation — handle the error inline rather than nesting the success path. Wrap with `fmt.Errorf("...: %w", err)` to preserve the chain; inspect with `errors.Is`/`errors.As`, never `==` or a manual type assertion. Reserve `panic` for unrecoverable boot-time failures (missing required config, failed DB connection at startup) — never for predictable business-logic errors.

## Interfaces

Accept interfaces, return concrete types. Keep interfaces small (one or two methods, `-er` suffix) and extract them only once a second concrete implementation actually exists — not preemptively for mocking.

## Performance

Pointer receivers when mutating, when the struct holds a `sync.Mutex`, or when the struct is large; value receivers for small immutable structs. Preallocate with `make([]T, 0, n)`/`make(map[K]V, n)` when the size is known. Pass slices and maps by value, not by pointer — they're already header structs.

## Concurrency

Never start a goroutine without knowing how it exits. `sync.Mutex`/`sync/atomic` for simple shared state; channels for pipelines and pub/sub. Pass `context.Context` as the first argument of any function doing I/O, and always `defer cancel()` immediately after `context.WithTimeout`/`WithCancel`.

## Testing

Table-driven tests for multiple cases. Run with `-race` locally and in CI. Structured logging (`slog`, `zap`, `zerolog`) over the standard `log` package. Drain in-flight connections and background workers on `SIGINT`/`SIGTERM` before `main` exits.
