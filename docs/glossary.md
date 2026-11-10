# Gantt Stop Me

A Kanban board over a GitHub repository, where each task is an issue. A planning layer turns a feature someone describes into tasks, and an automated layer tests a task's code before a person approves and merges it.

Code, data, URLs and docs use these words. The UI takes its terms from [`src/front/src/labels/labels.ts`](../src/front/src/labels/labels.ts), which uses the same ones.

## Board

**Project**:
One GitHub repository added to the app, with its own board.
_Avoid_: Repo, board (when meaning the repository), mission

**Member**:
A user whose linked GitHub account can see a project's repository. Their permission there, read, write or admin, sets what they may do: a read-only member can look, and a write member can also change tasks, plan features, turn automation on and approve.
_Avoid_: Collaborator, operator

**Task**:
One GitHub issue in a project, shown as a card on the board.
_Avoid_: Ticket, payload, issue (when meaning the board item), card (except for how the board draws a task)

**Column**:
One of the board's fixed stages: Backlog, In Progress, Testing, User Check, Closed.
_Avoid_: Status, lane, stage (except for the tile that draws a column), the space names such as Hangar and Landed

**Board Label**:
The `board:{key}` label on a task's issue that names its column, such as `board:in-progress`. A move on the board writes it, and one added on GitHub moves the task.
_Avoid_: Status label, column label

