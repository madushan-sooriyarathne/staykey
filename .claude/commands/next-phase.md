---
description: Start or resume the next StayKey phase from the roadmap
argument-hint: "[phase number, optional]"
---

Read `docs/HANDOVER.md`, `docs/ROADMAP.md`, the matching phase in `docs/backend-plan.md`, `CLAUDE.md` and `apps/mobile/AGENTS.md`.

Work on phase $ARGUMENTS if a number was given; otherwise take the first phase in `docs/ROADMAP.md` with unticked items. Check `git log` to see whether that phase is already part way done, and resume from the first unticked item.

Before writing code, check the open questions in `docs/HANDOVER.md`. Ask me only those that block this phase; otherwise use the listed defaults and flag them.

Set up a task list from the phase's roadmap items, ending with the gate and a docs update. Follow the order and working style in `CLAUDE.md`, commit after each meaningful step, and run `/verify` before each commit that touches code. When the gate passes, update the roadmap, handover, plan and README as `CLAUDE.md` describes, then give me a short summary with the calls you made without asking.
