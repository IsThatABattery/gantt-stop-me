# GanttStopMe: UI design plan

An agentic scrum board where human operators and AI astronauts share one backlog, styled as a space mission control room. This package is the **UI design only**. There is no code or backend yet, and the tech stack is **not decided**.

## What is in here

| File | Read it for |
| --- | --- |
| `AGENT_BRIEF.md` | The product, terminology, data model, behavior rules, and visual identity. Start here. |
| `IMPLEMENTATION_PLAN.md` | Decisions to make first, a phased build order, and open questions. |
| `design/SCREENS.md` | Every screen, what it does, and how screens link together. |
| `design/tokens.json` | Colors, fonts, radii, and sizes pulled from the mockups. |
| `design/mockups/` | The visual mockups (17 screens) plus `canvas.json`, their layout index. |

## About the mockups

Each `.dc.html` file is one screen written as HTML with inline CSS, plus a few template tags (`{{hole}}`, `<sc-for>`, `<sc-if>`, `<dc-import>`). They only render inside the Claude Design canvas, which supplies a `support.js` runtime that is **not** included here.

Use them as a structural and visual reference: copy the layout, spacing, colors, and copy text. Do not port the template tags. All names, IDs, counts, and dates in them are placeholder data.

Some filenames are legacy and no longer match their screens:
- `ProbeConsole.dc.html` is the **astronaut console**.
- `Home.dc.html` is the **mission select** screen.
- `ObjectiveEditor.dc.html` and `NewObjective.dc.html` serve **both objectives and payloads**. `PayloadEditor.dc.html` and `NewPayload.dc.html` just load them in payload mode.

## Kickoff prompt

> Read `README.md`, then `AGENT_BRIEF.md`, `design/SCREENS.md`, and `IMPLEMENTATION_PLAN.md`. Do not write code yet. List the decisions in the "Decisions needed first" section that you need from me, propose a stack with your reasoning, and wait for my answers. Then build Phase 1 only and stop for review.
