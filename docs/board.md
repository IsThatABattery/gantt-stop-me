# Board

## Add project

Adds a GitHub repository as a project, setting up its board labels and a task per open issue the first time, or returns the project already added.

`POST /api/projects`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `url` | body | string | `https://github.com/{owner}/{name}`, then optionally `.git`, then optionally one `/`; owner 1 to 39 letters, digits or `-`; name 1 to 100 letters, digits, `.`, `_` or `-`, not `.` or `..` |

```json
{ "url": "https://github.com/ada/gantt" }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | project, added before |
| `201` | project, newly added |
| `400` | `invalid_repo_url`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

```json
{ "id": 1, "owner": "ada", "name": "gantt", "permission": "write" }
```

## Show board

Returns the project's columns and tasks, syncing the tasks from the repository's issues first when the last sync is over 30 seconds old.

`GET /api/projects/{id}/board`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` |

**Output**

| Status | Body |
| --- | --- |
| `200` | project, columns and tasks |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

Every task, here and wherever an endpoint answers one, has `runStatus`, its latest test run's status as [List test runs](automation.md#list-test-runs) gives it, `null` when it has none, and `needsAttention`, why it waits in Testing after a runner error, `null` while clear.

```json
{
  "project": { "id": 1, "owner": "ada", "name": "gantt", "permission": "write" },
  "columns": [
    { "id": 1, "key": "backlog", "name": "Backlog", "position": 0, "closesIssue": false },
    { "id": 2, "key": "in-progress", "name": "In Progress", "position": 1, "closesIssue": false },
    { "id": 3, "key": "testing", "name": "Testing", "position": 2, "closesIssue": false },
    { "id": 4, "key": "user-check", "name": "User Check", "position": 3, "closesIssue": false },
    { "id": 5, "key": "closed", "name": "Closed", "position": 4, "closesIssue": true }
  ],
  "tasks": [
    {
      "id": 7,
      "issueNumber": 12,
      "columnId": 1,
      "position": 1,
      "version": 1,
      "title": "Draw the timeline",
      "state": "open",
      "url": "https://github.com/ada/gantt/issues/12",
      "runStatus": null,
      "needsAttention": null
    },
    {
      "id": 9,
      "issueNumber": 14,
      "columnId": 5,
      "position": 1.5,
      "version": 3,
      "title": "Set up CI",
      "state": "closed",
      "url": "https://github.com/ada/gantt/issues/14",
      "runStatus": "passed",
      "needsAttention": null
    }
  ]
}
```

## Create task

Opens an issue labeled `board:backlog` on GitHub and adds its task at the end of Backlog.

`POST /api/projects/{id}/tasks`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` |
| `title` | body | string | trimmed, 1 to 256 characters |
| `body` | body | string | optional; at most 65536 characters |

```json
{ "title": "Draw the timeline", "body": "One bar per task, by week." }
```

**Output**

| Status | Body |
| --- | --- |
| `201` | task |
| `400` | `invalid_title`, `invalid_body`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

```json
{
  "id": 7,
  "issueNumber": 12,
  "columnId": 1,
  "position": 1,
  "version": 1,
  "title": "Draw the timeline",
  "state": "open",
  "url": "https://github.com/ada/gantt/issues/12",
  "runStatus": null,
  "needsAttention": null
}
```

## Get task

Returns a task with its issue's title, state, body and labels as last read from GitHub, syncing the tasks from the repository's issues first when the last sync is over 30 seconds old.

`GET /api/projects/{id}/tasks/{taskID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` |
| `taskID` | path | integer | a task's `id` in the project |

**Output**

| Status | Body |
| --- | --- |
| `200` | task, with its issue's body and labels |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

```json
{
  "id": 7,
  "issueNumber": 12,
  "columnId": 2,
  "position": 1,
  "version": 2,
  "title": "Draw the timeline",
  "state": "open",
  "url": "https://github.com/ada/gantt/issues/12",
  "runStatus": "failed",
  "needsAttention": null,
  "body": "One bar per task, by week.",
  "labels": [
    { "name": "board:in-progress", "color": "fbca04" },
    { "name": "enhancement", "color": "a2eeef" }
  ]
}
```

## Edit task

Changes the title or body of a task's issue on GitHub.

`PATCH /api/projects/{id}/tasks/{taskID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` |
| `taskID` | path | integer | a task's `id` in the project |
| `title` | body | string | trimmed, 1 to 256 characters; optional when `body` is sent |
| `body` | body | string | at most 65536 characters; optional when `title` is sent |

```json
{ "title": "Draw the timeline by week", "body": "One bar per task." }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | task |
| `400` | `invalid_title`, `invalid_body`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `502` | `github_error` |

```json
{
  "id": 7,
  "issueNumber": 12,
  "columnId": 2,
  "position": 1,
  "version": 2,
  "title": "Draw the timeline by week",
  "state": "open",
  "url": "https://github.com/ada/gantt/issues/12",
  "runStatus": null,
  "needsAttention": null
}
```

## Move task

Puts a task in a column before another task or at the column's end, relabeling its issue on GitHub when the column changes and closing or reopening it when it enters or leaves Closed.

`POST /api/projects/{id}/tasks/{taskID}/move`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` |
| `taskID` | path | integer | a task's `id` in the project |
| `columnId` | body | integer | a column `id` on this board |
| `beforeTaskId` | body | integer or null | another task in that column, which the task lands above; null or left out for the column's end |
| `version` | body | integer | the task's `version` as last loaded |

```json
{ "columnId": 2, "beforeTaskId": 11, "version": 1 }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | task |
| `400` | `invalid_column`, `invalid_before_task`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `conflict`, and the current task; `run_in_progress`, `not_tested` |
| `502` | `github_error` |

While the project's [automation](automation.md) is on, a move to another column also follows these rules. A reorder within a column is not such a move.

| Move | Rule |
| --- | --- |
| Out of Testing | Refused with `run_in_progress` while the task's latest test run is `queued` or `running`. [Cancel test run](automation.md#cancel-test-run) unlocks it. Otherwise it clears `needsAttention`. |
| Into User Check | Refused with `not_tested` unless the task's latest test run `passed` on the head commit its pull request has now. |
| Into Testing | Starts a test run, which cancels the task's earlier queued or running one, of the one open, ready pull request that closes the issue and may run, at its head: from a branch of the repository, by an author with write access as GitHub says now, and not refused with a comment. With none, or more than one, the run ends at once as a runner error, and the task needs attention. |
| Any other | Free, Closed included. |

The rules hold for this endpoint only. A sync follows GitHub: a board label added there, or the issue closed, moves the task whatever its test run, and approving checks the run again. A sync's move out of Testing also clears `needsAttention`.

```json
{
  "id": 7,
  "issueNumber": 12,
  "columnId": 3,
  "position": 1.5,
  "version": 2,
  "title": "Draw the timeline",
  "state": "open",
  "url": "https://github.com/ada/gantt/issues/12",
  "runStatus": "queued",
  "needsAttention": null
}
```

```json
{
  "error": {
    "code": "run_in_progress",
    "message": "This task's test run is still going. Cancel the run to move the task out of Testing."
  }
}
```

```json
{
  "error": {
    "code": "conflict",
    "message": "This task changed since you loaded it. It is shown where it is now."
  },
  "task": {
    "id": 7,
    "issueNumber": 12,
    "columnId": 3,
    "position": 4,
    "version": 3,
    "title": "Draw the timeline",
    "state": "open",
    "url": "https://github.com/ada/gantt/issues/12",
    "runStatus": null,
    "needsAttention": null
  }
}
```
