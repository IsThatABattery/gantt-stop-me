# Agent brief

## 1. Product

GanttStopMe is a scrum board where **humans and AI agents work the same backlog**. The interface is a calm, dark mission-control console. A person can see what every AI agent is doing, why, and what it is waiting on. Nothing an agent produces merges, ships, or closes without a human approval point, unless autonomy has been explicitly raised.

Roles:
- **Flight director**: team lead / product owner. Sets sprint goals, assigns work, approves output, grants higher autonomy.
- **Operators**: human team members. Work payloads, review astronaut output, pair with astronauts.
- **Astronauts**: AI agents. Pick up payloads, report progress, ask for help, submit results for review.

## 2. Design principles

1. **Legibility over spectacle.** The space theme frames the product but never hides status. Every themed label has a plain word behind or beside it.
2. **Astronauts are visible teammates.** Always show what each is doing, why, and what it is waiting on.
3. **Humans stay in the loop by default.** Approval points are explicit.
4. **One glance tells sprint health.** Countdown, burndown, blockers, and items awaiting approval stay on screen.
5. **The theme is swappable.** The space skin is first. Layout and components must work with a Plain skin too, and a Plain theme should ship alongside it.

## 3. Terminology

The **data model, API, and code use the plain names**. Space names are display labels from a swappable label dictionary.

| Space label (display) | Plain name (code) | Meaning |
| --- | --- | --- |
| Space agency | Workspace / org | Top level: settings, operators, astronaut roster |
| Mission | Project | Has its own board, backlog, and operators |
| Mission phase | Epic | Groups objectives; shown as a colored band on cards |
| Objective | Feature | Groups related payloads (n payloads to one objective); sits under a mission phase |
| Payload | Task / story | The card on the board |
| Module | Subtask | Checklist item inside a payload |
| Hangar | Backlog | Unscheduled payloads waiting for a launch window |
| Launch window | Sprint | Fixed timebox with a T-minus countdown |
| Operator | User (human member) | Avatar with initials |
| Astronaut | AI agent | Helmet avatar with a violet ring |
| Mission control | Board | The main board screen |
| Mission log | Activity feed | Time-ordered events from operators and astronauts |
| Debrief | Retrospective | End-of-sprint review |
| Hail | Agent help request | A short question an astronaut pins to a card |
| Comms down | Blocked | A flag, not a column |
| Tag | Tag | Plain, neutral, **not themed** |

Hierarchy: **Workspace → Mission → Mission phase → Objective → Payload → Module.** Objectives do **not** replace mission phases; a phase can hold several objectives.

## 4. Suggested data model

Derived from the UI. Confirm before building.

| Entity | Key fields |
| --- | --- |
| Workspace | name, members, astronaut roster, settings |
| Mission | workspace, name, description, default autonomy, state (in launch window / between), operators |
| Phase | mission, name, color |
| Objective | mission, phase, name, description, owner (operator), target launch window, tags, success criteria (list with met flag), default autonomy for new payloads (inherit / 0 / 1 / 2), status (planned, active, landed, archived) |
| Payload | id (e.g. `PAY-41`), mission, objective, title, description, assignee (operator or astronaut), estimate (points), priority (Low, Medium, High, Critical), launch window, tags, acceptance criteria, autonomy override, column/status, blocked flag + reason, linked PRs |
| Module | payload, title, assignee, done |
| LaunchWindow | mission, name, start, end, capacity (crew hours and astronaut capacity) |
| Operator | name, initials, role (flight director / operator) |
| Astronaut | name, role/model, state, current payload, autonomy default, time on task |
| Event (Mission log) | timestamp, actor (operator or astronaut), verb, target payload, detail |
| Hail | payload, astronaut, question, answer, answered by/at |
| ObjectiveDoc | objective, version, sections, status (draft, needs review, approved, denied) |
| PayloadProposal | doc, drafted payload fields, source doc section, status (needs review, approved, approved-edited, sent back, dropped) |
| Feedback | target type + id, outcome, reasons (list), text (required), optional doc section |

## 5. The board

**Columns** (default, editable): Hangar (to do) → Pre-flight (ready) → In orbit (in progress) → Docking (in review, human or astronaut output awaiting approval) → Landed (done). **Comms down** (blocked) is a flag on a card, not a column.

**Top telemetry bar:** mission and launch window name, sprint goal, T-minus countdown (monospace, amber in the last 24 h), burndown sparkline, and three clickable filter counters: *Awaiting approval* (amber), *Comms down* (red), *Astronauts active* (violet).

**Board toolbar:** objective filter, links to manage objectives, New objective, New payload. Optional swimlanes by mission phase, objective, or assignee type.

**Left rail:** All missions (back), Mission control, Hangar, Launch windows, Astronauts, Mission log, Debrief, Settings (pinned bottom).

**Right drawer:** live Mission log (collapsible). Each entry: time, actor (astronaut names violet, operator names cyan), what happened, payload ID. Filters planned: operators only, astronauts only, needs action.

**Payload card, top to bottom:** thin phase color band; ID (monospace) + phase; title (two lines max); objective name with a target glyph; footer with owner avatar, estimate or progress or PR number, and a status chip.

| Card state | Treatment |
| --- | --- |
| Default | Surface fill, line border |
| Hover / focus | Cyan border, slight lift |
| Astronaut working | Violet border or accent, slow orbit pulse on the avatar |
| Hail raised | Amber dot on avatar plus amber "Hail" chip |
| Awaiting approval | Amber border, docking icon |
| Comms down | Red border plus red chip with the blocker reason |
| Landed | Dimmed, green check, no animation |
| Recalled / failed | Red outline, collapsed to title only |

