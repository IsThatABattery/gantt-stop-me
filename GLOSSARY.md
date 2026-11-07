# Gantt

A Kanban board that lives inside a software project's repository, with agents that scope features, write the first tests, run acceptance checks and merge, so that humans only write code and review it.

## Language

### Things on the board

**Project**:
A repository that has adopted the tool. It carries a Board and a description of how to build, run and test itself.
_Avoid_: target repo, workspace, codebase

**Board**:
The Kanban view of a Project's Features and Tasks, kept in the Project's repository as the only record of their state.
_Avoid_: scrum, scrumboard, backlog (for the whole board)

**Feature**:
A piece of work a human describes at a high level. Its state is derived from its Tasks and never stored.
_Avoid_: story, epic, initiative

**Task**:
A unit of work carved out of a Feature during Scoping, small enough for one branch and one pull request.
_Avoid_: ticket, issue, subtask, story

**Card**:
A Task as it appears on the Board. A Task and its Card are the same thing seen in two places.
_Avoid_: using card for a Feature

**Slug**:
The short name a human gives a Feature when adding it, which identifies the Feature for its whole life. A Task's id is its Feature's Slug plus a number.
_Avoid_: key, id (for a Feature), name

### Statuses

**Backlog**:
A Task nobody has started. Its branch exists and holds only failing tests.

**In progress**:
A Task someone has pushed code for but has not opened a pull request on.

**In review**:
A Task with an open pull request, awaiting a Review.

**Closed**:
A Task whose pull request has merged.

**Feature state**:
Planned when every Task is in backlog, Done when every Task is closed, In progress otherwise. Derived, never stored.
_Avoid_: feature status (as a stored field)

### Work the agents do

**Trigger**:
A repository event (a push, a pull request opened, a review submitted) that starts exactly one agent or job. Nothing coordinates agents beyond Triggers.
_Avoid_: hook, orchestrator, scheduler

**Agent**:
A step that uses a language model, started by a Trigger: the scoping agent, the test-writing agent and the browser agent. Each runs with a model chosen from the Project's Providers. Jobs that run tests, move Cards or merge are deterministic and are not agents.
_Avoid_: bot, worker, orchestrator

**Provider**:
An endpoint a Project has configured for language models, together with the models it offers. Agents are assigned models from Providers.
_Avoid_: vendor, backend, LLM (for the endpoint)

**Scoping**:
Turning a Feature into Tasks, each with acceptance criteria and initial failing tests. Done by the scoping agent when a Feature without Tasks lands on the main branch.
_Avoid_: planning, grooming, refinement, breakdown

**Acceptance criterion**:
A statement, written at Scoping, of something a person must be able to do once a Task is done. The browser agent checks it and the reviewer judges it.
_Avoid_: requirement, spec, user story

**Recording**:
The video of a browser agent's run against a Task's pull request, attached to the pull request and the Card.
_Avoid_: screencast, demo, capture

**Verdict**:
The browser agent's pass, fail or unsure for one acceptance criterion. Advisory only: it never merges or blocks on its own.
_Avoid_: result, grade, score

**Review**:
A human's approve or reject of a Task in review, made as a pull request review. Approve merges; reject is a Rejection.
_Avoid_: QA, sign-off, acceptance (for the act)

**Rejection**:
A Review that sends a Task back to in progress, always with a reason. A Task keeps every Rejection it has received.
_Avoid_: decline, bounce, fail
