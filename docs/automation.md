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

Turns automation off, forgets the project's pull request records, and cancels its queued and running test runs and workflow executions. Already off is not an error.

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

Only a pull request from a branch of the repository, by an author with write access or above, runs. Any other that would start a run gets one comment saying why it was not tested, and nothing else happens. A run started while more than one open, ready pull request closes the issue ends at once as a runner error, and the task needs attention.
