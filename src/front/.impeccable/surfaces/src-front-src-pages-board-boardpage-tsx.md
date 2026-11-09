---
version: 1
slug: "src-front-src-pages-board-boardpage-tsx"
primary_target: "src/front/src/pages/board/BoardPage.tsx"
related_targets: ["src/front/src/app/AppShell.tsx","src/front/src/components"]
---

# Board and app shell

Scope: the board (`/projects/:projectId`), the main screen, and from it every routed screen of the web app: shell and nav, task drawer and forms, automation, settings, sign-in, home and add project. Mode: Operate.

Audience and job: a small team with AI agents, working one GitHub repository through the day. They see what is moving, what is being tested, and what waits on a person, then act: filter, move, open, approve.

Constraints: plain words by default (PRODUCT.md). The number of columns will grow, so nothing may depend on there being five. Four palettes (Light as the default, Dark, Space light, Space dark) repaint one structure through the theme system. The board's drag and drop, keyboard moves and live regions stay as they are.

Chosen: prototype B, "Process flow", without its numbered spine. Memorable moment: the ink stage tabs over stepped gray tiles, where a tab lights signal or approve when its stage holds work for a person.

Unresolved: how agents get their own mark once the agent screens are routed.

## Direction contract

THESIS: The board is a process-flow sheet. Each stage is a stepped neutral tile under a solid ink label tab, and tasks are white keylined cards. It refuses the category default of borderless pastel columns, floating rounded cards and a rainbow of status pills.

OWN-WORLD: One cool neutral family in stepped values: ground, rail, and stage tiles stepping from first to last across however many columns exist. Solid ink is used for stage tabs, the primary action and the active nav block. Cards are white paper with a 1px line edge and an inset keyline. Radii are 2 to 4px, and controls have 1.5px ink borders. Big Shoulders Display caps set page titles and stage tabs, Public Sans the UI, Red Hat Mono issue numbers and branches. Colour appears only where a person is needed: vermilion signal for needs attention or failed, yellow approve fill for awaiting approval, each with an icon and a word.

STORY: Open a project and read the pipeline left to right. The counters say what needs you, and one click filters to it. Tasks drag or key between stages, and opening one shows its tests and the approval.

FIRST VIEWPORT: On the left, a blocky rail: product mark, projects as blocks with the current one filled ink, and Settings and the user at the foot. In the main area, the project name is set huge in display caps, with owner/name and the task count under it. To the right sit three counter buttons (a count block plus a label) and the ink New task button. Below, the stage tiles run side by side at a fixed minimum width. Each is topped by its ink tab (name, count, and a signal or approve mark when needed), then its purpose line, then the cards. Extra columns scroll sideways, and there is no numbered spine.

FORM: the user's reference infographic, a process flow, carried into the board: grounded candidate 1, chosen by the user as prototype B over the assigned drafting sheet. seed:3ea8a4ae5d109021

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
