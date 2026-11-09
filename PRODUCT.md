# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Small software teams sharing one GitHub repository, working alongside AI agents (confirmed 2026-11-09).

- **The lead** sets goals, assigns work and approves what lands.
- **Developers** work tasks and review what agents produce.
- **AI agents** test and review developers' code, and plan features, reporting what they do back to the people.

The team uses the board through the working day to see what is moving, what is being tested, and what is waiting on a person.

## Product Purpose

GanttStopMe is a Kanban board over a GitHub repository. Every task on the board is a GitHub issue. An automated layer tests a task's linked pull request in isolated containers and reports back, so a person approves and merges with evidence in hand. Agents also plan features: an interview produces a plan doc, which becomes tasks only after a person approves it.

Success means a person can see at a glance what every agent is doing, why, and what it is waiting on, and nothing an agent produces merges without a human approval point.

## Positioning

From the README, ARCHITECTURE.md, GLOSSARY.md and the design brief:

- Humans and AI agents work the same backlog. Agents are visible teammates, not a hidden bot.
- Human approval is the default gate. Approval points are explicit.
- Tests run in isolated containers on a runner the team controls. A result keeps "the code failed" (Test Failure) apart from "the run broke" (Runner Error).
- GitHub stays the source of truth. Issues, labels and open or closed state sync both ways. The board owns only each task's column and order.

## Operating Context

- **Getting started:** sign up with email and password, link GitHub (OAuth or a personal access token), then add a repository by URL. The board imports its open issues.
- **Columns:** Backlog, In Progress, Testing, User Check, Closed, in that fixed order. Cards move freely between any two, and each move writes a `board:*` label to the issue.
- **Testing:** a task's tests live in the Tests section of its issue body. Each is a command or a plain-English check verified by an LLM. A test run checks the linked PR. A runner error flags the task as Needs Attention. Manual tests are left for the approver.
- **Approval:** in User Check, a person checks the results and any manual tests, then approves, which merges.
- **Settings:** account and password, GitHub link, LLM providers (OpenAI-compatible endpoints), planning agents, appearance (color theme).
- **Hidden screens:** some screens are built on mock data but hidden until their API exists: the agent roster and console, the activity log, objectives and epics, sprints, and planner task review. `docs/implementation/api-gaps.md` tracks them.
- **Device:** desktop is the daily setting. Mobile layouts are built fluid but unverified.

## Capabilities and Constraints

- **Stack:** React 19, TypeScript, Vite, Tailwind CSS v4, TanStack Query, React Router and dnd-kit in `src/front`. A Go API in `src/back/api`, Postgres, and nginx serving the build. The frontend never calls GitHub directly.
- **Docker only:** the host needs only Docker. Every test suite runs in Docker: `api-test`, `runner-test` and `web-test`.
- **Plain names in code:** code, data and URLs use plain names (Project, Task, Agent). The words users see come from a swappable label dictionary, `src/front/src/labels/labels.ts`.
- **Color themes:** each is one swappable stylesheet in `src/front/src/styles/themes/`, listed in `src/front/src/theme/themes.ts`, and chosen per browser in Settings.
- **Terms:** GLOSSARY.md is binding. For example: Task, not card or ticket. Column, not status or lane. Test Run, Workflow, Runner, Test Failure versus Runner Error, Manual Test, Needs Attention.
- **Open decision:** whether teams self-host or use a hosted service is undecided (2026-11-09). Today it runs with Docker Compose and dev defaults.
- **Missing from the API:** user profile name and role, workspace settings, the agent roster and sprints (see `docs/implementation/api-gaps.md`).

## Brand Commitments

- **Name:** GanttStopMe.
- **Vocabulary:** plain names are the default voice (Project, Task, Backlog, Testing), confirmed 2026-11-09. The space-mission dictionary (Mission, Payload, Hangar, Flight check, Landed) becomes an optional label set. Wherever a themed word appears, its plain word appears beside it. The UI still defaults to the space labels today, so switching the default is pending work.
- **Existing asset:** the logo mark, an orbit ring around a planet, in `src/front/src/components/Logo.tsx` and `src/front/public/favicon.svg`. It is not confirmed as binding.

## Evidence on Hand

- There are no users, customers, testimonials, metrics or benchmarks. Do not invent any.
- **Sample data:** `src/front/src/test/fakeApi.ts` holds the tests' fake GitHub-backed project. `src/front/src/data/mock/fixtures.ts` holds sample missions and agents for the hidden screens.
- **Design brief and mockups:** `docs/ganttstopme-plan/`, which holds AGENT_BRIEF.md, `design/mockups/*.dc.html` and `design/tokens.json`. They are evidence of the first design, not a binding look.
- **Screenshot:** `screenshots/03-board.png`.

## Product Principles

1. **Humans stay in the loop by default.** Approval points are explicit and easy to find.
2. **Agents are visible teammates.** Always show what each agent is doing, why, and what it is waiting on.
3. **Legibility over spectacle.** Status reads at a glance, and no theme or label set hides it.
4. **Evidence before approval.** A person approves with test results in hand, and never mistakes a broken run for broken code.
5. **GitHub is the source of truth.** The board never disagrees with the repository.

## Accessibility & Inclusion

A standing requirement, from the design brief §10:

- **Contrast:** text and chips meet WCAG AA in every theme. `src/front/src/theme/contrast.test.ts` enforces it.
- **Never color alone:** state pairs every color with an icon and a word.
- **Targets and controls:** touch targets are at least 44px. Use real buttons, links and labeled inputs.
- **Keyboard:** the board works fully from the keyboard. Arrow keys move between tasks, `[` and `]` move a task a column, and Enter opens it.
- **Screen readers:** they announce plain labels. Themed words are decoration.
- **Reduced motion:** it turns off pulses and trails.
