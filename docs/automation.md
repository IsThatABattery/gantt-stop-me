# Automation

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

`owner` is the automation owner's user `id` and GitHub `login`, `null` once they unlink GitHub. `decisionInstructions` and `chatInstructions` are the owner's own instructions to the decision model and to LLM checks, `""` for none. `pausedReason` is `null` while automation runs. `webhookSecretSet` says whether a webhook secret was generated.

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

Turns automation on with the caller as its owner, or takes it over: the caller replaces the owner, models and instructions and unpauses it, and the webhook secret stays.

`PUT /api/projects/{id}/automation`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `decisionProviderId` | body | integer | one of the caller's providers |
| `decisionModel` | body | string | one of that provider's chosen `models`; picks workflows, so it should serve the OpenAI Decisions API |
| `chatProviderId` | body | integer | one of the caller's providers, the same or another |
| `chatModel` | body | string | one of that provider's chosen `models`; runs LLM checks, so it should support tool calling |
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

Turns automation off, forgets the project's pull request records, and cancels its queued and running test runs and workflow executions. A running execution stays `running` until the runner has removed it, within a few seconds. A finished run whose issue comment is not posted yet gets none. Already off is not an error.

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
| First seen ready, or draft to ready | Each task it closes moves to the end of Testing, unless it is there already, and gets a new test run, which cancels the task's earlier queued or running one. A task in Closed is never moved and gets no run. |
| Ready to draft | Its queued or running test runs are cancelled. |
| New head commit while ready | Nothing. |
| Closed, merged, or gone from the open list | Its record is deleted. |

Only a pull request from a branch of the repository, by an author with write access or above, runs. Any other that would start a run gets one comment saying why it was not tested, once while it stays open, and nothing else happens. A run started while more than one open, ready pull request that may run closes the issue ends at once as a runner error, and the task needs attention. So does a run whose issue was opened by someone without write access: its tests, which edits on GitHub can set, would run with the workflow's caches. A run tests the task's [tests](board.md#get-task), after taking its issue's Tests section as GitHub has it then. A task with no tests, or whose section does not parse (its `testsError`), ends the run at once as a runner error too.

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

Each execution is one workflow running its tests. `logTail` is the last 64 KB of its log: build and start output, then each script's command and output. Its `status` is a run's, from its own results.

Each result is one test, by its `position` in the task's tests. `text` is its line and block as the issue's Tests section shows it. `kind` is `script` or `llm`. `pinned` says it named its workflow. `executionId` and `workflow` are `null` when it went to no workflow. `probabilities` are the decision model's, by workflow name, empty when it made no pick. `status` is `pending`, `passed`, `failed`, `manual` (for the approver to check by hand), `error` (a runner error) or `skipped` (cancelled). `detail` is a script's exit code and output, an LLM check's reason, or the runner error. `transcript` is an LLM check's tool calls with their answers, cut to 64 KB, else `null`. `hasScreenshot` says whether [Show screenshot](#show-screenshot) has one, and `hasVideo` whether [Show video](#show-video) has one. The `manual` results of a task's latest run are the approver's checklist: [Approve and merge](#approve-and-merge) takes their `id`s.

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

Returns an LLM check's recording of the browser, when it used the browser, also after automation is turned off. It starts just before the check's first browser tool call and ends with the check, whether it reported, made 50 tool calls without a report, or the chat API failed. A check that a cancel or the timeout ended has none, and so does one whose recording the browser or the runner failed to make or hand over: that changes nothing else about its result. Range requests are answered, so a player can seek.

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
| `409` | `not_tested`, `run_in_progress`, `head_moved`, `not_mergeable`, `pull_request_closed`, `automation_paused`, `automation_off` |
| `422` | `unchecked_manual_tests` |
| `502` | `github_error` |

- `not_tested`: the task is not in User Check, or its latest test run did not pass.
- `run_in_progress`: another task linked to the same pull request has a test run queued or running. Merging closes its issue too, so approve once that run ends. Nothing merged.
- `unchecked_manual_tests`: `checked` misses a manual test or holds an id that is none of the run's. The message names the tests not ticked and the ids that are not the run's.
- `head_moved`: the pull request has commits newer than the one tested, found before merging or by GitHub refusing the merge. Nothing merged: the task moved back to Testing and a new test run of the new head is queued. When the pull request is a draft again, or its author no longer has write access, that run ends at once as a runner error saying so, as the triggers would refuse it.
- `not_mergeable`: GitHub will not merge the pull request as it is. Before merging, the API reads GitHub's `mergeable_state` and merges only when it is `clean`, `has_hooks` or `unstable`, so branch protection that the owner may bypass, such as an admin, still holds for the approver: `blocked` (a required review or check), `behind`, `dirty` (conflicts), `draft` and `unknown` (GitHub has not worked it out yet; try again) each answer with their own message. When GitHub then refuses the merge itself, the message is GitHub's. The task stays in User Check.
- `pull_request_closed`: the pull request is closed or merged already, also by another approval at the same moment. Of two approvals at once, one merges.
- `automation_paused`, `automation_off`: automation is paused, also when GitHub just refused the owner's credential, or off, also when it was turned off while the approval read GitHub. Nothing merged.
- `403` and `502`: GitHub refused or failed a call with the owner's credential, answered as on other routes. Nothing merged. GitHub refusing the merge with `409` while a new read still shows the tested head also answers `502`: try again.

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

## Request changes

Sends a task in User Check back to its author: it posts the caller's feedback on the task's linked pull request with the caller's GitHub credential, moves the task to the end of In Progress, and turns the pull request back into a draft, as a failed test run does, so marking it ready for review again starts a new test run. The comment starts with `Changes requested by {login}:`. The linked pull request is the open one closing the task's issue that its latest test run tested, else the open one GitHub lists first. A failure to make it a draft is logged and changes nothing else. It answers the task as [Get task](board.md#get-task) does.

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

A comment stands even when GitHub then fails the move: the answer is GitHub's error and the task stays in User Check, so a retry comments again.

