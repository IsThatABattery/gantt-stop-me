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

GanttStopMe is a Kanban board over a GitHub repository. Every task on the board is a GitHub issue. An automated layer tests a task's linked pull request in isolated containers and reports back, so a person approves and merges with evidence in hand. Agents also plan features: an interview produces a plan doc, then proposed tasks, and a person approves each before it reaches the board, unless they turned review off.

Success means a person can see at a glance what every agent is doing, why, and what it is waiting on, and nothing an agent produces merges without a human approval point.

## Positioning

From the README, `docs/architecture.md` and `docs/glossary.md`:

- Humans and AI agents work the same backlog. Agents are visible teammates, not a hidden bot.
- Human approval is the default gate. Approval points are explicit.
- Tests run in isolated containers on a runner the team controls. A result keeps "the code failed" (Test Failure) apart from "the run broke" (Runner Error).
- GitHub stays the source of truth. Issues, labels, assignees and open or closed state sync both ways. The board owns only each task's column, order and tests, and keeps the tests in step with the issue's Tests section.

## Operating Context

- **Getting started:** sign up with email and password, link GitHub (OAuth or a personal access token), then add a repository by URL. The board imports its open issues.
- **Columns:** Backlog, In Progress, Testing, User Check, Closed, in that fixed order. Tasks move between any two, and a move to another column writes its `board:*` label to the issue. While automation is on, a task can't leave Testing while its test run is going, or enter User Check without a passing run.
- **Testing:** a task's tests live in the Tests section of its issue body. Each is a command or a plain-English check verified by an LLM. A test run checks the linked PR. A runner error flags the task as Needs Attention. Manual tests are left for the approver.
- **Approval:** in User Check, a person checks the results and any manual tests, then approves, which merges.
- **Planning:** Plan a feature, on the board, starts an interview with an agent that writes a plan doc. Once a person approves the doc, a second agent proposes tasks, and a person approves, sends back or drops each one.
- **Settings:** account and password, appearance (color theme), GitHub link, LLM providers (OpenAI-compatible endpoints) and planning agents. Each project's Automation page turns test runs on or off, picks their two agents, and proposes changes to the workflows file as a pull request.
- **Device:** desktop is the daily setting. Mobile layouts are built fluid but unverified.

## Capabilities and Constraints

- **Stack:** React 19, TypeScript, Vite, Tailwind CSS v4, TanStack Query, React Router and dnd-kit in `src/front`. A Go API in `src/back/api`, Postgres, a runner that runs tests in Docker containers, and nginx serving the build. The frontend never calls GitHub directly.
- **Docker only:** the host needs only Docker. Every test suite runs in Docker: `api-test`, `runner-test` and `web-test`.
- **Plain names in code:** code, data and URLs use plain names (Project, Task, Agent). The words users see belong in a swappable label dictionary, `src/front/src/labels/labels.ts`. Names, actions, columns and run states come from it today, and many sentences are still written in the views.
- **Color themes:** each is one swappable stylesheet in `src/front/src/styles/themes/`, listed in `src/front/src/theme/themes.ts`, and chosen per browser in Settings.
- **Terms:** `docs/glossary.md` is binding. For example: Task, not card or ticket. Column, not status or lane. Test Run, Workflow, Runner, Test Failure versus Runner Error, Manual Test, Needs Attention.
- **Open decision:** whether teams self-host or use a hosted service is undecided (2026-11-09). Today it runs with Docker Compose and dev defaults.
- **Not built yet:** `docs/gaps.md` lists what the app doesn't do yet, such as an agent roster, an activity log, features and sprints, and workspace settings.

## Brand Commitments

- **Name:** GanttStopMe.
- **Vocabulary:** plain names are the default voice (Project, Task, Backlog, Testing), confirmed 2026-11-09, and the UI shows only plain words today. The space-mission dictionary (Mission, Payload, Hangar, Flight check, Landed) is to become an optional label set, which isn't built yet. Wherever a themed word appears, its plain word appears beside it.
- **Existing asset:** the logo mark, an ink square holding three stage blocks with the last in approve yellow, where the work stops for a person, in `src/front/src/components/Logo.tsx` and `src/front/public/favicon.svg`. It is not confirmed as binding.

## Evidence on Hand

- There are no users, customers, testimonials, metrics or benchmarks. Do not invent any.
- **Sample data:** `src/front/src/test/fakeApi.ts`, the tests' in-memory stand-in for the API, holds one GitHub-backed project. There is no other sample data.
- **Design:** `src/front/DESIGN.md` is the current design system, and `src/front/.impeccable/surfaces/` holds the brief for the board and app shell. The first, space-themed design brief and its mockups are no longer in the repository; git history keeps them.

## Product Principles

1. **Humans stay in the loop by default.** Approval points are explicit and easy to find.
2. **Agents are visible teammates.** Always show what each agent is doing, why, and what it is waiting on.
3. **Legibility over spectacle.** Status reads at a glance, and no theme or label set hides it.
4. **Evidence before approval.** A person approves with test results in hand, and never mistakes a broken run for broken code.
5. **GitHub is the source of truth.** The board never disagrees with the repository.

## Accessibility & Inclusion

A standing requirement:

- **Contrast:** text and chips meet WCAG AA in every theme. `src/front/src/theme/contrast.test.ts` enforces it.
- **Never color alone:** state pairs every color with an icon and a word.
- **Targets and controls:** touch targets are at least 44px. Use real buttons, links and labeled inputs.
- **Keyboard:** the board works fully from the keyboard. Arrow keys move between tasks, `[` and `]` move a task a column, and Enter opens it.
- **Screen readers:** they announce plain labels. Themed words are decoration.
- **Reduced motion:** it turns off every animation and transition, the live mark and the move trail included.
