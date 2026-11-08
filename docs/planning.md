# Planning

A member describes a feature, and an interviewer agent asks them about it one question at a time while it writes a plan doc. The member then reviews the doc, unless their settings turn review off. Once the doc is approved, a task agent turns it into proposed tasks with tests, which members review one by one, and each approved one becomes a task in Backlog.

The agent works in the API's planner worker, not in requests, because its LLM calls can outlast a request. A request records what the person did, sets the plan's `pending` job, and answers at once. The worker takes each plan with a job every 2 seconds and does one agent turn with the model and instructions the user who started the plan picked for that agent: the interviewer's for `ask` and `draft`, the task agent's for `generate` and `revise`, one turn at a time for each such user. Load the plan again until `pending` is `null`. A turn that fails sets `error` and clears `pending`, and [Retry](#retry-the-agents-turn) gives the job back. A turn also fails, without calling the model, when the user who started the plan has no model for that agent, their provider key cannot be read, or their GitHub account can no longer see the repository, or, for `generate`, which creates tasks with it, can no longer push to it. While a turn runs on a plan, every change to it answers `409 agent_busy` at once instead of waiting.

A plan's `status` is `briefing` (the interview), `needs_review`, `approved`, `done` or `discarded`. `pending` is the agent's next job: `ask` (update the doc and ask the next question, or finish), `draft` (write the doc from what it has and finish), `generate` (turn the doc into proposed tasks), or `revise` (rewrite the proposal sent back), and `null` while the plan waits on a person.

When the interview ends, because the agent finished, the member asked for the draft, or it asked 12 questions, the agent writes its last doc pass. With the creator's `review` on, the plan goes to `needs_review`. With it off, it goes straight to `approved` with `pending: "generate"`.

On `generate`, the task agent proposes up to 30 tasks, each small enough for one pull request, with a description, acceptance criteria, tests, the earlier proposals it depends on, and the doc section it came from. A proposal the API refuses, such as one with invalid tests, is answered back to the agent with why, so it can fix it. A turn that proposes nothing fails. With the creator's `review` on, the proposals wait in `needs_review` for [Approve a proposal](#approve-a-proposal), [Edit a proposal](#edit-a-proposal) and [Deny a proposal](#deny-a-proposal). With it off, the worker approves every proposal and creates its task at once, in dependency order, with the creator's GitHub token. The proposals are saved before any task is made, and each task with its proposal, so a restart or a failed creation keeps both, and the next `generate` only creates the missing tasks.

A proposal's task is an issue labeled `board:backlog`, at the end of Backlog. Its body is the description, an `## Acceptance criteria` checklist, a `Depends on #N` line for each proposal it depends on that has its task (`Depends on` and that proposal's title while it has none, and no line for a dropped one), and the proposal's tests as a `## Tests` section. The card's [tests](board.md#get-task) are the proposal's. The plan becomes `done` once no job is pending and every proposal not dropped has its task.

## Show planning settings

Returns whether the caller reviews plan docs, and the model and own instructions of each planning agent: the interviewer and the task agent.

`GET /api/me/planning`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `200` | the settings; until saved, `review` is `true` and each agent is `{ "providerId": null, "model": null, "instructions": "" }` |
| `401` | `unauthorized` |

```json
{
  "review": true,
  "interviewer": { "providerId": 2, "model": "gpt-5", "instructions": "" },
  "taskAgent": { "providerId": 2, "model": "gpt-5", "instructions": "Keep every task under a day of work." }
}
```

## Save planning settings

Replaces the caller's planning settings. The agents of every plan the caller starts use them, including plans started before a change.

`PUT /api/me/planning`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `review` | body | boolean | optional, default `true`; `false` approves plan docs without review |
| `interviewer` | body | object | required, so a save cannot wipe it by leaving it out; `{}` has no model and no instructions |
| `taskAgent` | body | object | required, as `interviewer` |
| `….providerId` | body | integer or null | one of the caller's providers; `null` with `model` picks no model, so the caller can delete the provider |
| `….model` | body | string or null | one of that provider's chosen `models`; it should support tool calling |
| `….instructions` | body | string | optional, default `""`; trimmed, then at most 4000 characters and no NUL; added to the agent's prompt after its own rules, which win where they conflict |

```json
{
  "review": true,
  "interviewer": { "providerId": 2, "model": "gpt-5" },
  "taskAgent": { "providerId": 2, "model": "gpt-5", "instructions": "Keep every task under a day of work." }
}
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the settings, as [Show planning settings](#show-planning-settings) answers them |
| `400` | `invalid_provider`, `invalid_model`, `invalid_instructions`, `bad_request`: also when `interviewer` or `taskAgent` is missing |
| `401` | `unauthorized` |

## Start a plan

Starts a plan in `briefing` with `pending: "ask"`, for the agent's first question. Its agent always uses the caller's planning settings.

`POST /api/projects/{id}/plans`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `title` | body | string | 1 to 256 characters, trimmed |
| `description` | body | string | the feature as the caller describes it, 1 to 10000 characters, trimmed |

```json
{ "title": "Board export", "description": "Let admins export the board as CSV." }
```

**Output**

| Status | Body |
| --- | --- |
| `201` | the plan, as [Show plan](#show-plan) answers it |
| `400` | `invalid_title`, `invalid_description`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `planning_not_set_up`: the caller has no interviewer model |
| `502` | `github_error` |

## List plans

Returns the project's plans, newest first, without their messages and proposals.

`GET /api/projects/{id}/plans`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |

**Output**

| Status | Body |
| --- | --- |
| `200` | `plans`, each as [Show plan](#show-plan) answers it without `messages` and `proposals` |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

## Show plan

Returns one plan with its doc, its interview and its proposed tasks.

`GET /api/projects/{id}/plans/{planID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |
| `planID` | path | integer | a plan's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

`doc` is always these sections, in order: `goal` (Goal), `users` (Users and use cases), `scope` (Scope), `out_of_scope` (Out of scope), `requirements` (Requirements), `success` (Success criteria), `risks` (Risks) and `open_questions` (Open questions). A section's `body` is text, or a list of items. Its `state` is `covered`, `asking` (the topic of the question waiting) or `not_yet`.

`messages` is the interview in order: the agent's questions, with `options` (0 to 4 suggested answers), and the person's answers, with `skipped` true for a question skipped instead. `answered` counts the questions answered or skipped. `expectedQuestions` is roughly how many questions the interview takes: those asked, plus one per topic not yet touched, at most 12, and those asked once it is over. `feedback` is a denial's, until the agent's next turn takes it, else `null`. `error` says why the agent's last turn failed, `null` once one succeeds. `version` changes with every change to the plan.

`proposals` are the task agent's proposed tasks, by `position` (from 1, in the order it proposed them), `[]` until it proposes any. A proposal's `status` is `needs_review`, `approved`, `approved_edited` (approved after a person's [edit](#edit-a-proposal)), `sent_back` (for the agent to revise) or `dropped`. `acceptanceCriteria` are one-line texts. `tests` are as a card's [tests](board.md#get-task). `dependsOn` holds the `position`s of the earlier proposals it needs. `source` is the doc section it came from, and `agentNote` what the agent wants a reviewer to know, `""` for none. `feedback` is a denial's: a revision takes it, and a dropped proposal keeps it. `taskId` is its task's `id` once that is created, else `null`. With review off, an `approved` proposal has none until its task is made, as when GitHub refused it.

```json
{
  "id": 4,
  "position": 2,
  "status": "needs_review",
  "title": "Export button",
  "description": "Add an Export button to the board.",
  "acceptanceCriteria": ["The button downloads the CSV"],
  "tests": [{ "name": "npm test", "kind": "script", "command": "npm test", "details": "", "workflow": null }],
  "dependsOn": [1],
  "source": "Scope, item 1",
  "agentNote": "",
  "feedback": null,
  "taskId": null,
  "createdAt": "2026-11-08T10:05:00Z"
}
```

A whole plan, in `briefing`:

```json
{
  "id": 1,
  "projectId": 1,
  "title": "Board export",
  "description": "Let admins export the board as CSV.",
  "status": "briefing",
  "doc": [
    { "key": "goal", "title": "Goal", "body": "Admins export the board as CSV.", "state": "covered" },
    { "key": "users", "title": "Users and use cases", "body": "", "state": "asking" },
    { "key": "scope", "title": "Scope", "body": ["Every column", "Open tasks only"], "state": "covered" }
  ],
  "pending": null,
  "feedback": null,
  "error": null,
  "version": 3,
  "answered": 1,
  "expectedQuestions": 6,
  "createdAt": "2026-11-08T10:00:00Z",
  "updatedAt": "2026-11-08T10:01:12Z",
  "messages": [
    { "id": 1, "role": "agent", "text": "Which columns go in?", "options": ["All", "Open only"], "skipped": false, "createdAt": "2026-11-08T10:00:04Z" },
    { "id": 2, "role": "user", "text": "All", "options": [], "skipped": false, "createdAt": "2026-11-08T10:00:40Z" },
    { "id": 3, "role": "agent", "text": "Who exports?", "options": [], "skipped": false, "createdAt": "2026-11-08T10:01:12Z" }
  ],
  "proposals": []
}
```

(The example shows three of the eight sections.)

## Answer a question

Answers the agent's waiting question, or skips it, and sets `pending: "ask"`.

`POST /api/projects/{id}/plans/{planID}/answer`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |
| `text` | body | string | 1 to 10000 characters, trimmed; ignored with `skip` |
| `skip` | body | boolean | optional; `true` skips the question |

```json
{ "text": "Only admins." }
```

```json
{ "skip": true }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `400` | `invalid_text`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `briefing`, `agent_busy` while `pending` is set, `no_question` when no question waits, after a failed turn |
| `502` | `github_error` |

## Draft the doc now

Ends the interview early with `pending: "draft"`: the agent writes the doc from what it has, and puts the topics it could not cover into open questions.

`POST /api/projects/{id}/plans/{planID}/draft`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `briefing`, `agent_busy` while `pending` is set |
| `502` | `github_error` |

## Edit the doc

Changes doc sections as a person wrote them, in `briefing` or `needs_review`, unless the plan changed since they loaded `version`.

`PATCH /api/projects/{id}/plans/{planID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |
| `doc` | body | array | sections by `key`, in any number; each `body` (text, or a list of texts, at most 10000 characters in all) and `state` is optional and kept when left out; `title` is ignored, so the doc can be sent back as loaded |
| `version` | body | integer | the plan's `version` as loaded |

```json
{ "doc": [{ "key": "goal", "body": "Admins export the board as CSV." }], "version": 3 }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `400` | `invalid_doc`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status`, `agent_busy` while `pending` is set, `conflict` with the plan as it is now |
| `502` | `github_error` |

```json
{ "error": { "code": "conflict", "message": "This plan changed since you loaded it. It is shown as it is now." }, "plan": { "id": 1, "version": 4 } }
```

(`plan` is the whole plan, as [Show plan](#show-plan) answers it.)

## Approve the doc

Accepts the doc of a plan in `needs_review`: it becomes `approved` with `pending: "generate"`, for the task agent.

`POST /api/projects/{id}/plans/{planID}/approve`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `needs_review`, `agent_busy` |
| `502` | `github_error` |

## Deny the doc

Sends the doc of a plan in `needs_review` back to `briefing` with feedback and `pending: "ask"`. The sections the feedback points at go back to `not_yet`. The agent's next turn reads the feedback and writes it into the doc, then asks about those sections, or, once 12 questions are asked, puts them into open questions.

`POST /api/projects/{id}/plans/{planID}/deny`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |
| `feedback` | body | string | 1 to 10000 characters, trimmed |
| `sections` | body | array of strings | optional; section keys |

```json
{ "feedback": "Scope is too wide: start with CSV only.", "sections": ["scope"] }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `400` | `invalid_feedback`, `invalid_sections`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `needs_review`, `agent_busy` |
| `502` | `github_error` |

## Retry the agent's turn

Gives the agent back the job its last turn failed. In `briefing`, that is the draft when a question waits unanswered and there is no feedback, else the next question. In `approved`, it is `revise` while a proposal is `sent_back`, else `generate`, which only creates the missing tasks when the agent's proposals are already there, as after GitHub refused a task with review off. `error` stays until a turn succeeds.

`POST /api/projects/{id}/plans/{planID}/retry`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `briefing` and `approved`, `agent_busy` while `pending` is set, `nothing_to_retry` when the last turn did not fail |
| `502` | `github_error` |

## Edit a proposal

Changes a proposal in `needs_review` of an `approved` plan as a person wrote it, unless the plan changed since they loaded `version`. Approving it then makes it `approved_edited`.

`PATCH /api/projects/{id}/plans/{planID}/proposals/{proposalID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |
| `proposalID` | path | integer | a proposal's `id` in the plan |
| `title` | body | string | optional; 1 to 256 characters on one line, trimmed |
| `description` | body | string | optional; 1 to 10000 characters, trimmed; no `## Tests` heading or unclosed code fence of its own |
| `acceptanceCriteria` | body | array of strings | optional; at least one, each 1 to 10000 characters on one line, trimmed |
| `tests` | body | array | optional; at least one test, as [Set tests](board.md#set-tests) takes them |
| `dependsOn` | body | array of integers | optional; `position`s of earlier proposals |
| `version` | body | integer | the plan's `version` as loaded |

A field left out keeps its value. The issue the proposal would get must fit GitHub's 65536 characters.

```json
{ "title": "CSV button", "tests": [{ "name": "npm test", "kind": "script", "command": "npm test" }], "version": 5 }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `400` | `invalid_proposal`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `approved` or for a proposal not in `needs_review`, `agent_busy` while `pending` is set, `conflict` with the plan as it is now, as [Edit the doc](#edit-the-doc) answers it |
| `502` | `github_error` |

## Approve a proposal

Approves a proposal in `needs_review` of an `approved` plan and creates its task with the caller's GitHub token: an issue labeled `board:backlog`, at the end of Backlog, whose card holds the proposal's tests. The proposal becomes `approved`, or `approved_edited` after an edit, with its `taskId`.

`POST /api/projects/{id}/plans/{planID}/proposals/{proposalID}/approve`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |
| `proposalID` | path | integer | a proposal's `id` in the plan |

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `400` | `invalid_proposal` when the issue would no longer fit GitHub |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `approved` or for a proposal not in `needs_review`, `agent_busy` while `pending` is set |
| `502` | `github_error` |

## Deny a proposal

Turns down a proposal in `needs_review` of an `approved` plan with feedback. `revise` makes it `sent_back` with `pending: "revise"`: the agent's next turn rewrites it from the feedback, in its place, and returns it to `needs_review`. `drop` makes it `dropped`. A `sent_back` proposal whose revision failed can be denied again, with new feedback or to drop it.

`POST /api/projects/{id}/plans/{planID}/proposals/{proposalID}/deny`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |
| `proposalID` | path | integer | a proposal's `id` in the plan |
| `feedback` | body | string | 1 to 10000 characters, trimmed |
| `outcome` | body | string | `revise` or `drop` |

```json
{ "feedback": "Use a menu item, not a button.", "outcome": "revise" }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the plan, as [Show plan](#show-plan) answers it |
| `400` | `invalid_feedback`, `invalid_outcome`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `wrong_status` outside `approved` or for a proposal not in `needs_review` or `sent_back`, `agent_busy` while `pending` is set |
| `502` | `github_error` |

## Discard a plan

Sets the plan `discarded`, in any status, and drops its job. Tasks it created stay. Already discarded is not an error.

`DELETE /api/projects/{id}/plans/{planID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; write access |
| `planID` | path | integer | a plan's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `agent_busy` while the agent's turn runs on it |
| `502` | `github_error` |