**Sync**:
The API's refresh of a project's tasks from its issues and labels on GitHub, when the board or a task loads, at most once every 30 seconds per project.
_Avoid_: Import (the first read, when a project is added), poll (automation's read of pull requests)

**Assignee**:
A GitHub user the task's issue is assigned to. The rail lists the open tasks assigned to the signed-in user.
_Avoid_: Owner

**Task Branch**:
The branch the API makes on GitHub, off the default branch, when a task first moves to In Progress, named from its issue number and title, such as `feature/12-retry-cap`. A linked PR may come from it or from any other branch of the repository.
_Avoid_: PR branch

## Planning

**Plan**:
A feature a write member describes, carried from an interview through a plan doc to proposed tasks. Its status is briefing, needs review, approved, done or discarded.
_Avoid_: Objective, epic, feature (when meaning the plan)

**Plan Doc**:
The document the interviewer writes for a plan, in fixed sections from Goal to Open questions. Each section is covered, asking or not yet.
_Avoid_: Spec, brief

**Planning Agents**:
The interviewer and the task agent. Each user picks each one's provider, model and own instructions in Settings, and a plan uses its creator's.
_Avoid_: Planner (when meaning one agent), bot

**Interviewer**:
The planning agent that asks about a plan's feature one question at a time, at most 12, and writes the plan doc.
_Avoid_: Planner

**Task Agent**:
The planning agent that turns an approved plan doc into proposals, and rewrites a proposal sent back.
_Avoid_: Planner, generator

**Proposal**:
A task the task agent proposes for a plan, with a description, acceptance criteria, tests and the earlier proposals it depends on. Approving it creates its task in Backlog. The UI calls proposals proposed tasks.
_Avoid_: Draft task, suggestion

**Review**:
A write member approving or denying a plan doc, or each proposal, before the plan goes on. A user who turns review off in Settings has the docs and proposals of their plans approved as the agents write them.
_Avoid_: Sign-off

## Automation

**Automation**:
A project's automated testing and merging of linked PRs. A write member turns it on, and it needs the API to have a runner.
_Avoid_: CI, pipeline

**Automation Owner**:
The member who turned automation on, whose GitHub credential and decision and chat models every test run in that project uses. Merges and run comments show on GitHub as theirs. Another write member who turns automation on takes it over and becomes the owner.
_Avoid_: Bot, runner user

**Paused**:
Automation whose owner unlinked GitHub, lost write access, or whose credential GitHub no longer accepts. Queued work waits, and nothing merges, until a write member turns automation on again.
_Avoid_: Disabled, off (automation that is off has no owner)

**Automation Agents**:
The decision agent and the check agent, as the UI names a project's decision model and chat model.
_Avoid_: Bots

**Decision Model**:
The automation owner's model that answers typed questions with probabilities instead of text. It picks the workflow for each test with no pin, when the repository has more than one. The UI calls it the decision agent.
_Avoid_: Router, classifier

**Chat Model**:
The automation owner's model that carries out LLM checks, in a tool-calling loop. The UI calls it the check agent.
_Avoid_: Checker, test agent

**Linked PR**:
An open pull request in the project's repository that closes a task's issue, such as with "Closes #12". A test run checks its code, and an approval merges it.
_Avoid_: Branch (when meaning the code under test)

**Test**:
One of a task's tests, kept in the Tests section of its issue body: a script or an LLM check.
_Avoid_: Test case, check (one kind of test)

**Tests Section**:
The `## Tests` checklist in a task's issue body. The task stores its tests, and the section mirrors them: an edit to it on GitHub is taken when it parses.
_Avoid_: Checklist, acceptance criteria

**Script**:
A test whose command, in a code span or a code block, runs with `sh -c` in a container from the workflow's test image. It passes when the command exits 0.
_Avoid_: Command test, unit test

**LLM Check**:
A test in plain English that the chat model carries out with tools: commands, HTTP requests to the app or a sidecar, and a browser when the workflow runs an app. It reports a pass, a fail, or that it could not verify, which makes it a manual test.
_Avoid_: AI test, prompt

**Pin**:
A `[workflow]` name before a test's text, which sends the test to that workflow instead of the decision model's pick.
_Avoid_: Tag, label

**Test Run**:
One execution of a task's tests against one commit of its linked PR.
_Avoid_: Job, build, pipeline

**Workflow**:
A named test environment, such as frontend or backend, defined in the repository's workflows file. Each test runs in one: its pin, else the only workflow, else the decision model's pick.
_Avoid_: Pipeline, job, suite

**Workflows File**:
`.gantt/workflows.yml` in the repository, which defines its workflows: each one's description, build, env, caches, sidecars, app port and timeout. A test run reads it at the commit it tests. Without one, the repository has one workflow, `default`.
_Avoid_: Config, compose file

**Sidecar**:
A container a workflow starts beside its tests, such as a database, from an image or a build. Tests reach it by its name.
_Avoid_: Service (the workflows file's key for sidecars is `services`)

**Workflow Execution**:
One workflow running its tests within one test run. Only so many may run at once, and the rest queue.
_Avoid_: Container (when meaning the unit of the limit), job

**Runner**:
The service that builds a workflow's images and runs its containers, on the API's request.
_Avoid_: Worker (the API's queue worker drives the runner), executor, agent

**Test Failure**:
A test run that finished and found at least one test not passing.
_Avoid_: Error (when meaning a failed test)

**Runner Error**:
A test, or a whole test run, that could not finish, so it says nothing about the code, such as a container that would not build, a test no workflow took, or a timeout.
_Avoid_: Failure (when meaning the run broke)

**Manual Test**:
An LLM check the chat model could not verify by itself, left for the approver to check by hand before approving.
_Avoid_: Skipped test, unverifiable test

**Needs Attention**:
A flag on a task in Testing whose test run ended in a runner error. The task waits there for a person. It clears when a new run starts or the task leaves Testing.
_Avoid_: Blocked, stuck

**Approve and Merge**:
A write member approving a task in User Check whose latest test run passed, after ticking its manual tests. The API merges the linked PR at the tested commit and moves the task to Closed.
_Avoid_: Land, ship

**Request Changes**:
A write member sending a task in User Check back to In Progress with feedback. The linked PR turns back into a draft, and the feedback is posted on it.
_Avoid_: Reject, deny (that is for a plan doc or a proposal)

**Take Over**:
A write member taking a task from automation to finish by hand. Its queued and running test runs are cancelled, and until they give it back, it moves freely and gets no new runs.
_Avoid_: Claim, override

## Accounts

**Linked GitHub Account**:
The one GitHub account a user links, with OAuth or a personal access token, or by signing in with GitHub. Every GitHub call made for them uses its credential, and it decides which projects they can see.
_Avoid_: Connected account, integration

**Provider**:
An OpenAI-compatible endpoint a user adds in Settings, with a base URL, an API key and the models chosen from it. The planning agents and automation call their models through providers.
_Avoid_: Vendor, integration, LLM (when meaning the endpoint)