Payload detail opens as a **right-side drawer** over the board: header (ID, status, assignee, phase, estimate), tabs (Brief, Modules, Flight log, Cargo), agent trace, linked PRs, and a pinned **approval bar** when output is in Docking.

## 6. Astronaut behavior

**Payload lifecycle for astronaut-owned work:** Assigned → Planning → In orbit → Docking → Landed.
- **Hail:** from In orbit, an astronaut that is stuck raises a Hail and waits for an operator; the answer returns it to In orbit and is kept in the card's Flight log.
- **Request changes:** from Docking, sends it back to In orbit.
- **Take over:** an operator finishes it; then it lands.
- **Recall:** the lead can recall from any state, returning the payload to the Hangar.
- Every state that needs a human is **amber** and counted in the top-bar counter.

**Autonomy levels** (set per astronaut, overridable per objective default and per payload):

| Level | Label | Astronaut may | Human must |
| --- | --- | --- | --- |
| 0 | Tethered | Suggest plans and draft only | Approve the plan before any action, and approve the output |
| 1 | Guided | Act on its plan, open PRs | Approve output before it lands |
| 2 | Free flight | Act and land low-risk payloads (tagged) | Review after the fact in the Debrief |

Show the level as a small badge (1, 2, or 3 orbit rings) on the astronaut avatar.

**Astronaut console:** roster tiles (name, role, state, current payload, autonomy, time on task); selected astronaut panel with a step-by-step monospace trace (plan, tool calls, results), an inline reply box for a Hail, and controls: pause, redirect, change autonomy, recall.

## 7. Create and edit (objectives and payloads)

One pair of screens serves both: a full edit page and a short create dialog. The item type changes only the fields.

| | Objective | Payload |
| --- | --- | --- |
| Name field | Name | Title |
| First select | Mission phase | Objective (parent) |
| Second select | Owner | Assignee (operator or astronaut) |
| Extra fields | none | Estimate (points), priority |
| Checklist | Success criteria | Acceptance criteria |
| Autonomy | Default for new payloads (inherits mission) | Override for this payload (inherits objective) |
| Right-hand list | Payloads, landed progress bar | Modules, done progress bar |

Shared: description, target launch window, tags (plain, neutral color). The edit page has Archive, Cancel, Save changes. The create dialog asks only for essentials; the objective dialog also offers **Plan with an astronaut**.

## 8. Objective planning flow (astronaut-assisted)

An operator can plan an objective by being interviewed instead of filling a form. Two human gates keep it reviewable: **nothing reaches the board until the operator approves it.**

1. **Briefing.** A briefing astronaut asks one question at a time. A draft objective doc fills in beside the chat; each topic shows Covered, Asking now, or Not yet. The operator can skip a question or request the draft early; uncovered topics become open questions in the doc.
2. **Review doc.** The doc is shown whole with a decision panel: *Approve and plan payloads*, *Deny with feedback*, *Edit the doc myself*. Astronaut comments appear beside it.
3. **Review payloads.** A planning astronaut proposes payloads. A queue lists them with status chips (Needs review, Approved, Approved · edited, Sent back); the selected one shows description, acceptance criteria, estimate, priority, suggested assignee, dependencies, objective, phase, and the doc section it came from. Actions: *Approve and add to Hangar*, *Edit before approving*, *Deny with feedback*. A sticky bar shows progress.
4. **Done.** Summary of payloads added to the Hangar, sent back, and still waiting; links to the board and the objective.

**Denying with feedback** (same form for a doc and a payload):
- Outcome: *send back for revision* or *discard*. For a payload: *revise* or *drop*.
- Reasons: any of too broad, missing detail, wrong scope, wrong approach, other (payloads: too big, unclear criteria, duplicate, wrong objective, other).
- Free text is **required**. A doc denial can point at a section.
- The astronaut revises and returns the item with a note on what changed. A revised payload re-enters the queue marked "Revised".

## 9. Visual identity

See `design/tokens.json`.

- **Look:** dark mission-control console; navy surfaces, thin instrument lines, color used only for state. Dark mode is primary; a light "daylight ops" mode should mirror it.
- **Color meaning:** cyan = focus and primary actions; **violet = an astronaut is involved**; green = done or approved; amber = waiting on a human; red = blocked, failed, rejected.
- **Type:** Inter for UI, Space Grotesk for headings, JetBrains Mono for IDs, timestamps, countdowns, and traces. Small letter-spaced uppercase eyebrows (e.g. `MISSION · ORBITAL-NAV`) on headers.
- **Icons:** line icons at one stroke weight. A helmet for astronauts, an orbit ring for in progress, a target glyph for objectives.
- **Motion:** subtle and functional (orbit pulse on active astronauts, a short trail when a card changes column). No ambient animation; the starfield texture is static. Honor reduced motion.
- **Copy:** light mission language, always paired with a plain status word (e.g. "Docking · Awaiting review").

## 10. Accessibility requirements

- State is never color-only: every color pairs with an icon and a word.
- Text and chips meet WCAG AA contrast on dark surfaces; check violet and amber.
- Touch targets at least 44 px. Real buttons, links, and labeled inputs.
- Full keyboard control of the board: arrow keys between cards, a key to move a card a column, Enter to open the drawer, A / R / T for Approve, Request changes, Take over.
- Screen readers announce plain labels; space names are decoration.
- Reduced-motion setting disables pulses and trails.

## 11. Not designed yet

- Hangar (backlog list) screen, including astronaut triage chips and the next-launch-window panel
- Launch windows planning screen, Debrief, and the full Mission log page
- Most of Settings: only astronaut autonomy defaults, approval rules, and theme are drawn
- Empty, loading, and error states; mobile layouts (pages are built fluid but unverified)
- The Plain theme and the light theme
- Objective-level views beyond the edit page (e.g. an objective swimlane)
