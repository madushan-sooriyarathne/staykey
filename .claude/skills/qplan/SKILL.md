---
name: qplan
description: Investigate the repository and turn a task into an implementation-ready plan plus a portable execution handoff, without modifying code. Trigger with /qplan followed by the task.
argument-hint: "[task description]"
disable-model-invocation: true
allowed-tools:
  - Read
  - Write
  - Grep
  - Glob
  - Bash(git status)
  - Bash(git log *)
  - Bash(git diff *)
  - Bash(pbcopy *)
  - Bash(wl-copy *)
  - Bash(xclip *)
  - Bash(clip.exe *)
---

# QPlan

Plan the task in `$ARGUMENTS` so it fits the codebase instead of fighting it. Understand first,
then propose a grounded, actionable plan with the smallest change that works. Modify no project
files — the only write is the plan file below.

## Preconditions

Ensure `/qnew` has run. If not, first resolve the project instructions (`CLAUDE.md` / `AGENTS.md`)
and environment constraints before planning.

## Investigation Procedure

1. **Read the rules.** Load the project instructions (`CLAUDE.md` / `AGENTS.md`) plus any rules/guides they reference. The plan must obey them.
2. **Map the ground.** If `graphify-out/GRAPH_REPORT.md` exists, read it first for the god nodes and community structure before scanning raw files. Then grep for existing functionality, patterns, and naming conventions the task touches — trace the real flow end to end.
3. **Deconstruct the requirement.** Identify functional requirements, acceptance criteria, boundaries, and potential side effects.
4. **Inspect context.** Read entry points, relevant components, types, schemas, services, and existing tests. Trace the end-to-end data flow.
5. **Check patterns.** Match existing architectural paradigms and utilities. Prefer extending existing code over new files; no new abstractions when existing ones suffice.
6. **Determine validation.** Identify targeted verification (unit tests, integration tests, lint, type-check) in the project's existing test style.
7. **Determine required rules & skills.** Identify repository rules (e.g. path-scoped rules in `.claude/rules/` / `.agents/rules/`) and skills (e.g. `ponytail`) that `/qcode` must load before executing the plan.
8. **Clarify ambiguities.** If an ambiguity materially changes implementation decisions, pause and ask targeted questions in a single grouped prompt. Document minor assumptions directly in the plan.

## Plan Structure

Build the plan with this schema. Enforce the `ponytail` ladder (YAGNI, smallest working diff, stdlib/platform first).

```markdown
# Implementation Plan

## Objective

<precise statement of the requested outcome>

## Required Rules & Skills

- **Rules**: `<rules to load before execution, e.g. .claude/rules/testing.md>`
- **Skills**: `<skills to load before execution, e.g. ponytail>`

## Current State vs. Proposed Changes

### 1. <Target component/feature>

- **Files**: `<path>`
- **Changes**: <specific modification details>
- **Rationale**: <why this change is needed>

## Edge Cases & Risks

- <identified edge case or operational risk, and its mitigation>

## Testing & Validation

- `<command>`: <behavior it validates>

## Acceptance Criteria

- [ ] <verifiable criterion>

## File Manifest

- **Modify**: `<paths>`
- **Create**: `<paths>`
- **Delete**: `<paths>`

## Implementation Sequence

1. <ordered, step-by-step execution list>
```

## Portable Handoff Contract

Always append a self-contained execution block, so `/qcode` or another agent can run the plan cold:

```plaintext
EXECUTION HANDOFF

Repository assumptions:
<key branch, environment, paths, tools>

Task:
<clear objective>

Required Rules & Skills:
- Rules: <rules to load before execution>
- Skills: <skills to load before execution>

Instructions:
<project-specific conventions and constraints>

Detailed plan:
<the implementation plan>

Validation:
<targeted validation commands>

Execution rules:
<scope boundaries and specific restrictions>
```

## Presentation & Delivery

1. **Save the plan.** Write the detailed plan plus the EXECUTION HANDOFF to
   `${TMPDIR:-/tmp}/qplan-<task-slug>.md` — never inside the repository.
2. **Show a TL;DR only**, shaped per `caveman` and `i-have-adhd`: lead with the next action and a
   time estimate, then the approach and scope (files touched) in a few lines.
3. **Offer two choices**, as a numbered list:
   1. **View detailed plan** — print the plan file.
   2. **Copy handoff prompt** — copy the plan file to the clipboard so another model can execute it
      (`pbcopy < file`, `wl-copy < file`, `xclip -selection clipboard < file`, or `clip.exe < file`,
      whichever exists). If none is available, render the handoff in a code block instead.
4. **Otherwise**, end by telling the user to run `/qcode` to execute the plan.
