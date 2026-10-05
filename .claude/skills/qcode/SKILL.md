---
name: qcode
description: Execute the /qplan plan or a pasted EXECUTION HANDOFF without expanding scope, then validate and report results. Trigger with /qcode.
argument-hint: "[EXECUTION HANDOFF or notes]"
disable-model-invocation: true
allowed-tools:
  - Read
  - Edit
  - Write
  - Grep
  - Glob
  - Bash
---

# QCode

Execute an existing implementation plan precisely, validate the output, and report status. `/qcode`
executes; it does not plan or redesign. Before any work, load every rule and skill the plan names.

## Input & Plan Selection

Source priority:

1. A plan or `EXECUTION HANDOFF` block passed in `$ARGUMENTS`.
2. The most recent `/qplan` or `/qcheck` output in the session.
3. The newest `${TMPDIR:-/tmp}/qplan-*.md` or `qcheck-*.md` file. Show its Objective (or Task) and continue only if it matches
   the session's task; otherwise ask.

If no valid plan exists, halt and ask the user to run `/qplan` first — do not improvise one.

## Pre-Execution Checks

Before editing any file:

1. **Load required rules & skills.** Read every rule (e.g. `.claude/rules/*.md` / `.agents/rules/*.md`) and skill listed under the plan's `Required Rules & Skills` or instructions, plus the project instructions (`CLAUDE.md` / `AGENTS.md`). Obey them throughout.
2. **Verify the workspace baseline.** Branch, paths, and dependencies match the plan's assumptions.
3. **Read referenced files** before editing; confirm target APIs and interfaces match the plan.
4. **Reconcile discrepancies.** If the codebase materially differs from the plan, pause and reconcile before changing anything.
5. **Detect the project's commands.** Read `package.json` scripts (or the equivalent manifest — `Cargo.toml`, `go.mod`, `pyproject.toml`, `Makefile`) and config files. Prefer commands the instruction file names.

## Execution Constraints

- **Strict scope.** Implement the planned changes in sequence. Do not refactor unrelated code, reformat untouched files, or upgrade unrelated dependencies.
- **Preserve conventions.** Match the naming, error handling, and architecture of the surrounding code.
- **Minimal footprint.** Apply the smallest coherent diff that meets the acceptance criteria (the `ponytail` ladder).
- **Dependencies & generated files.** Add no dependency unless the plan says so. Never hand-edit generated files; edit their sources.

## Execution & Validation Loop

For each planned step:

1. **Inspect** the target code.
2. **Implement** the planned change.
3. **Run targeted validation** (the relevant test, type-check, or lint). Do not defer all validation to the end.
4. **Reconcile failures.** Separate pre-existing breakage from regressions the change caused; fix regressions before moving on.

After the last step, run the project's full quality gate, skipping any step it lacks: type-check,
lint, test, build. If the instructions name a single gate (e.g. one `test`/`check` script), run that.

## Deviations

Allowed only when an interface mismatch, missing dependency, or repository rule blocks the planned
approach. Then:

- Pick the smallest compatible adjustment that keeps the original objective.
- Document the rationale in the report.
- For architectural deviations, pause and confirm with the user.

## Final Report

Terse, per `caveman` and `i-have-adhd`. Never claim done on an unverified change.

On success:

```plaintext
Execution Complete

Implemented:
  <summary of completed work>

Files changed:
  <modified/added paths>

Validation:
  <command> → passed

Deviations:
  <none, or brief explanation>

Notes:
  <operational follow-ups, or none>
```

On failure or blocker:

```plaintext
Execution Blocked

Implemented:
  <work completed so far>

Validation:
  <command> → failed

Failure:
  <root cause or failing output>

Remaining:
  <work still required to resolve>
```

End with one next action: `/qcheck` (review) or `/qgit` (commit) when green.
