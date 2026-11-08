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

`owner` is the automation owner's user `id` and GitHub `login`, `null` once they unlink GitHub. `pausedReason` is `null` while automation runs. `webhookSecretSet` says whether a webhook secret was generated.

```json
{
  "enabled": true,
  "owner": { "id": 1, "login": "ada" },
  "decisionProviderId": 1,
  "decisionModel": "gpt-6-luna",
  "chatProviderId": 2,
  "chatModel": "gpt-5",
  "pausedReason": null,
  "payloadUrl": "https://gantt.example.com/api/github/webhook/1",
  "webhookSecretSet": false
}
```

```json
{ "enabled": false }
```

## Turn on automation

Turns automation on with the caller as its owner, or takes it over: the caller replaces the owner and models and unpauses it, and the webhook secret stays.

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

```json
{ "decisionProviderId": 1, "decisionModel": "gpt-6-luna", "chatProviderId": 2, "chatModel": "gpt-5" }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | automation settings, as [Show automation](#show-automation) answers them |
| `400` | `invalid_provider`, `invalid_model`, `bad_request` |
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

Only a pull request from a branch of the repository, by an author with write access or above, runs. Any other that would start a run gets one comment saying why it was not tested, once while it stays open, and nothing else happens. A run started while more than one open, ready pull request that may run closes the issue ends at once as a runner error, and the task needs attention. So does a run whose issue was opened by someone without write access: its Tests section would run with the workflow's caches.

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

`status` is `queued`, `running`, `passed`, `failed`, `error` (a runner error) or `cancelled`. `prNumber` and `headSha` are the pull request and commit tested, `null` when no one linked pull request was found. `reason` is a runner error or cancel reason for the whole run, else `null`. `finishedAt` is `null` until the run ends.

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
      "finishedAt": "2026-11-08T09:34:12Z"
    },
    {
      "id": 3,
      "prNumber": 12,
      "headSha": "e83c5163316f89bfbde7d9ab23ca2e25604af290",
      "status": "cancelled",
      "reason": "Cancelled by ada.",
      "createdAt": "2026-11-08T09:12:00Z",
      "finishedAt": "2026-11-08T09:13:40Z"
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

Each result is one test, by its `position` in the Tests section. `text` is its line and block. `kind` is `script` or `llm`. `pinned` says it named its workflow. `executionId` and `workflow` are `null` when it went to no workflow. `probabilities` are the decision model's, by workflow name, empty when it made no pick. `status` is `pending`, `passed`, `failed`, `manual` (for the approver to check by hand), `error` (a runner error) or `skipped` (cancelled). `detail` is a script's exit code and output, an LLM check's reason, or the runner error. `transcript` is an LLM check's tool calls with their answers, cut to 64 KB, else `null`. `hasScreenshot` says whether [Show screenshot](#show-screenshot) has one.

```json
{
  "id": 4,
  "prNumber": 12,
  "headSha": "9fceb02d0ae598e95dc970b74767f19372d61af8",
  "status": "passed",
  "reason": null,
  "createdAt": "2026-11-08T09:30:00Z",
  "finishedAt": "2026-11-08T09:34:12Z",
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
      "hasScreenshot": false
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
      "hasScreenshot": true
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
