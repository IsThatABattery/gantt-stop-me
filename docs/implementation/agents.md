# Astronaut console

The roster of AI astronauts on a mission, with one selected astronaut's trace, its open hail and the controls to steer it.

- **Branch:** `feature/frontend-agents`
- **Lane:** A (board and astronauts)
- **Phase:** 3

## Design source

- `docs/ganttstopme-plan/design/mockups/ProbeConsole.dc.html` (a legacy filename; it's the astronaut console)
- AGENT_BRIEF §6 (lifecycle, hails, autonomy levels and badges)
- AGENT_BRIEF §9 (violet means astronaut; orbit pulse)

## Routes

- `/projects/:projectId/agents`, rendered by `src/pages/agents/index.tsx` inside the app shell with the nav rail.

## Files you own

- `src/pages/agents/`
- `src/data/views/agents.ts`
- `src/data/mock/agents.ts` if needed
- Tests next to these files

## Data available

`listAgents` / `useAgents` return each agent working on this project, with:
- its current task
- its open hail (if any)
- its trace steps for that task

Sample: Astronaut-3 has an open hail on PAY-45 ("3 or 5?"). The others are docking, planning and comms down. `Agent.paused` and `Agent.autonomy` are there to change.

## Scope

- [ ] **Header:** eyebrow "ASTRONAUTS · N ACTIVE · N NEEDS YOU", title, "Back to board".
- [ ] **Roster tiles:** name, role, state chip (plain word plus color), current payload key and title, autonomy, time on task. A tile with an open hail gets the amber border, as in the mockup. Selecting a tile selects that astronaut, keyboard-reachable.
- [ ] **Selected astronaut panel:**
  - **Hail box,** when there's an open hail: the question, "Hail · Waiting on you", and a reply field with Send. Sending writes `answer`, `answeredBy` and `answeredAt`, sets the agent back to `working`, sets the task status back to `working`, and adds an `answered` event.
  - **Trace:** mono lines with time, kind and text.
- [ ] **Controls:**
  - **Pause / Resume:** toggles `Agent.paused`.
  - **Redirect:** a short instruction, logged as an event.
  - **Autonomy:** a three-way choice using `RadioCard` with tone `astronaut` (Tethered, Guided, Free flight), with the orbit-ring badge. Respect `settings.freeFlightGrantedBy`: if it's `lead`, only a lead can pick Free flight.
  - **Recall to Hangar:** moves the task to Backlog, clears `currentTaskId`, sets the agent to `idle`, logs `recalled`. It needs an in-page confirm step.
- [ ] Every write refreshes `keys.agents`, `keys.board` and `keys.activity`.
- [ ] **Empty state** for a mission with no active astronauts (e.g. `/projects/deep-field/agents`).

## Out of scope

- Real agent runtime, cost or confidence data.
- Creating or deleting astronauts.
- Per-astronaut settings beyond autonomy.

## Walkthrough

1. Open `/projects/orbital-nav/agents`. Four tiles show, and Astronaut-3 is amber.
2. Select Astronaut-3 and reply "3". The hail clears, and PAY-45 shows Working on the board.
3. Set Astronaut-2 to Free flight. As the sample lead (TV) this is allowed.
4. Recall Astronaut-4. After the confirm step, PAY-43 is in Hangar.
5. Open `/projects/deep-field/agents`. The empty state shows.

## Open questions

- With no agent runtime, what should Redirect change beyond logging the instruction?
- Should Pause also stop the astronaut's task timer?

## Shared changes

Shared files are frozen on this branch (see `src/front/README.md`):

- `src/data/types.ts`, `keys.ts`, `core.ts`
- `src/data/mock/fixtures.ts`, `store.ts`
- `src/labels/labels.ts`
- `src/app/`, `src/components/`, `src/styles/`

If you need a new type field, label, route or shared component, open a small PR with just that change straight into `feature/frontend`. Rebase this branch once it merges, and tell the other person so they rebase too.

## Done when

- `npm run lint`, `npm run build` and `npm test` pass in `src/front`.
- Tests cover this view's main flows. Use `renderApp(path)` from `src/test/render.tsx`, as in `src/app/router.test.tsx`.
- Every item in Scope works against the mock data, and the walkthrough above runs without console errors.
- The PR into `feature/frontend` touches only the files listed under Files you own, plus any shared-change PRs that already merged.
