# Automation

The layer that tests a task's code and merges it once a person approves. When a task's linked PR becomes ready for review, the API runs the tests written on its issue in containers built from the repository, then moves the task on by the result.

It builds on [ARCHITECTURE.md](ARCHITECTURE.md). Every term in bold below is defined in [GLOSSARY.md](GLOSSARY.md). Nothing here writes code: code arrives as pull requests from people or outside agents.

The app is self-hosted by one user or a small group, on a machine they control. The **runner** builds and runs test containers on a Docker daemon they choose: the same machine, or a spare VM.

**For the implementer:** [Build order](#build-order) is the work, in order. Every later section is reference, and each slice names the sections it needs. `AGENTS.md`'s rules hold throughout:

- Run `docker compose run --rm api-test` after every API change.
- Put a test file beside every handler.
- Document every endpoint in `docs/` in the same commit.

When this doc and the code disagree, stop and ask. Don't pick one.

## Build order

Each slice is one branch and one PR, named `feature/automation-<slice>`. Stack a slice on another only where the table says it depends on it. Otherwise branch it from `main`. A slice is done when everything in its "Done when" column holds and `docker compose run --rm api-test` exits 0.

| # | Slice | Depends on | Builds | Reads | Done when |
| --- | --- | --- | --- | --- | --- |
| 1 | `runner` | — | `cmd/runner`, `internal/runner`, the `runner` and `runner-dev` Dockerfile stages, the `runner` and `runner-test` compose services, `.env.example` entries | [The runner](#the-runner), [Configuration](#configuration), [Security](#security) | Unit tests cover every [run flag](#run-flags) and every forbidden one, and every [extraction](#preparing-an-execution) rule with a hostile tarball for each. `docker compose --profile test run --rm runner-test` exits 0. It builds a small Dockerfile, starts a sidecar and the app, runs a command that calls the app, makes one `/http` request, opens the browser and lists its MCP tools, then leaves nothing labelled behind. |
| 2 | `parse` | — | Tests-section parser and workflow-file parser in `internal/automation` | [Tests on the issue](#tests-on-the-issue), [Workflows](#workflows) | Table tests cover every rule in both sections, including each example shown there and the default workflow. |
| 3 | `settings` | — | `migrations/007_automation.sql` (all tables), the automation settings endpoints, the webhook secret endpoint, pausing | [Turning automation on](#turning-automation-on), [Database](#database), [Endpoints](#endpoints) | Every settings endpoint has tests and a section in `docs/automation.md`. |
| 4 | `triggers` | 3 | GitHub client additions and their `githubtest` fakes, the PR record, poller, webhook receiver, trust check | [Linked PRs and triggers](#linked-prs-and-triggers), [Existing code to reuse](#existing-code-to-reuse) | Tests show each event in the trigger table having its effect exactly once, whether it arrives by poller, by webhook, or both. A fork PR and a read-only author each get one comment and no run. |
| 5 | `runs` | 1, 2, 4 | `internal/llm` and `llmtest`, `runnertest`, routing, the queue worker, LLM checks, outcomes, comments, restart recovery, run endpoints, move rules | [Routing](#routing), [Running tests](#running-tests), [Queue and restarts](#queue-and-restarts), [Outcomes](#outcomes), [Moves while automated](#moves-while-automated) | Tests drive a test run from trigger to each row of the outcomes table, with fake runner and fake LLM. Restart recovery is tested for one interruption and for two. Both move rules answer their `409`s. |
| 6 | `approve` | 5 | The approve endpoint, the manual-test checklist, merging | [Approve and merge](#approve-and-merge) | Tests cover a merge, a moved head before merging, a moved head at merge time, and a missing manual tick. |
| 7 | `ui` | 6, and the board pages from ARCHITECTURE.md | Automation settings, run badges, run history, Cancel and Approve | [Frontend](#frontend) | Each item in the Frontend section works against the running stack. |

The frontend is still the Vite starter page today. Slice 7 waits until the board and Settings pages in ARCHITECTURE.md exist.

## Overview

```mermaid
flowchart LR
  B[Backlog] --> IP[In Progress]
  IP -->|linked PR ready| T[Testing]
  T -->|pass| UC[User Check]
  T -->|test failure: PR back to draft| IP
  T -->|runner error| NA[Testing, needs attention]
  UC -->|Approve: merge| C[Closed]
  UC -->|head moved| T
```

```mermaid
flowchart LR
  API[api] -->|HTTP + bearer token| Runner[runner]
  Runner -->|Docker socket| Docker[(Docker daemon)]
  Docker --> Net
  subgraph Net[internal network, one per workflow execution]
    Test[test containers]
    App[app]
    Sidecars[sidecars]
    Browser[Playwright MCP]
  end
  Runner -. joins to make requests and proxy MCP .-> Net
```

1. A **linked PR** is opened as ready, or goes from draft to ready.
2. Its task moves to Testing from any open column, and a **test run** starts.
3. Each **test** on the issue is assigned a **workflow**: the one it names, or else the **decision model**'s pick.
4. For each workflow, the API sends the runner the PR's code. The runner builds the images from the repository's Dockerfiles and runs the tests in containers it configures itself. **Workflow executions** queue under an install-wide limit.
5. The result moves the task:
   - **Pass:** to User Check.
   - **Test failure:** to In Progress, and the PR turns back into a draft.
   - **Runner error:** the task stays in Testing, flagged **needs attention**.
6. In User Check, a write member ticks any **manual tests** and approves. The API merges the linked PR, then moves the task to Closed.

The runner does not use the repository's compose files or dev container. The repository decides only what goes into its images, never how they run.

## Turning automation on

Automation is off for every project until a write member turns it on. That member becomes the project's **automation owner**. Every test run uses the owner's GitHub credential and the owner's providers:

- a **decision model**: a provider and model that serve the OpenAI Decisions API, such as `gpt-6-luna` on `https://api.openai.com/v1`;
- a **chat model**: a provider and model with tool calling, used for LLM checks.

Both come from the owner's providers in Settings. The board shows who the owner is.

Automation pauses when the owner loses write access or GitHub stops accepting their credential. A paused project starts no new workflow executions. Triggers are still recorded, and queued executions wait. `paused_reason` holds why, and the board shows it with the owner's name. Any write member can take over as owner, which unpauses it.

The API checks the owner before it starts each workflow execution, using the same 5-minute permission cache as project routes. It also notices a rejected credential on any GitHub call.

Automation needs `RUNNER_URL` set on the API. Without it, the settings endpoints answer `404`.

## Linked PRs and triggers

A task's **linked PR** is the open pull request whose closing references include the task's issue, such as `Closes #42`. GitHub exposes those only through GraphQL `closingIssuesReferences`.

The API keeps a record of each open PR in an automated project, in `pull_requests`: head commit, draft state, author and linked issues. Both event sources compare a PR's new state against that record, under the project's lock, and then save it. So a PR seen by the webhook and later by the poller triggers only once.

- **Poller.** Every 30 seconds per automated project, the API lists open PRs with an `ETag`, so an unchanged repository costs nothing. When the list changes, it reads the changed PRs' closing references in one GraphQL query.
- **Webhook.** Optional. Project settings show the payload URL, `{PUBLIC_URL}/api/github/webhook/{projectID}`, and generate a secret. A repository admin adds the webhook on GitHub by hand, for `pull_request` events, with content type `application/json`. No new OAuth scope is needed. The poller keeps running either way, so missed deliveries are caught. A delivery is only a hint: the API re-reads the PR before acting.

What each change does:

| Change to a PR | Effect |
| --- | --- |
| First seen ready, or draft → ready | Move each linked task to Testing from any open column and start a test run. |
| Ready → draft | Cancel the task's queued or running test run, if any. |
| New head commit while ready | Nothing. The approval check catches it. |
| Closed, merged, or gone from the open list | Delete the record. The existing issue sync moves the task. |

Dragging a task into Testing by hand also starts a test run, for its current linked PR. A drag with no linked PR is a runner error.

A PR runs only when both are true:

- its head repository id equals the project's repository id, so it is not a fork;
- its author has write access or above, checked with `GET /repos/{id}/collaborators/{login}/permission`.

Any other PR that would start a run, because it is ready and linked to an open task, gets one comment saying it was not run, posted once per PR, and nothing else happens. Tasks in Closed are never moved by a trigger.

These cases start a test run that ends at once in a runner error, so the task waits in Testing as needing attention:

- the issue has no Tests section;
- more than one open, ready PR is linked to the issue.

## Workflows

Workflows live in the repository, at `.gantt/workflows.yml`. The API reads that file from the linked PR's head commit, with `GET /repos/{id}/contents/.gantt/workflows.yml?ref={sha}`, so it changes with the code it tests.

If the file is absent at that commit, the repository has one workflow, `default`, with every field at its default: the repository-root `Dockerfile`, last stage, no sidecars, no `port`. A repository with neither file gets a build error, which is a runner error.

This repository's own file would be:

```yaml
workflows:
  backend:
    description: Go API, endpoints, database
    context: src/back/api
    target: build              # the stage that has the source and module cache
    env:
      TEST_DATABASE_URL: postgres://gantt:gantt@test-db:5432/gantt?sslmode=disable
    cache: [/root/.cache/go-build]
    services:
      test-db:
        image: postgres:18
        env: { POSTGRES_USER: gantt, POSTGRES_PASSWORD: gantt, POSTGRES_DB: gantt }
        health: pg_isready -U gantt -d gantt
    timeout: 20m

  frontend:
    description: React UI seen in a browser, against the real API
    context: src/front
    port: 80                   # the app, at http://app:80
    services:
      db:
        image: postgres:18
        env: { POSTGRES_USER: gantt, POSTGRES_PASSWORD: gantt, POSTGRES_DB: gantt }
        health: pg_isready -U gantt -d gantt
      api:                     # nginx in the image proxies /api to http://api:8080
        context: src/back/api
        env:
          DATABASE_URL: postgres://gantt:gantt@db:5432/gantt?sslmode=disable
          PUBLIC_URL: http://app
          ENCRYPTION_KEY: rWhMzl7tTjnG0yUf8pzqa3gBBWudv5DWpXxrF6sgzB0=   # the public dev key
        port: 8080
    timeout: 30m
```

Workflow fields:

| Field | Rule |
| --- | --- |
| name (the key) | Lowercase letters, digits and `-`, 2 to 32 characters, starting with a letter. Unique in the file. |
| `description` | Required. The decision model reads it. |
| `context` | Build context, relative to the repository root. Default `.`. It must stay inside the repository. |
| `dockerfile` | Path relative to `context`. Default `Dockerfile`. |
| `target` | Build stage. Default: the last stage. |
| `env` | String map, set in every container built from the test image. The values are in the repository, so they are not secret. |
| `cache` | Absolute paths in test containers that keep their contents between runs of this workflow in this project, such as a compiler cache. At most 4. |
| `port` | The app's port when the test image runs its default command. When set, the app runs throughout the execution as hostname `app`, and LLM checks get the browser. |
| `services` | Sidecars, below. At most 8. |
| `timeout` | Default `30m`, at most `2h`. A Go duration. The limit for the whole workflow execution, builds included. |

Sidecar fields. A sidecar has either `image`, or a build from `context`, `dockerfile` and `target` with the rules above:

| Field | Rule |
| --- | --- |
| name (the key) | As for workflows. Also its hostname on the network. `app` and `browser` are reserved. |
| `image` | A registry image reference. Pulled at build time. |
| `env` | String map. |
| `health` | A command run in the sidecar with `sh -c` until it exits 0. Needs a shell in the image. |
| `port` | A TCP port the runner dials until it accepts. Use this for images with no shell, such as distroless. |

Sidecars start in file order, and each must be healthy before the next starts. One with neither `health` nor `port` counts as healthy once it is running. A sidecar not healthy within 2 minutes is a runner error.

Parse the file with a strict YAML decoder that rejects unknown fields. A file that doesn't parse, or breaks a rule, is a runner error naming the field. The format is kept simple enough for the UI to edit it later, by committing to the repository.

## Tests on the issue

Tests are a checklist under a `## Tests` heading in the issue body. The card is enough to describe any test: a command, a multi-line script, or a check in plain English that an LLM carries out with a shell, HTTP and a browser.

````markdown
## Tests
- [ ] `go test ./internal/board -run Move`
- [ ] [backend] Store and board tests pass together
  ```sh
  go vet ./...
  go test ./internal/store ./internal/board
  ```
- [ ] Signing up with an empty email answers 400 with code invalid_email
- [ ] Dragging a card to Closed closes the issue on GitHub
  1. Sign up and add a project.
  2. Drag any card to Closed.
  3. The card stays in Closed after a reload.
- [ ] The empty board looks right on a phone-sized screen
````

Parsing rules:

- **Section.** It starts at the first line matching `## Tests`, in any case, with optional trailing spaces. It ends at the next level-1 or level-2 heading, or the end of the body.
- **Items.** A line in the section starting with `- ` or `* `, at no indent, starts a test. Every following line that is indented or blank, up to the next such line, belongs to it as its **block**. The block's common indent is removed. Other lines are ignored.
- **Checkbox.** After the bullet, an optional `[ ]`, `[x]` or `[X]` is dropped.
- **Pin.** Then an optional `[name]`, naming a workflow, pins the test to that workflow and is dropped. A name not in the workflow file is a runner error for that test.
- **Text.** What is left of the line, trimmed, is the test's text.

Each test is one of two kinds:

- **Script**, when either:
  - the text is exactly one inline code span and the block is empty. The command is the span's contents.
  - the block is exactly one fenced code block tagged `sh`, `bash`, `shell` or untagged. The command is the fence's contents, and the text is only a name.

  A script passes when the command exits 0.
- **LLM check**, for anything else. The text and block together are the check.

A section with no items counts as no Tests section. More than 64 tests is a runner error.

## Routing

Each test needs one workflow:

1. A pinned test goes to its pin.
2. When the workflow file has one workflow, or is absent, every test goes there.
3. Otherwise the API makes one call to the owner's decision model for all the remaining tests:

```
POST {baseUrl}/decisions
```

- **Input:** the issue's title, every workflow's name and description, and the tests.
- **Questions:** one `choice` question per unpinned test, over the workflow names.

```json
{
  "model": "gpt-6-luna",
  "input": "Issue: Sign-up rejects empty emails\n\nWorkflows:\n- backend: Go API, endpoints, database\n- frontend: React UI seen in a browser, against the real API\n\nTests:\n3. Signing up with an empty email answers 400 with code invalid_email",
  "questions": [
    {
      "type": "choice",
      "name": "test_3",
      "instructions": "Which workflow can run test 3?",
      "choices": [
        { "value": "backend", "description": "Go API, endpoints, database" },
        { "value": "frontend", "description": "React UI seen in a browser, against the real API" }
      ]
    }
  ]
}
```

The answer has one entry in `answers` per question, matched by `name`:

```json
{
  "answers": [
    {
      "type": "choice",
      "name": "test_3",
      "choice": "backend",
      "probabilities": [{ "value": "backend", "probability": 0.92 }, { "value": "frontend", "probability": 0.08 }],
      "confidence": 0.9
    }
  ]
}
```

An answer of `"type": "refusal"`, or a missing one, counts as no workflow above 0.5.

Each test runs in its most likely workflow. A test whose top probability is under 0.5 is not run. It gets a runner error naming the test and its best guesses, and the card's author can pin it. The test run stores every probability with its results.

## The runner

The runner is a separate service, a Go binary in the same module at `cmd/runner`. It is the only part of the app with Docker access, and it holds no secret except its own token:

- no `ENCRYPTION_KEY`, GitHub credential or provider key;
- no database access.

The API owns the queue and every decision. The runner only builds, starts, runs and removes containers when the API asks.

The runner must itself run as a container on the Docker daemon it drives, because it joins each execution's network. It finds its own container by its hostname, which Docker sets to the container id.

- **On one machine,** it mounts `/var/run/docker.sock`.
- **On a spare VM,** to isolate test containers from the machine serving the app, run the `runner` service there with its own Docker, and point `RUNNER_URL` at it. The VM's port 8081 then needs to be reachable from the API only.

The runner sits on several networks at once: its own compose network and every live execution's. A name such as `app`, `db` or `api` can resolve on more than one of them, even to the app's own services. So the runner never dials an execution's containers by name. It records each container's address on the execution's network when it starts it, and `/http`, `/mcp`, `port` health checks and the wait for the app dial that address. Test containers sit on their execution's network only, so they reach each other by name.

### Runner image

It is a new `runner` stage in `src/back/api/Dockerfile`, based on the official `docker:<version>-cli` image with a pinned version, which includes Buildx. A `runner-dev` stage adds the Go toolchain to the same base, for `runner-test`. The runner shells out to the `docker` CLI for every Docker operation. The API image stays distroless with no Docker access.

When it calls `docker`, the runner passes only `PATH`, `HOME` and `DOCKER_HOST` to the child process. Nothing else from its environment reaches a build or container.

### Runner HTTP API

The runner listens on port 8081 on the compose network and is not published. Every request except `GET /health` needs `Authorization: Bearer {RUNNER_TOKEN}`, or it answers `401`. Execution ids are the API's `workflow_executions.id`.

| Method and path | Does |
| --- | --- |
| `GET /health` | Answers `ok`. |
| `POST /executions/{id}` | Prepare an execution. The `multipart/form-data` body has a `spec` part (JSON, below) and a `source` part (the GitHub tarball, `.tar.gz`). The runner builds images, creates the network, and starts the sidecars, then the app if `port` is set. It answers when they are ready: `200 {"log": "…"}`, or `422 {"error": "build" or "start", "log": "…"}`. |
| `POST /executions/{id}/commands` | `{"command": "…"}`. Runs `sh -c` with the command in a fresh container from the test image, and answers `{"exitCode": 0, "output": "…"}`. The output is the last 64 KB of stdout and stderr together. |
| `POST /executions/{id}/http` | `{"method": "POST", "url": "http://app:80/api/auth/signup", "headers": {…}, "body": "…"}`. Makes one HTTP request on the execution's network and answers `{"status": 400, "headers": {…}, "body": "…"}`, with the body cut at 64 KB. The URL must be `http`, and its host must be `app` or a sidecar's name. Redirects are not followed. 30-second limit. |
| `POST /executions/{id}/browser` | Start the browser. Answers once its MCP endpoint accepts connections, or `422` after 2 minutes. Safe to repeat. `409` when the spec has no `port`. |
| `POST /executions/{id}/mcp` | Forward one MCP request to the browser's streamable HTTP endpoint and return its answer. `Mcp-Session-Id` passes through both ways. |
| `DELETE /executions/{id}` | Remove the execution's containers and network. Keep images and cache volumes. Safe to repeat. |
| `DELETE /executions` | The same for every execution with this runner's prefix. |

The `spec` part is the workflow after defaults and validation, plus the execution's `project` id:

```json
{
  "project": 1,
  "workflow": "backend",
  "context": "src/back/api", "dockerfile": "Dockerfile", "target": "build",
  "env": {"TEST_DATABASE_URL": "…"},
  "cache": ["/root/.cache/go-build"],
  "port": null,
  "services": [{"name": "test-db", "image": "postgres:18", "env": {}, "health": "pg_isready -U gantt -d gantt", "port": null}]
}
```

`DELETE` on an execution, or the API dropping a request, stops whatever that request was running. The execution's containers stay until `DELETE` removes them.

### Preparing an execution

1. **Source.** Read the tarball into a fresh directory under `/work`, a tmpfs inside the runner container. While extracting:
   - Drop the first path segment, the `owner-repo-sha/` folder GitHub adds.
   - Reject absolute paths, `..` segments, device files, and links that point outside the directory.
   - Stop at 500 MiB uncompressed. Larger is a build error.

   Git submodules and LFS contents are not in tarballs, so builds cannot use them.
2. **Build.** Run `docker build` for the test image and for each sidecar that has a build:
   - Pass `--target` when one is set, and `-f` with the Dockerfile.
   - Tag each image `{prefix}-p{project}-{workflow}` for the test image, or `{prefix}-p{project}-{workflow}-{service}` for a sidecar.
   - Pass no `--allow` entitlements, build secrets or SSH agents.

   Builds have network access, so dependencies install at build time. BuildKit's layer cache on the daemon is the image cache. An unchanged layer, such as `go mod download` with the same `go.mod`, is reused on the next build.
3. **Pull.** Pull sidecar `image`s.
4. **Network.** Create `{prefix}-x{id}` with `docker network create --internal`. An internal network has no route to the internet, the LAN or the host. Containers on it reach each other by name. The runner connects its own container to the network.
5. **Sidecars.** Start each one in order with the run flags below, using its name as hostname and network alias. Wait for it to be healthy before starting the next.
6. **App.** If `port` is set, start the test image with its default command and the workflow's `env`, under hostname `app`. Wait up to 2 minutes for `app:{port}` to accept TCP. The app keeps running until the execution is removed.
7. **Clean up the source.** Delete the directory. Images now hold everything the containers need.

### Run flags

Every container the runner starts uses the same fixed flags. The workflow file cannot change them:

| Flag | Value |
| --- | --- |
| `--network` | The execution's internal network only. |
| `--security-opt` | `no-new-privileges`. Docker's default seccomp and AppArmor profiles stay on. |
| `--cap-drop` | `NET_RAW`, `MKNOD`. Docker's other default capabilities stay, so images like `postgres` can start as root and drop to their own user. |
| `--memory`, `--cpus`, `--pids-limit` | `4g`, `2`, `1024` per container. |
| `--tmpfs` | `/tmp`. |
| `--label` | `gantt.prefix={prefix}`, `gantt.execution={id}`. |
| Mounts | Only the workflow's `cache` paths, each a named volume `{prefix}-p{project}-{workflow}-cache{n}`, on test containers and the app. |

These are never used: `--privileged`, `--cap-add`, `--device`, host bind mounts, host network, PID or IPC namespaces, published ports, and the Docker socket. Containers run as the image's own user.

shortcut: the limits are constants. Make them settings when a repository needs more.

## Running tests

The API drives each workflow execution in this order. The workflow's `timeout` covers all of it. Past the timeout, the API cancels its open request, sends `DELETE`, and records a runner error for every test not yet finished.

1. **Prepare:** `POST /executions/{id}`. Sidecars and the app are now running. A `422` is a runner error for every test in the execution, and the runner's log goes into the execution's log tail.
2. **Scripts:** one `POST …/commands` per script, in issue order. Each runs in a fresh container on the execution's network, so it can call the app and the sidecars by name, but not the internet. Exit 0 passes. The test image needs `sh`.
3. **LLM checks:** one at a time, in issue order.
4. **Clean up:** `DELETE /executions/{id}`, always, whatever happened.

### LLM checks

For each check, the API runs a tool-calling loop over `POST {baseUrl}/chat/completions` with the owner's chat model. The model gets these tools:

| Tool | When offered | Carried out by |
| --- | --- | --- |
| `run_command(command)` | Always | `POST …/commands`. Each call is a fresh container, so files don't carry over from one call to the next. State lives in the app and sidecars. |
| `http_request(method, url, headers, body)` | When the app or any sidecar exists | `POST …/http`. |
| The browser's tools, as listed by MCP `tools/list` | When `port` is set | `POST …/mcp`. The API calls `POST …/browser` before the first check that needs it. |
| `report(outcome, reason)` | Always | The API. `outcome` is `pass`, `fail` or `unverifiable`. |

The browser is the Playwright MCP image, pinned in the runner's code, started headless and isolated, serving streamable HTTP on port 8931. Check its flags against the pinned version's README.

The first message holds:

- the issue's title and body, as context;
- the check's text and block, as the thing to verify;
- the workflow's name, the app's URL `http://app:{port}` if any, and the sidecars' names;
- the rule: verify with the tools, then call `report` once. Use `unverifiable` only for what no tool can check, such as how something looks to a person, and say what a person should check.

The check ends at the first `report` call. After a report, if the browser was used, the API takes a screenshot through the browser's screenshot tool. The result keeps the reason and a transcript of each tool call with its answer, cut to 64 KB.

| Report | Test result |
| --- | --- |
| `pass` | `passed` |
| `fail` | `failed` |
| `unverifiable` | `manual`: a manual test for the approver |
| None after 50 tool calls, or the chat API failing | `error`, a runner error |

## Queue and restarts

The queue is `workflow_executions` where `status = 'queued'`. A worker goroutine in the API claims from it:

- in `id` order, across the whole install, with `FOR UPDATE SKIP LOCKED`;
- at most `MAX_CONCURRENT_WORKFLOWS` running at once (default 1), however many containers each starts;
- never two at once for the same project and workflow, so cache volumes and image tags are never shared mid-run;
- never for a paused project.

When the API starts, it calls `DELETE /executions`, then puts executions that were running back on the queue once. A second interruption is a runner error. When the runner starts, it removes every container and network labelled with its prefix. A runner restart therefore fails the API's open requests, and the API requeues those executions the same way.

The prefix keeps installs apart on a shared daemon: the host stack uses `gantt`, the dev container `gantt-dev`, and `runner-test` uses `gantt-test`.

## Outcomes

A test run ends when all of its workflow executions end. Its status comes from its test results, first match wins:

| Results include | Run status | Task | Linked PR | Issue comment |
| --- | --- | --- | --- | --- |
| Any `failed` | `failed` | Moves to In Progress | Turned back into a draft, with GraphQL `convertPullRequestToDraft` | Lists the failures |
| Any `error` | `error` | Stays in Testing, `needs_attention` set to the first error's reason | Unchanged | Names the errors |
| Only `passed` and `manual` | `passed` | Moves to User Check | Unchanged | Lists the manual tests for the approver |
| Cancelled | `cancelled` | Stays in Testing, unlocked | Unchanged | None |

A test failure outranks a runner error, because it already says something about the code. The API makes these moves as ordinary moves: labels written to GitHub, the project's lock held.

Every finished run posts one comment on the issue, with the owner's credential. It has one line per test and a link to `{PUBLIC_URL}/projects/{id}/tasks/{taskID}?run={runID}`.

The run's full record stays in Postgres and shows as run history in the task's detail view:

- each test's workflow, probability, result, reason, and LLM transcript;
- the last 64 KB of each execution's log: build and start output, then each script's output;
- each LLM check's screenshot, if it used the browser.

Re-running starts a whole new test run: mark the PR ready again, or drag the task into Testing.

## Moves while automated

These rules apply to projects with automation on. Projects without it keep moving freely.

- **Run lock.** While a task's test run is queued or running, moving the task out of Testing answers `409 run_in_progress`. A write member can press **Cancel run** to stop it and unlock the task.
- **User Check gate.** A task may enter User Check only if its latest test run is `passed` on the linked PR's current head commit. Otherwise the move answers `409 not_tested`. The API makes this move itself on a pass.
- **Into Testing** by hand starts a test run.
- **Leaving Testing** clears `needs_attention`.
- **Other moves**, including into Closed, stay free.

## Approve and merge

In User Check, a write member ticks every manual test and presses **Approve**:

`POST /api/projects/{id}/tasks/{taskID}/approve` with `{ "checked": [resultID, …] }`

1. The task must be in User Check with a `passed` latest test run, else `409 not_tested`. `checked` must hold exactly that run's `manual` result ids, else `422 unchecked_manual_tests`.
2. The API reads the linked PR's head.
   - If it is still the commit that passed, merging continues.
   - If not, the task moves to Testing, a new test run starts, and it answers `409 head_moved`.
3. The API merges with `PUT /repos/{id}/pulls/{n}/merge`, using the first method the repository allows: squash, then merge commit, then rebase. It passes the tested commit as `sha`, so GitHub refuses the merge if the head moved at the last moment. That refusal is handled like step 2's second case.
4. The API saves `approved_by` and `approved_at` on the run, and moves the task to Closed as an ordinary move.

Whether the branch is deleted follows the repository's "automatically delete head branches" setting.

## Frontend

React pages, using TanStack Query like the rest of the app:

- **Project settings → Automation.**
  - Turn automation on or off, or take over as owner.
  - Pick the decision model and the chat model from the caller's providers.
  - Show the owner and any pause reason.
  - Show the webhook payload URL. "Generate secret" shows the secret once.
- **Board cards.** A badge for the latest test run: queued, running, passed, failed or error. Needs-attention cards show their reason.
- **Task detail.**
  - Run history, newest first. Each run lists its tests with workflow, probability, result and reason. A test opens its log tail, LLM transcript and screenshot.
  - **Cancel run** while a run is queued or running.
  - In User Check: the manual tests as a checklist, and **Approve**, enabled once every box is ticked.
- **While a run is queued or running,** refetch its task every 5 seconds.
- **Moves refused** with `run_in_progress`, `not_tested` or `head_moved` snap back with a message saying why.

## Existing code to reuse

| Need | Use |
| --- | --- |
| Access checks on project routes | `projects.Check(w, r, projects.Write)` or `projects.Read`. |
| Moving a task (outcomes, triggers, approve) | The core of `board/move.go`, pulled into a function that takes a transaction rather than a request, so the board and automation share one move. It keeps `projects.Serialize` before the transaction and `projects.Lock` inside it. |
| The owner's GitHub token, with no request | `github.Token(ctx, db, box, userID)`, which already loads a user's `github_links.credential_enc` and opens it with `secret.Box`. |
| GitHub REST calls | New methods on `github.Client` built on `c.do`. GraphQL is new: one `graphql` method posting to `{API}/graphql`. Fake each one in `githubtest`. |
| Calls to provider URLs | `settings/modelsClient` and `dialGuard`, moved into `internal/llm` so decisions and chat calls keep the SSRF guard and `ALLOW_PRIVATE_PROVIDER_URLS`. Raise the timeout to 120 seconds for chat. |
| Provider keys and the webhook secret | `secret.Box` `Seal` and `Open`. |
| Errors | `httpx.Error` with the codes in this doc, and `httpx.Internal`. |
| The webhook route | `httpx.CheckOrigin` wraps the whole mux in `app.New`. Exempt `POST /api/github/webhook/` from it. The HMAC check replaces the Origin check there. |
| Background work | Start the poller and the queue worker from `cmd/api/main.go`, and stop them on shutdown. Give apptest a way to run one poll or one claim on demand, so tests never sleep. |
| Endpoint tests | `apptest.New(t, func(c *app.Config) { … })`, with fake runner and fake LLM fields added to `app.Config` beside `GitHub`. |

## Configuration

Added to `.env.example`:

| Variable | Service | Purpose |
| --- | --- | --- |
| `RUNNER_URL` | `api` | The runner's base URL, such as `http://runner:8081`. Unset turns automation off. |
| `RUNNER_TOKEN` | `api`, `runner` | Shared bearer token, 32 random bytes, base64. The runner refuses to start without one. |
| `MAX_CONCURRENT_WORKFLOWS` | `api` | Workflow executions running at once across the install. Default `1`. |
| `RUN_PREFIX` | `runner` | Prefix for every image, container, network and volume the runner makes. Default `gantt`. |

New services in `docker-compose.yml`:

```yaml
  runner:
    build:
      context: src/back/api
      target: runner
    environment:
      RUNNER_TOKEN: ${RUNNER_TOKEN-<dev-only default, like ENCRYPTION_KEY>}
      RUN_PREFIX: ${RUN_PREFIX:-gantt}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    tmpfs:
      - /work                 # source checkouts

  # Runner tests against real Docker: docker compose --profile test run --rm runner-test
  runner-test:
    profiles: [test]
    build:
      context: src/back/api
      target: runner-dev      # Go toolchain plus the docker CLI
    volumes:
      - ./src/back/api:/src
      - go-cache:/go
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      GOCACHE: /go/cache
      RUN_PREFIX: gantt-test
      RUNNER_DOCKER_TESTS: "1"
    command: go test ./internal/runner/...
```

The `api` service gains `RUNNER_URL: ${RUNNER_URL:-http://runner:8081}` and `RUNNER_TOKEN`, and `depends_on: runner`. It gets no Docker socket.

Tests that need Docker run only when `RUNNER_DOCKER_TESTS=1`, so `api-test` stays socket-free.

In the dev container, add a **Runner** entry to Run and Debug. It runs the runner under Delve with `RUN_PREFIX=gantt-dev`, using the dev container's existing Docker access. The dev container is itself a container on the host daemon, so it can join execution networks the way the `runner` service does.

## Code layout

| Path | Holds |
| --- | --- |
| `cmd/runner/` | The runner's `main`. |
| `internal/runner/` | The runner's HTTP server, tar extraction, the `docker` command builder, and the API's client for the runner. |
| `internal/runner/runnertest/` | A fake runner for API tests. It records requests and answers from a script, like `githubtest`. |
| `internal/automation/` | API side: settings, poller, webhook, PR record, triggers, queue worker, Tests parsing, workflow file parsing, routing, LLM checks, outcomes, approve. One handler file per endpoint, with its test beside it. |
| `internal/llm/` | The guarded HTTP client, the OpenAI Decisions client, and the chat tool-calling loop. |
| `internal/llm/llmtest/` | A fake provider serving `/decisions` and `/chat/completions` from a script. |
| `migrations/007_automation.sql` | The tables below. |

The workflow file needs a YAML parser. Add the current stable release of `go.yaml.in/yaml`, with `docker compose run --rm --no-deps api-test go get` and `go mod tidy`. Everything else uses the standard library.

## Endpoints

Each lands with a section in `docs/automation.md`, in the shape of `docs/accounts.md`.

| Method and path | Purpose |
| --- | --- |
| `GET /api/projects/{id}/automation` | Whether automation is on, who owns it, its models, and why it is paused, if it is. Read permission. |
| `PUT /api/projects/{id}/automation` | Turn it on, or take over, as the caller, with `{decisionProviderId, decisionModel, chatProviderId, chatModel}` from the caller's providers. Write permission. |
| `DELETE /api/projects/{id}/automation` | Turn it off. Queued and running executions are cancelled. Write permission. |
| `POST /api/projects/{id}/automation/webhook` | Generate a new webhook secret and return it once, with the payload URL. Write permission. |
| `POST /api/github/webhook/{projectID}` | GitHub deliveries. No session or `Origin` check. The `X-Hub-Signature-256` HMAC must match the project's secret, or it answers `401`. Answers `202` and acts in the background. |
| `GET /api/projects/{id}/tasks/{taskID}/runs` | The task's test runs, newest first, with their status. Read permission. |
| `GET /api/projects/{id}/runs/{runID}` | One test run with its executions, results, log tails and transcripts. Read permission. |
| `GET /api/projects/{id}/runs/{runID}/results/{resultID}/screenshot` | An LLM check's screenshot, as PNG. Read permission. |
| `POST /api/projects/{id}/runs/{runID}/cancel` | Cancel a queued or running test run. Write permission. |
| `POST /api/projects/{id}/tasks/{taskID}/approve` | Approve and merge. Write permission. |

`GET /api/projects/{id}/board` and the task detail gain each task's latest run status and `needsAttention`. `POST /api/projects/{id}/tasks/{taskID}/move` gains the rules in [Moves while automated](#moves-while-automated).

## Database

`migrations/007_automation.sql`:

```sql
create table project_automation (
  project_id         bigint primary key references projects on delete cascade,
  owner_id           bigint not null references users on delete cascade,
  decision_provider  bigint not null references providers on delete cascade,
  decision_model     text not null,
  chat_provider      bigint not null references providers on delete cascade,
  chat_model         text not null,
  paused_reason      text,               -- null while running
  webhook_secret_enc bytea,              -- sealed with secret.Box
  pulls_etag         text,
  enabled_at         timestamptz not null default now()
);

-- Last seen state of each open PR, so the poller and webhook act on each change once.
create table pull_requests (
  project_id    bigint references projects on delete cascade,
  number        integer,
  head_sha      text not null,
  head_repo_id  bigint,                  -- null when the fork was deleted
  author_id     bigint not null,         -- GitHub user id
  draft         boolean not null,
  linked_issues integer[] not null default '{}',
  refused       boolean not null default false, -- the "not run" comment was posted
  primary key (project_id, number)
);

create table test_runs (
  id          bigserial primary key,
  task_id     bigint not null references tasks on delete cascade,
  pr_number   integer,                   -- null when no linked PR was found
  head_sha    text,
  status      text not null,             -- 'queued', 'running', 'passed', 'failed', 'error', 'cancelled'
  reason      text,                      -- runner error or cancel reason for the whole run
  approved_by bigint references users on delete set null,
  approved_at timestamptz,
  created_at  timestamptz not null default now(),
  finished_at timestamptz
);

create table workflow_executions (
  id          bigserial primary key,
  run_id      bigint not null references test_runs on delete cascade,
  project_id  bigint not null references projects on delete cascade,
  workflow    text not null,
  spec        jsonb not null,            -- as sent to the runner
  status      text not null,             -- 'queued', 'running', 'passed', 'failed', 'error', 'cancelled'
  attempts    integer not null default 0,
  log_tail    text not null default '',  -- last 64 KB
  started_at  timestamptz,
  finished_at timestamptz
);
create index on workflow_executions (id) where status = 'queued';

create table test_results (
  id            bigserial primary key,
  run_id        bigint not null references test_runs on delete cascade,
  execution_id  bigint references workflow_executions on delete cascade, -- null when routing refused it
  position      integer not null,        -- order in the Tests section
  text          text not null,           -- the line's text and its block
  kind          text not null,           -- 'script' or 'llm'
  pinned        boolean not null,
  probabilities jsonb not null default '{}', -- workflow name to probability
  status        text not null,           -- 'pending', 'passed', 'failed', 'manual', 'error', 'skipped'
  detail        text not null default '',    -- exit code and output, or the model's reason
  transcript    jsonb,                   -- an LLM check's tool calls and answers, cut to 64 KB
  screenshot    bytea
);

alter table tasks add column needs_attention text; -- a runner error's reason; cleared when a run starts or the task leaves Testing
```

## Security

What a test container can reach:

- **The repository controls only image contents.** It supplies Dockerfiles, a sidecar list and a few fields. The runner chooses every runtime flag, and none of them can grant host mounts, privileges, devices, host networking or the socket.
- **Builds get no entitlements, secrets or SSH agent.** They have network access, so a Dockerfile can download anything, but it cannot reach the host's filesystem.
- **At run time, a container reaches only its own execution's network.** That network has no internet, no LAN and no host, so test code cannot probe the network you self-host on. The runner joins it too, and its API refuses requests without the token.
- **No app secrets go into a container.** `env` values come from the repository itself.

What a compromise reaches:

- **The API has no Docker access.** A bug in the web-facing API does not give root on the host.
- **The runner holds the socket**, which is root on its daemon's host, but it has no app secrets and no database access. Running it on a spare VM confines even a container escape to that VM.
- **Containers share their host's kernel.** For stronger isolation on Linux, install gVisor and make it the daemon's default runtime, or turn on `userns-remap`. Docker Desktop on macOS already runs containers inside a VM.

Who and what is trusted:

- **Only same-repository PRs by authors with write access run.** Cache volumes are shared by the runs of one workflow in one project, so such an author could poison them for later runs. That is within the trust write access already gives.
- **Webhooks** are checked with HMAC before the body is parsed. The secret is stored sealed and shown once.
- **LLM checks** read pages, responses and command output that the PR's code produces, and those can carry instructions. The model's tools reach only the execution's own containers, so the worst a hostile PR can do is a false pass or a false manual test. A person still approves before anything merges.
- **The owner's credential** makes the merges, comments and draft conversions, so GitHub shows them as the owner's actions.

## Not in this design

- A coding agent that implements tasks.
- Cloudflare Clef and other decision models besides the OpenAI Decisions API.
- The repository's compose files or dev container as test environments.
- Editing workflows in the UI.
- Running fork PRs, or PRs by authors without write access.
- Internet access for test containers at run time.
- Creating webhooks through the API.
- A per-project or UI-set concurrency limit, or per-workflow resource limits.
- Starting a run on every push.
- More than one runner.

## Open questions

1. **Ticking the checklist.** Should passing tests tick their boxes in the issue body?
