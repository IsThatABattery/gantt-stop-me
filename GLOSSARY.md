# Gantt Stop Me

A Kanban board over a GitHub repository, where cards are issues and an automated layer tests a card's code before a person approves and merges it.

## Board

**Project**:
One GitHub repository added to the app, with its own board.
_Avoid_: Repo, board (when meaning the repository)

**Task**:
One GitHub issue in a project, shown as a card on the board.
_Avoid_: Card (in code and docs), ticket, issue (when meaning the board item)

**Column**:
One of the board's fixed stages: Backlog, In Progress, Testing, User Check, Closed.
_Avoid_: Status, lane, stage

## Testing

**Linked PR**:
The open pull request that references a task's issue, whose code a test run checks and an approval merges.
_Avoid_: Branch (when meaning the code under test)

**Test**:
One item in the Tests section of a task's issue body: either a command to run or a plain-English check for an LLM.
_Avoid_: Test case, check

**Test Run**:
One execution of a task's tests against one commit of its linked PR.
_Avoid_: Job, build, pipeline

**Workflow**:
A named test environment, such as frontend, backend or acceptance, defined in the repository and chosen per test.
_Avoid_: Pipeline, job, suite

**Workflow Execution**:
One workflow running its tests within one test run. Only so many may run at once, and the rest queue.
_Avoid_: Container (when meaning the unit of the limit), job

**Runner**:
The service that builds a workflow's images and runs its containers, on the API's request.
_Avoid_: Worker, executor, agent

**Decision Model**:
A model that answers typed questions with probabilities instead of text. It picks the workflow for each test.
_Avoid_: Router, classifier

**Automation Owner**:
The project member whose GitHub access and LLM provider every test run in that project uses.
_Avoid_: Bot, runner user

**Test Failure**:
A test run that finished and found at least one test not passing.
_Avoid_: Error (when meaning a failed test)

**Runner Error**:
A test run that could not finish, so it says nothing about the code, such as a container that would not build or a timeout.
_Avoid_: Failure (when meaning the run broke)

**Manual Test**:
A test the LLM could not verify by itself, left for the approver to check by hand before approving.
_Avoid_: Skipped test, unverifiable test

**Needs Attention**:
A flag on a task in Testing whose test run ended in a runner error. The task waits there for a person.
_Avoid_: Blocked, stuck
