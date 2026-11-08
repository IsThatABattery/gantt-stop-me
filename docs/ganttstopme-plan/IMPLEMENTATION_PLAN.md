# Implementation plan (suggested)

A suggested build order. Nothing here is decided: confirm the first section with the owner before writing code.

## Decisions needed first

1. **Stack.** Frontend framework, backend, and database are all open. Pick for realtime updates and a themeable UI.
2. **Realtime transport.** Boards and the Mission log update live. The design assumes it; websockets or server-sent events both fit.
3. **Astronaut runtime.** What actually runs the AI agents, and what can it report (step traces, cost, confidence)? The astronaut console and approval flow depend on a trace format.
4. **Auth.** The login screen shows GitHub, SSO, and email and password.
5. **Astronaut capacity.** Is it measured in concurrent payloads, a compute or token budget, or both? (Drives launch window planning.)
6. **Astronaut authority.** May astronauts create or split payloads, or only work ones assigned to them? Who can grant Free flight: only the lead, or any operator?
7. **Scope check.** The product name suggests a Gantt or timeline view. Is it in v1?
8. **Multi-tenancy and hosting.**

## Phases

Stop for review after each phase.

**Phase 1: foundations**
- Project setup, auth, workspace and mission model, operators and roles.
- Theme system: color tokens from `design/tokens.json`, fonts, and a **label dictionary** so Space and Plain labels switch without touching the data model.
- Shared components: nav rail, chips, avatars (operator and astronaut), progress bar, form controls.
- Done when: sign in, pick a mission, and see an empty board in both themes.

**Phase 2: board and payloads**
- Phases, objectives, payloads, modules, tags; create and edit screens for objectives and payloads (one shared form).
- Board with five columns, drag between columns, objective filter, card states, blocked flag, payload drawer.
- Launch windows (basic): create one, commit payloads, countdown in the telemetry bar.
- Done when: a team can run a sprint on the board with no astronauts.

**Phase 3: astronauts**
- Astronaut roster and console, the payload lifecycle, Hails, autonomy levels (0 to 2) with enforcement, approval bar (Approve and land, Request changes, Take over), recall.
- Mission log with live updates and filters.
- Done when: an astronaut can pick up a payload, ask a question, submit work, and be approved or sent back, with every step visible.

**Phase 4: objective planning flow**
- Briefing chat with a live draft doc, doc review with deny and feedback, payload proposal queue with approve, edit, and deny, and the Done summary.
- Approved payloads go to the Hangar tagged with the objective; denied items return to the astronaut with the feedback attached.
- Done when: an operator can go from an idea to approved payloads on the board without filling in a form.

**Phase 5: the rest**
- Hangar (backlog) screen, launch window planning with capacity, Debrief, full Mission log page, remaining Settings sections, notifications, integrations.
- Light theme, empty and error states, mobile layouts, accessibility pass against `AGENT_BRIEF.md` section 10.

## Risks to watch

- **Label drift.** Keep plain names in code and the API; space names only in the label dictionary.
- **Approval gates are the product.** Test that an astronaut cannot land work above its autonomy level.
- **Violet means astronaut.** Do not reuse violet for anything else.
- **Placeholder data.** Mockup names, IDs, and numbers are samples. The doc success criteria contain `[TARGET]` placeholders that need real values from the owner.
