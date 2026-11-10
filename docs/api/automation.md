# Automation

Automation tests a task's linked pull request and merges it once a member approves. A pull request of the project's repository that closes a task's issue, such as with "Closes #12", becomes ready for review, and the task moves to Testing with a new test run. Each of the task's [tests](#tests-section) goes to a workflow of the repository's [workflows file](#workflows-file), as [Routing](#routing) says, and the runner builds that workflow's images from the pull request's code and [runs the tests](#running-tests) in them. The run's [outcome](#outcomes) moves the task: a pass to User Check, a test failure back to In Progress with the pull request turned back into a draft, and a runner error leaves it in Testing, flagged as needing attention. In User Check a write member checks the manual tests by hand and [approves](#approve-and-merge), and the API merges the pull request and moves the task to Closed. While automation is on, board moves follow [Moves while automated](#moves-while-automated). Terms are in the [glossary](../glossary.md).

A write member turns automation on and becomes its automation owner: every test run uses their GitHub credential and their decision and chat models. The API reads each automated project's open pull requests from GitHub every 30 seconds with the owner's credential, and an optional [webhook](#receive-webhook) tells it of a change sooner. Before it plans a test run, starts a workflow execution, ends a run, posts a run's comment or merges, it checks the owner, from the same 5-minute cache as project routes. When they have unlinked GitHub, lost write access, or GitHub no longer accepts their credential, automation pauses, and `pausedReason` says why. A credential GitHub refuses is read again first, since a refresh by another request retires the old token: a new one is checked instead, and only GitHub refusing that too pauses. An expired OAuth token that GitHub could not be reached to refresh pauses nothing, and the work waits. While paused, nothing is planned, started, ended, commented on or merged, and queued work waits. Pull requests are still read, so tasks still move and runs still queue, unless the owner unlinked GitHub or GitHub refused their credential. [Turn on automation](#turn-on-automation) again, by the owner or another write member, unpauses it.

Every endpoint here answers `404 not_found`, as an unknown route, while the API has no `RUNNER_URL`.

## Show automation

Returns whether automation is on, who owns it, its models, why it is paused if it is, and the webhook's payload URL.

`GET /api/projects/{id}/automation`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |

**Output**

| Status | Body |
| --- | --- |
| `200` | automation settings, or `{"enabled": false}` alone while it is off |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

`owner` is the automation owner's user `id` and GitHub `login`, the `login` `null` once they unlink GitHub. `decisionInstructions` and `chatInstructions` are the owner's own instructions to the decision model and to LLM checks, `""` for none. `pausedReason` is `null` while automation runs, else why it paused, such as `ada lost write access to the repository.` `webhookSecretSet` says whether a webhook secret was generated.

```json
{
  "enabled": true,
  "owner": { "id": 1, "login": "ada" },
  "decisionProviderId": 1,
  "decisionModel": "gpt-6-luna",
  "chatProviderId": 2,
  "chatModel": "gpt-5",
  "decisionInstructions": "",
  "chatInstructions": "Prefer curl over the browser.",
  "pausedReason": null,
  "payloadUrl": "https://gantt.example.com/api/github/webhook/1",
  "webhookSecretSet": false
}
```

```json
{ "enabled": false }
```

## Turn on automation

Turns automation on with the caller as its owner, or takes it over: the caller replaces the owner, models and instructions and unpauses it, and the webhook secret and the pull request records stay. When automation was off, the next read of GitHub sees every open pull request for the first time, so each ready one that closes an open task's issue starts a test run, as the [triggers](#receive-webhook) say.

`PUT /api/projects/{id}/automation`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `decisionProviderId` | body | integer | one of the caller's providers |
| `decisionModel` | body | string | one of that provider's chosen `models`; picks workflows, so it should serve the OpenAI Decisions API, `POST {baseUrl}/decisions` |
| `chatProviderId` | body | integer | one of the caller's providers, the same or another |
| `chatModel` | body | string | one of that provider's chosen `models`; runs LLM checks, so it should serve the OpenAI Responses API, `POST {baseUrl}/responses`, with tool calling, `store` false and the reasoning's encrypted content |
| `decisionInstructions` | body | string | optional, default `""`; trimmed, then at most 4000 characters and no NUL; added once to the end of the decision model's input |
| `chatInstructions` | body | string | optional, default `""`; trimmed, then at most 4000 characters and no NUL; added to every LLM check's prompt after its rules, which win where they conflict |

```json
{
  "decisionProviderId": 1,
  "decisionModel": "gpt-6-luna",
  "chatProviderId": 2,
  "chatModel": "gpt-5",
  "chatInstructions": "Prefer curl over the browser."
}
```

**Output**

| Status | Body |
| --- | --- |
| `200` | automation settings, as [Show automation](#show-automation) answers them |
| `400` | `invalid_provider`, `invalid_model`, `invalid_instructions`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

## Turn off automation

Turns automation off: it forgets the owner, models, instructions and webhook secret, and the project's pull request records, and cancels its queued and running test runs and workflow executions. A running execution stays `running` until the runner has removed it, within a few seconds. A finished run whose issue comment is not posted yet gets none. Test runs and their results stay. Already off is not an error.

`DELETE /api/projects/{id}/automation`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

## Show workflows

Returns the repository's [workflows file](#workflows-file), `.gantt/workflows.yml`, on its default branch, and the workflows a test run would use from it. A run reads the file at the commit it tests, so a pull request can change its own workflows.

`GET /api/projects/{id}/workflows`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |

**Output**

| Status | Body |
| --- | --- |
| `200` | the workflows file |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

`content` is the file's text, `""` without a file. `sha` is its blob's sha, for [Save workflows](#save-workflows), `null` without a file. `workflows` are each workflow's `name` and `description`: the file's, or without a file the one `default` workflow, with `description` `""`, which builds the `Dockerfile` at the repository's root. `error` says why the file does not parse, with `workflows` empty, else `null`.

```json
{
  "content": "workflows:\n  frontend:\n    description: React UI\n    context: src/front\n",
  "sha": "3d21ec53a331a6f037a91c368710b99387d012c1",
  "workflows": [{ "name": "frontend", "description": "React UI" }],
  "error": null
}
```

## Save workflows

Commits a workflows file that parses to a new branch off the default branch, `chore/gantt-workflows-{unix time}-{8 random hex digits}`, and opens a pull request of it into the default branch, titled `Update .gantt/workflows.yml`, with the caller's GitHub credential. The default branch keeps its file until the pull request merges. A save that fails once the branch is made deletes the branch again.

`PUT /api/projects/{id}/workflows`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `content` | body | string | a [workflows file](#workflows-file) that parses, at most 64 KiB |
| `sha` | body | string or null | the `sha` [Show workflows](#show-workflows) answered; `null` to create the file |

```json
{
  "content": "workflows:\n  frontend:\n    description: React UI\n    context: src/front\n",
  "sha": "3d21ec53a331a6f037a91c368710b99387d012c1"
}
```

**Output**

| Status | Body |
| --- | --- |
| `200` | `branch`, and `pullRequest` with its `number` and `url` |
| `400` | `invalid_workflows`, whose message names the field that breaks a rule and its line, such as `workflows.frontend.port: must be a port from 1 to 65535 (line 5)`; `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `workflows_changed`: the file changed on GitHub since it was loaded; nothing was saved |
| `502` | `github_error` |

```json
{
  "branch": "chore/gantt-workflows-1794137400-9f86d081",
  "pullRequest": { "number": 16, "url": "https://github.com/ada/gantt/pull/16" }
}
```

## Generate webhook secret

Replaces the secret GitHub signs the project's webhook deliveries with, and returns it this once with the payload URL to enter on GitHub, for `pull_request` events with content type `application/json`.

`POST /api/projects/{id}/automation/webhook`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |

**Output**

| Status | Body |
| --- | --- |
| `200` | the secret, 64 hex characters, and the payload URL |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `automation_off` |
| `502` | `github_error` |

```json
{
  "secret": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "payloadUrl": "https://gantt.example.com/api/github/webhook/1"
}
```

## Receive webhook

Takes a GitHub delivery from the project's repository webhook. It needs no session or `Origin`: the signature proves the delivery came from GitHub. For a `pull_request` event it re-reads that pull request and its closing references from GitHub, with the automation owner's credential, and acts on them in the background, as the poller would on its next read. Any other event, such as `ping`, is accepted and changes nothing.

`POST /api/github/webhook/{projectID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `projectID` | path | integer | a project's `id` with automation on and a webhook secret generated |
| `X-Hub-Signature-256` | header | string | `sha256=` and the hex HMAC-SHA256 of the raw body, keyed with the project's webhook secret |
| `X-GitHub-Event` | header | string | only `pull_request` is acted on |
| `number` | body | integer | for a `pull_request` event, the pull request's number; the body is at most 5 MiB |

**Output**

| Status | Body |
| --- | --- |
| `202` | empty |
| `400` | `bad_request` |
| `401` | `invalid_signature`, also for a project without a webhook secret or with automation off |

What a pull request's change does, whether the webhook or the poller sees it first, and only once if both do:

| Change | Effect |
| --- | --- |
| First seen ready, or draft to ready | Each task it closes moves to the end of Testing, unless it is there already, and gets a new test run, which cancels the task's earlier queued or running one. A task in Closed, or one a member [took over](#take-over-task), is never moved and gets no run. |
| Ready to draft | Its queued or running test runs are cancelled. |
| New head commit while ready | Nothing. Approving catches it. |
| Closed, merged, or gone from the open list | Its record is deleted. |

Only a pull request from a branch of the repository, by an author with write access or above, runs. Any other that would start a run gets one comment saying why it was not tested, once while it stays open, and nothing else happens. A run started while more than one open, ready pull request that may run closes the issue ends at once as a runner error, and the task needs attention. What a run checks before its tests run is in [Routing](#routing).

## List test runs

Returns a task's test runs, newest first, also after automation is turned off.

`GET /api/projects/{id}/tasks/{taskID}/runs`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |
| `taskID` | path | integer | a task's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | the task's test runs, empty when it has none |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

`status` is `queued`, `running`, `passed`, `failed`, `error` (a runner error) or `cancelled`. `prNumber` and `headSha` are the pull request and commit tested, `null` when no one linked pull request was found. `reason` is a runner error or cancel reason for the whole run, else `null`. `finishedAt` is `null` until the run ends. `approvedAt` is when a member [approved](#approve-and-merge) the run and its pull request merged, else `null`, and `approvedBy` is their GitHub login, `null` until then or once they unlink GitHub.

```json
{
  "runs": [
    {
      "id": 4,
      "prNumber": 12,
      "headSha": "9fceb02d0ae598e95dc970b74767f19372d61af8",
      "status": "passed",
      "reason": null,
      "createdAt": "2026-11-08T09:30:00Z",
      "finishedAt": "2026-11-08T09:34:12Z",
      "approvedBy": "ada",
      "approvedAt": "2026-11-08T10:02:31Z"
    },
    {
      "id": 3,
      "prNumber": 12,
      "headSha": "e83c5163316f89bfbde7d9ab23ca2e25604af290",
      "status": "cancelled",
      "reason": "Cancelled by ada.",
      "createdAt": "2026-11-08T09:12:00Z",
      "finishedAt": "2026-11-08T09:13:40Z",
      "approvedBy": null,
      "approvedAt": null
    }
  ]
}
```

## Show test run

Returns one test run with its workflow executions and its test results, also after automation is turned off.

`GET /api/projects/{id}/runs/{runID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |
| `runID` | path | integer | a test run's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | the run, as [List test runs](#list-test-runs) shows it, with `executions` and `results` |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

Each execution is one workflow running its tests. Its `status` is `queued` or `running`, then `passed`, `failed` or `error` from its own results, as a run's [outcome](#outcomes) is, or `cancelled`. `attempts` counts the times the queue started it, as [Queue and restarts](#queue-and-restarts) says. `logTail` is the last 64 KB of its log: build and start output, then each script's command and output. `startedAt` is `null` while it is queued, and `finishedAt` until it ends.

Each result is one test, by its `position` in the task's tests. `text` is its line, without the bullet, checkbox and pin, and any block under it, as in the issue's [Tests section](#tests-section). `kind` is `script` or `llm`, an LLM check. `pinned` says it had a `[name]` pin, even one that names no workflow. `executionId` and `workflow` are `null` when it went to no workflow. `probabilities` are the decision model's answer for the test, by workflow name, also when the top one was too low to run it. They are empty when the model was not asked, as for a pinned test or a repository with one workflow, or answered none. `status` is `pending`, `passed`, `failed`, `manual` (for the approver to check by hand), `error` (a runner error) or `skipped` (cancelled). `detail` is a script's exit code and output, an LLM check's reason, or the runner error. `transcript` is an LLM check's tool calls with their answers, cut to 64 KB, else `null`. `hasScreenshot` says whether [Show screenshot](#show-screenshot) has one, and `hasVideo` whether [Show video](#show-video) has one. The `manual` results of a task's latest run are the approver's checklist: [Approve and merge](#approve-and-merge) takes their `id`s.

```json
{
  "id": 4,
  "prNumber": 12,
  "headSha": "9fceb02d0ae598e95dc970b74767f19372d61af8",
  "status": "passed",
  "reason": null,
  "createdAt": "2026-11-08T09:30:00Z",
  "finishedAt": "2026-11-08T09:34:12Z",
  "approvedBy": null,
  "approvedAt": null,
  "executions": [
    {
      "id": 7,
      "workflow": "frontend",
      "status": "passed",
      "attempts": 1,
      "logTail": "#1 [internal] load build definition from Dockerfile\n…\n$ npm test\n12 passed\n",
      "startedAt": "2026-11-08T09:30:02Z",
      "finishedAt": "2026-11-08T09:34:10Z"
    }
  ],
  "results": [
    {
      "id": 21,
      "position": 1,
      "text": "`npm test`",
      "kind": "script",
      "pinned": false,
      "executionId": 7,
      "workflow": "frontend",
      "probabilities": { "backend": 0.1, "frontend": 0.9 },
      "status": "passed",
      "detail": "Exit code 0.\n12 passed\n",
      "transcript": null,
      "hasScreenshot": false,
      "hasVideo": false
    },
    {
      "id": 22,
      "position": 2,
      "text": "The sign-up page refuses an empty email",
      "kind": "llm",
      "pinned": false,
      "executionId": 7,
      "workflow": "frontend",
      "probabilities": { "backend": 0.2, "frontend": 0.8 },
      "status": "passed",
      "detail": "Submitting the form with no email showed \"Enter an email\" and sent nothing.",
      "transcript": [
        { "tool": "browser_navigate", "arguments": "{\"url\": \"http://app:80/signup\"}", "answer": "Page title: Sign up\n…" },
        { "tool": "report", "arguments": "{\"outcome\": \"pass\", \"reason\": \"…\"}", "answer": "Reported." }
      ],
      "hasScreenshot": true,
      "hasVideo": true
    }
  ]
}
```

## Show screenshot

Returns the screenshot an LLM check took after its report, when it used the browser, also after automation is turned off.

`GET /api/projects/{id}/runs/{runID}/results/{resultID}/screenshot`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |
| `runID` | path | integer | a test run's `id` in the project |
| `resultID` | path | integer | a result's `id` in that run, with `hasScreenshot` true |

**Output**

| Status | Body |
| --- | --- |
| `200` | the PNG, as `image/png` |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found`, also for a result with no screenshot |
| `502` | `github_error` |

## Show video

Returns an LLM check's recording of the browser, when it used the browser, also after automation is turned off. It starts just before the check's first browser tool call and ends with the check, whether it reported or could not finish, as [LLM checks](#llm-checks) says. A check that a cancel, the timeout or the runner failing ended has none, and so does one whose recording the browser or the runner failed to make or hand over: that changes nothing else about its result. Range requests are answered, so a player can seek.

`GET /api/projects/{id}/runs/{runID}/results/{resultID}/video`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |
| `runID` | path | integer | a test run's `id` in the project |
| `resultID` | path | integer | a result's `id` in that run, with `hasVideo` true |
| `Range` | header | string | optional, such as `bytes=0-` |

**Output**

| Status | Body |
| --- | --- |
| `200` | the video, as `video/webm`, at most 64 MiB |
| `206` | the range asked for |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found`, also for a result with no video |
| `416` | plain text: the range is outside the video |
| `502` | `github_error` |

A finished run's issue comment links each test's video and screenshot, when it has them, after its line. GitHub's API cannot upload attachments, so the comment links to them rather than embedding them, and opening a link needs an account on the app with read access to the project, as the comment's link to the run does.

The browser makes the recording in its own container, which the PR's pages reach, so code under test can alter it. The check's `transcript` is the authoritative record of what the check did.

## Cancel test run

Cancels a queued or running test run and its workflow executions, and skips its unfinished tests. A running execution stays `running` until it has stopped and the runner has removed it, within a few seconds, so nothing starts beside it meanwhile. The reason names the caller. The task stays where it is and can be moved out of Testing again. A cancelled run gets no issue comment.

`POST /api/projects/{id}/runs/{runID}/cancel`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `runID` | path | integer | a test run's `id` in the project, `queued` or `running` |

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `run_finished` |
| `502` | `github_error` |

## Approve and merge

Merges a task's linked pull request once the caller has checked its manual tests by hand, then moves the task to Closed. The task must be in User Check with a latest test run that `passed`. The merge and the move use the automation owner's GitHub credential, so GitHub shows them as the owner's, and the run records the caller as its approver. The merge method is the first of squash, merge commit and rebase that the repository allows, and the tested commit goes with it as `sha`, so GitHub merges nothing newer. Whether the branch is deleted follows the repository's "automatically delete head branches" setting.

`POST /api/projects/{id}/tasks/{taskID}/approve`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` with automation on; write access |
| `taskID` | path | integer | a task's `id` in the project |
| `checked` | body | integer array | exactly the `id`s of the latest run's `manual` results, in any order; empty or `null` when it has none |

```json
{ "checked": [22] }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the approved test run, as [List test runs](#list-test-runs) shows it |
| `400` | `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `not_tested`, `taken_over`, `run_in_progress`, `head_moved`, `not_mergeable`, `pull_request_closed`, `automation_paused`, `automation_off` |
| `422` | `unchecked_manual_tests` |
| `502` | `github_error` |

- `not_tested`: the task is not in User Check, or its latest test run did not pass, also when a newer run passed while the approval read GitHub.
- `taken_over`: a member [took the task over](#take-over-task), so automation leaves merging it to them. Nothing merged. [Give it back](#give-back-task) to approve it.
- `run_in_progress`: another task linked to the same pull request has a test run queued or running. Merging closes its issue too, so approve once that run ends. Nothing merged.
- `unchecked_manual_tests`: `checked` misses a manual test or holds an id that is none of the run's. The message names the tests not ticked and the ids that are not the run's.
- `head_moved`: the pull request has commits newer than the one tested, found before merging or by GitHub refusing the merge. Nothing merged: the task moved back to Testing and a new test run of the new head is queued. When the pull request is a draft again, or its author no longer has write access, that run ends at once as a runner error saying so, as the triggers would refuse it.
- `not_mergeable`: GitHub will not merge the pull request as it is. Before merging, the API reads GitHub's `mergeable_state` and merges only when it is `clean`, `has_hooks` or `unstable`, so branch protection that the owner may bypass, such as an admin, still holds for the approver: `blocked` (a required review or check), `behind`, `dirty` (conflicts), `draft` and `unknown` (GitHub has not worked it out yet; try again) each answer with their own message. When GitHub then refuses the merge itself, the message is GitHub's. The task stays in User Check.
- `pull_request_closed`: the pull request is closed or merged already, also by another approval at the same moment. Of two approvals at once, one merges.
- `automation_paused`, `automation_off`: automation is paused, also when GitHub just refused the owner's credential, or off, also when it was turned off while the approval read GitHub. Nothing merged.
- `403` and `502`: GitHub refused or failed a call with the owner's credential, answered as on other routes, or the owner's OAuth token has expired and could not be refreshed, which answers `502`. A `401` for the owner's token pauses automation, unless the owner's credential has since become another token, as by another request's refresh, or another member took over: that answers `502`, never `github_not_linked`, which only the approver's own credential answers. Nothing merged. GitHub refusing the merge with `409` while a new read still shows the tested head also answers `502`: try again.

A merge stands even when GitHub then fails the move to Closed: the answer is still `200`, and the task stays in User Check until the issue sync sees its issue closed. So does a merge whose answer got lost, as on a timeout: the API reads the pull request again, and when it merged at the tested head, the approval is saved as usual.

```json
{
  "id": 4,
  "prNumber": 12,
  "headSha": "9fceb02d0ae598e95dc970b74767f19372d61af8",
  "status": "passed",
  "reason": null,
  "createdAt": "2026-11-08T09:30:00Z",
  "finishedAt": "2026-11-08T09:34:12Z",
  "approvedBy": "ada",
  "approvedAt": "2026-11-08T10:02:31Z"
}
```

## Retest task

Starts a new test run of a task's linked pull request at its head, as a move into Testing does, cancelling the task's queued or running one. The card stays in its column: only a run of a task in Testing moves it.

`POST /api/projects/{id}/tasks/{taskID}/retest`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `taskID` | path | integer | a task's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | the task, as [Get task](board.md#get-task) answers it |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `automation_off`, `taken_over` |
| `502` | `github_error` |

With no linked pull request, or more than one, the run ends at once as a runner error saying so, as from a move. Only a task in Testing then needs attention.

## Request changes

Sends a task in User Check back to its author: it moves the task to the end of In Progress, turns the task's linked pull request back into a draft, as a failed test run does, so marking it ready for review again starts a new test run, and last posts the caller's feedback on the pull request, all with the caller's GitHub credential. The comment starts with `Changes requested by {login}:`. The linked pull request is, of the open ones in the project's repository closing the task's issue, the one its latest test run tested, else one ready for review, else a draft, each the first GitHub lists. It answers the task as [Get task](board.md#get-task) does.

`POST /api/projects/{id}/tasks/{taskID}/request-changes`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `taskID` | path | integer | a task's `id` in the project, in User Check |
| `feedback` | body | string | trimmed, 1 to 10000 characters |

```json
{ "feedback": "The bars overlap on narrow screens." }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | task, as [Get task](board.md#get-task) answers it |
| `400` | `invalid_feedback`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `not_in_user_check`, `no_pull_request` when no open pull request closes the issue |
| `502` | `github_error` |

When GitHub fails the move, nothing else is written and the task stays in User Check, so a retry comments once. Once the task has moved, it stays in In Progress whatever GitHub does next, and a failure answers `502` `github_error` with a message saying what to do by hand: post the feedback when the comment failed, or mark the pull request draft and then ready for review when it could not be made a draft, since new commits on a ready pull request start no test run.

## Take over task

Takes a task from automation for the caller to finish by hand. It cancels the task's queued and running test runs, as [Turn off automation](#turn-off-automation) cancels them, with the reason `Taken over by {login}.`, and assigns the caller to its issue on GitHub. While it is taken over, a pull request that becomes ready neither moves it nor starts a run for it, and [Move task](board.md#move-task) moves it freely: no `run_in_progress` or `not_tested`, and no run on entering Testing. A task taken over already, by anyone, is answered as it is. It answers the task as [Get task](board.md#get-task) does, with `takenOverBy` the caller.

`POST /api/projects/{id}/tasks/{taskID}/takeover`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `taskID` | path | integer | a task's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | task, as [Get task](board.md#get-task) answers it |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

```json
{
  "id": 7,
  "issueNumber": 12,
  "columnId": 3,
  "position": 1,
  "version": 2,
  "title": "Draw the timeline",
  "state": "open",
  "url": "https://github.com/ada/gantt/issues/12",
  "runStatus": "cancelled",
  "needsAttention": null,
  "assignees": [{ "login": "ada", "avatarUrl": "https://avatars.githubusercontent.com/u/1?v=4" }],
  "branch": "feature/12-draw-the-timeline",
  "body": "One bar per task, by week.",
  "labels": [{ "name": "board:testing", "color": "fbca04" }],
  "tests": [],
  "testsError": null,
  "pullRequests": [
    { "number": 15, "title": "Draw the timeline", "state": "open", "draft": false, "url": "https://github.com/ada/gantt/pull/15" }
  ],
  "pullRequestsError": null,
  "takenOverBy": { "id": 1, "login": "ada" }
}
```

## Give back task

Hands a task taken over back to automation: moves follow the automation rules again and pull requests start runs for it. It starts no test run by itself and leaves the issue's assignees as they are. A task not taken over is answered as it is. It answers the task as [Get task](board.md#get-task) does, with `takenOverBy` `null`.

`DELETE /api/projects/{id}/tasks/{taskID}/takeover`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `taskID` | path | integer | a task's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | task, as [Get task](board.md#get-task) answers it |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

## Workflows file

A repository's workflows live in `.gantt/workflows.yml`. A test run reads the file at the commit it tests. Without a file at that commit, the repository has one workflow, `default`, with every field at its default: the `Dockerfile` at the repository's root, its last stage, no sidecars and no `port`. A file that does not parse ends the run at once as a runner error naming the field. A build that finds no Dockerfile is a runner error for the workflow's tests. [Show workflows](#show-workflows) and [Save workflows](#save-workflows) read and change the file on the default branch.

For example, a repository with a Go API and a React frontend that LLM checks see in a browser, against the real API:

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

The file holds one key, `workflows`, a mapping of at least one workflow by name. Workflow fields:

| Field | Rule |
| --- | --- |
| name (the key) | 2 to 32 lowercase letters, digits or `-`, starting with a letter and ending with a letter or digit |
| `description` | required, not blank; the decision model reads it |
| `context` | the build context, a relative path inside the repository; default `.` |
| `dockerfile` | relative to `context`, and inside the repository; default `Dockerfile` |
| `target` | the build stage; default the last stage |
| `env` | a string map, set in every container from the test image: the app, each script and each `run_command`. Numbers and booleans count as their text, and null as `""`. A key may not be empty or hold `=`. The values are in the repository, so they are not secret. |
| `cache` | at most 4 absolute, clean paths of letters, digits and `._@+-`, other than `/` and `/tmp`, none twice. Each is a volume in every container from the test image that keeps its contents between executions of this workflow in this project, such as a compiler cache. |
| `port` | 1 to 65535. When set, the test image's default command runs as host `app` throughout the execution, ready once it accepts on this port, and LLM checks get the browser. Default none. |
| `services` | sidecars, below; at most 8 |
| `timeout` | a Go duration above 0 and at most `2h`, such as `20m`; default `30m`. It bounds the whole execution, builds included. |

Sidecar fields. A sidecar has either `image`, or a build from `context`, `dockerfile` and `target`, with the workflow fields' rules and defaults:

| Field | Rule |
| --- | --- |
| name (the key) | as for workflows, and its host name on the execution's network; `app` and `browser` are reserved |
| `image` | a registry image reference, pulled at build time; not blank and not starting with `-` |
| `env` | a string map, as for workflows, set in the sidecar |
| `health` | a command run in the sidecar with `sh -c` until it exits 0; needs a shell in the image |
| `port` | 1 to 65535, a TCP port the runner dials until it accepts; for an image with no shell |

Sidecars start in file order, each healthy before the next starts, and the app starts last. A sidecar is healthy once its `health` passes and its `port` accepts, each when set, and with neither once it runs. A sidecar not healthy, or an app not accepting, within 2 minutes is a runner error.

The file is at most 64 KiB. Unknown fields, repeated keys and YAML aliases are refused. Of the names `{workflow}`, one per workflow, and `{workflow}-{sidecar}`, one per built sidecar, no two may be the same: a workflow `back-end` beside a workflow `back` with a built sidecar `end` is refused.

## Tests section

A task's tests live in its issue body as a checklist under a `## Tests` heading. [Set tests](board.md#set-tests) writes the section, and every read of the issue takes it, as [Get task](board.md#get-task) says. A test run takes it again before its tests run. For example:

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

The first test is a script whose command is its code span. The second is a script pinned to `backend`, whose command is its code block. The other three are checks, and the fourth has its steps as `details`.

- **Section.** It starts at the first line that reads `## Tests`, in any case, with optional trailing spaces, and ends at the next level-1 or level-2 heading or the end of the body. A line inside a code fence that opens at the start of a line neither starts nor ends it.
- **Items.** A line that starts with `- ` or `* `, not indented, starts a test. The indented and blank lines after it, up to the next such line, are its block, with their common indent removed. Other lines are ignored.
- **Checkbox.** An optional `[ ]`, `[x]` or `[X]` after the bullet is dropped. A tick changes nothing.
- **Pin.** Then an optional `[name]`, where name follows the workflow name rule and whitespace or the line's end follows it, pins the test to that workflow and is dropped. So `[docs](https://…)` stays text.
- **Name.** The rest of the line, trimmed, is the test's `name`.
- **Kind.** A test is a script when its block is exactly one fenced code block, tagged `sh`, `bash` or `shell` or untagged, with a command that is not blank: the command is the block's contents. It is also a script when its name is exactly one inline code span and it has no block: the command, and the name, is the span's contents. A script passes when its command exits 0. Any other test is a check, which an LLM carries out from its name and block. In a run's results, a check's `kind` is `llm`.
- **Limit.** A section with more than 64 tests does not parse. A section with no items holds no tests.

## Routing

A queued test run is planned before any of its tests run. The API reads the issue from GitHub with the owner's credential and takes its Tests section, unless the card's tests changed while it read. The run ends at once as a runner error when the issue's author has no write access to the repository: its tests would run with the workflow's caches, and only the author and members with write access can edit them. It also ends so when the task has no tests, its section does not parse (its `testsError`), or the [workflows file](#workflows-file) at the run's head does not parse. GitHub refusing a read for good, such as for an issue deleted meanwhile, ends it so too. GitHub failing for now leaves the run queued, and GitHub refusing the owner's credential pauses automation.

Then each test needs one workflow:

1. A pinned test goes to its pin. A pin that names no workflow in the file is a runner error for that test.
2. When the repository has one workflow, as without a file, every other test goes to it.
3. Otherwise the API asks the owner's decision model about every other test in one call, `POST {baseUrl}/decisions`, with the issue's title, each workflow's name and description, the tests' text, and the owner's `decisionInstructions`. A test goes to its most likely workflow when that one is at least 0.5 likely. A lower top probability, a refusal or a missing answer is a runner error for that test that names its best guesses, up to 3, so its author can pin it. The call failing ends the whole run as a runner error.

The run gets one workflow execution per workflow that has tests. A run none of whose tests went to a workflow ends at once, as a runner error.

## Running tests

The queue worker carries out each workflow execution within its workflow's `timeout`, counted from when it starts:

1. **Prepare.** The API downloads the source at the run's head commit from GitHub, with the owner's credential, and hands it to the runner. The runner builds the test image and each built sidecar, pulls each image sidecar, makes the execution's network, which does not reach the internet, and starts the sidecars and then the app, as the [workflows file](#workflows-file) says. Its output starts the execution's log.
2. **Scripts.** Each runs `sh -c` with its command in a fresh container from the test image, on the execution's network, so it reaches the app and the sidecars by name. Exit 0 passes, else it fails. Its `detail` is the exit code and the last 64 KB of its output. The test image needs `sh`.
3. **LLM checks.** One at a time, as [LLM checks](#llm-checks) says.
4. **Clean up.** The runner removes the execution's containers and network, whatever happened. The images and cache volumes stay.

Scripts and LLM checks each run in issue order. Every container gets at most 4 GiB of memory, 2 CPUs and 1024 processes, with `/tmp` as a tmpfs.

A build that fails, an app or sidecar that does not start, and the timeout passing are each a runner error for every test of the execution not finished yet. The runner refusing one script's command is a runner error for that test alone. [Cancel test run](#cancel-test-run) skips the unfinished tests.

## LLM checks

A check runs as a tool-calling loop over the owner's chat model, `POST {baseUrl}/responses`, which stores nothing at the provider. Its first message holds the issue's title and body, as JSON marked as data and never instructions, the check's text, the workflow's name, the app's URL `http://app:{port}` or none, the sidecars' names, the rule to verify with the tools and then call `report` once, and last the owner's `chatInstructions`, which that rule wins over. The API reads the issue from GitHub before an execution's first check, with the owner's credential read again then, since the build and the scripts may have outlasted the token the execution started with.

| Tool | Offered | What it does |
| --- | --- | --- |
| `run_command` | always | runs `sh -c` with the command in a fresh container from the test image, on the execution's network; files do not carry over from one call to the next. It answers the exit code and the last 64 KB of output. |
| `http_request` | when the workflow has a `port` or a sidecar | makes one `http` request to `app` or a sidecar by name, following no redirects. It answers the status, headers and body, the body cut at 64 KB. |
| the browser's tools | when the workflow has a `port` | those the Playwright MCP browser lists, without its DevTools tools. The browser starts before the execution's first check, and its checks share it. |
| `report` | always | takes `outcome`, `pass`, `fail` or `unverifiable`, and `reason`, and ends the check |

A call the model gets wrong, or one the runner or the browser refuses, is answered with why, so the model can try again. The runner failing stops the whole execution.

| The check | Result |
| --- | --- |
| reports `pass` | `passed` |
| reports `fail` | `failed` |
| reports `unverifiable`, for what no tool can check, saying what a person should check | `manual`, a manual test for the approver |
| makes 50 tool calls without a report, answers twice without calling a tool, or the chat API fails | `error`, a runner error for this check alone |
| finds the browser not starting, or its MCP session not opening | `error`, for this check alone |

The result's `detail` is the report's `reason`, or why the check could not finish. Its `transcript` is each tool call, `report` included, as `tool`, `arguments` and `answer`, cut to 64 KB in all. A check that used the browser keeps a PNG screenshot of the page, taken after its report, for [Show screenshot](#show-screenshot), and a WebM recording, as [Show video](#show-video) says.

## Queue and restarts

Workflow executions queue across the install, and the worker takes them every 2 seconds: in `id` order, while fewer than `MAX_CONCURRENT_WORKFLOWS` run, by default 1, never two at once of the same project and workflow, and never one of a paused project. Each start counts in its `attempts`. The API shutting down, the runner out of reach, GitHub failing for now, or GitHub refusing the owner's credential, which pauses automation, puts the execution back on the queue, its unfinished tests still `pending`, and takes the attempt back.

When the API starts, it has the runner remove every execution, then puts back on the queue those that were running. So does the runner restarting under an execution, once: an execution interrupted a second time ends, its unfinished tests a runner error, `The runner was interrupted twice.`

## Outcomes

A test run ends once every workflow execution of it has. Its status comes from its results, first match wins:

| Results include | Run status | Task, if still in Testing | Linked pull request |
| --- | --- | --- | --- |
| any `failed` | `failed` | moves to the end of In Progress | turned back into a draft, wherever the task is |
| any other result not `passed` or `manual` | `error` | stays, with `needsAttention` the first such result's `detail` | unchanged |
| only `passed` and `manual` | `passed` | moves to the end of User Check | unchanged |

A run that ends at once as a runner error is `error` too, with `needsAttention` its `reason`. A test failure outranks a runner error, because it already says something about the code. The moves are the API's own, with the owner's credential, so [Moves while automated](#moves-while-automated) refuses none of them. A task moved out of Testing meanwhile stays where it is.

Every finished run but a cancelled one posts one comment on its issue, with the owner's credential: whether the tests passed, failed or could not finish, on which commit; a line per test with its result, its text's first line, its workflow and links to its video and screenshot; the run's `reason`; the manual tests to check by hand before approving; and a link to the run, `{PUBLIC_URL}/projects/{id}/tasks/{taskID}?run={runID}`. A post GitHub refuses for good, such as on a locked issue, is given up.

## Moves while automated

While the project's automation is on, [Move task](board.md#move-task) follows these rules for a move to another column, unless a member [took the task over](#take-over-task): then it moves freely and starts no test run. A reorder within a column is not such a move.

| Move | Rule |
| --- | --- |
| Out of Testing | Refused with `409 run_in_progress` while the task's latest test run is `queued` or `running`. [Cancel test run](#cancel-test-run) unlocks it. Otherwise it clears `needsAttention`. |
| Into User Check | Refused with `409 not_tested` unless the task's latest test run `passed` on the head commit its pull request has, as the API last read it from GitHub. |
| Into Testing | Starts a test run, which cancels the task's earlier queued or running one, of the one open, ready pull request that closes the issue and may run, at its head: from a branch of the repository, by an author with write access as GitHub says now, and not refused with a comment. With none, or more than one, the run ends at once as a runner error, and the task needs attention. |
| Any other | Free, Closed included. |

The refusals and the run hold for that endpoint only. The API's own moves, such as for a pull request becoming ready, a run's outcome, an approval or a change request, are never refused. A sync follows GitHub: a board label added there, or the issue closed, moves the task whatever its test run, and approving checks the run again. Every move out of Testing, the API's and a sync's included, clears `needsAttention`.
