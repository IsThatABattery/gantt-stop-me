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

## List projects

Returns the projects the caller has added or opened, most recently visited first, keeping only those their GitHub account still sees. Opening a project's board, which every project page loads, counts as a visit. It never lists other repositories the account can see.

`GET /api/projects`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `200` | `projects`, each as [Add project](#add-project) answers it; `[]` for none |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |

A project whose access check is over 5 minutes old is checked on GitHub again, and dropped, as from opening it, when the account no longer sees it. When GitHub fails that check for another reason, such as an outage, the project is left out of this answer only. A credential GitHub no longer accepts answers `github_not_linked`. Unlinking GitHub forgets every project. Linking it again keeps them, and checks each on GitHub again with the new credential.

```json
{ "projects": [{ "id": 1, "owner": "ada", "name": "gantt", "permission": "write" }] }
```

## List assigned tasks

Returns the open tasks assigned to the caller's GitHub account, grouped by project in [List projects](#list-projects) order, leaving out projects with none.

`GET /api/me/tasks`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `200` | `projects`, each a `project` as [Add project](#add-project) answers it and its `tasks` in board order; `[]` for none |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |

Each task is as [Show board](#show-board) gives it, with `column`, the `key` and `name` of the column it is in. Assignees are as the project's last sync read them, so an assignment made on GitHub shows once someone opens that project's board.

```json
{
  "projects": [
    {
      "project": { "id": 1, "owner": "ada", "name": "gantt", "permission": "write" },
      "tasks": [
        {
          "id": 7,
          "issueNumber": 12,
          "columnId": 3,
          "position": 1,
          "version": 2,
          "title": "Draw the timeline",
          "state": "open",
          "url": "https://github.com/ada/gantt/issues/12",
          "runStatus": "running",
          "needsAttention": null,
          "assignees": [{ "login": "ada", "avatarUrl": "https://avatars.githubusercontent.com/u/1" }],
          "column": { "key": "testing", "name": "Testing" }
        }
      ]
    }
  ]
}
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

Every task, here and wherever an endpoint answers one, has `runStatus`, its latest test run's status as [List test runs](automation.md#list-test-runs) gives it, `null` when it has none, `needsAttention`, why it waits in Testing after a runner error, `null` while clear, `assignees`, its issue's assignees as each `login` and `avatarUrl`, as last read from GitHub like its title, and `branch`, the branch made for it on GitHub when it first moved to In Progress, `null` until then.

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
      "needsAttention": null,
      "assignees": []
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
      "needsAttention": null,
      "assignees": []
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
  "needsAttention": null,
  "assignees": []
}
```

## Get task

Returns a task with its issue's title, state, body, labels and assignees as last read from GitHub, its tests, and the pull requests that close its issue, syncing the tasks from the repository's issues first when the last sync is over 30 seconds old.

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
| `200` | task, with its issue's body and labels, its tests and `testsError`, `pullRequests` and `pullRequestsError`, and `takenOverBy` |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

`tests` are the task's own, which [test runs](automation.md) run. The `## Tests` section of the issue body is GitHub's view of them, kept in step both ways: [Set tests](#set-tests) writes it, and every read of the issue (adding the project, a sync, [Create task](#create-task), [Edit task](#edit-task), a test run) takes a section changed on GitHub. A deleted section takes no tests. A section that does not parse, such as one with more than 64 tests, is not taken: the task keeps its tests, and `testsError` says why until the section parses or is put back, else it is `null`. Each test is:

| Field | Type | Meaning |
| --- | --- | --- |
| `name` | string | its line; for a script written as one code span, its command |
| `kind` | string | `script`, a command whose exit code decides, or `check`, a plain-English check for an LLM |
| `command` | string | a script's command, `""` for a check |
| `details` | string | a check's indented block, such as its steps, `""` when none or for a script |
| `workflow` | string or null | the workflow its `[name]` pin names, as written, else `null` for the decision model to pick |

`pullRequests` are the pull requests of the project's repository, open, closed or merged, that close the issue on GitHub, such as with "Closes #12", whether the project's automation is on or off. Pull requests of other repositories that close it are left out. They are read from GitHub with the caller's credential when the task's last read of them is over 30 seconds old, else answered as last read. When that read fails, they are answered as last read too, and `pullRequestsError` says so, else it is `null`; the next view reads them again. Each has `number`, `title`, `state` (`open`, `closed` or `merged`), `draft` and `url`. `takenOverBy` is the member who [took the task over](automation.md#take-over-task) from automation, by user `id` and GitHub `login` (`null` once they unlink GitHub), else `null`.

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
  "assignees": [{ "login": "ada", "avatarUrl": "https://avatars.githubusercontent.com/u/1?v=4" }],
  "body": "One bar per task, by week.\n\n## Tests\n- [ ] `npm test`\n- [ ] The timeline shows one bar per task\n  1. Open a board with two tasks.\n",
  "labels": [
    { "name": "board:in-progress", "color": "fbca04" },
    { "name": "enhancement", "color": "a2eeef" }
  ],
  "tests": [
    { "name": "npm test", "kind": "script", "command": "npm test", "details": "", "workflow": null },
    { "name": "The timeline shows one bar per task", "kind": "check", "command": "", "details": "1. Open a board with two tasks.", "workflow": null }
  ],
  "testsError": null,
  "pullRequests": [
    { "number": 15, "title": "Draw the timeline", "state": "open", "draft": false, "url": "https://github.com/ada/gantt/pull/15" }
  ],
  "pullRequestsError": null,
  "takenOverBy": null
}
```

## Edit task

Changes the title, body or assignees of a task's issue on GitHub. GitHub drops an assignee who cannot be assigned to the repository's issues without an error, so the answer's `assignees` show who was.

`PATCH /api/projects/{id}/tasks/{taskID}`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` |
| `taskID` | path | integer | a task's `id` in the project |
| `title` | body | string | trimmed, 1 to 256 characters; optional |
| `body` | body | string | at most 65536 characters; optional |
| `assignees` | body | string array | optional; GitHub logins, at most 10, each once; replaces the issue's assignees, and `[]` unassigns everyone |

At least one of `title`, `body` and `assignees` is sent.

```json
{ "title": "Draw the timeline by week", "body": "One bar per task." }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | task |
| `400` | `invalid_title`, `invalid_body`, `invalid_assignees`, `bad_request` |
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
  "needsAttention": null,
  "assignees": []
}
```

## List assignable users

Returns the people the project's tasks can be assigned to, as GitHub lists the repository's assignable users.

`GET /api/projects/{id}/assignees`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id`; read access |

**Output**

| Status | Body |
| --- | --- |
| `200` | `assignees`, each a `login` and `avatarUrl` |
| `401` | `unauthorized` |
| `403` | `github_not_linked` |
| `404` | `not_found` |
| `502` | `github_error` |

```json
{ "assignees": [{ "login": "ada", "avatarUrl": "https://avatars.githubusercontent.com/u/1?v=4" }] }
```

## Set tests

Replaces a task's tests, writing them to the Tests section of its issue on GitHub first.

`PUT /api/projects/{id}/tasks/{taskID}/tests`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `id` | path | integer | a project's `id` |
| `taskID` | path | integer | a task's `id` in the project |
| `tests` | body | array | required, `[]` for none; at most 64 tests, as [Get task](#get-task) shows them; at most 64 KiB as a Tests section; no NUL anywhere |
| `tests[].name` | body | string | one line, not blank |
| `tests[].kind` | body | string | `script` or `check` |
| `tests[].command` | body | string | a script's, not blank; ignored for a check |
| `tests[].details` | body | string | optional; ignored for a script |
| `tests[].workflow` | body | string or null | optional; 2 to 32 lowercase letters, digits or `-`, starting with a letter and ending in a letter or digit |
| `base` | body | array | required: the task's `tests` as you last loaded them, from [Get task](#get-task) or this endpoint's last answer |

```json
{
  "tests": [
    { "name": "npm test", "kind": "script", "command": "npm test" },
    { "name": "The timeline shows one bar per task", "kind": "check", "details": "1. Open a board with two tasks.", "workflow": "frontend" }
  ],
  "base": [
    { "name": "npm test", "kind": "script", "command": "npm test", "details": "", "workflow": null }
  ]
}
```

**Output**

| Status | Body |
| --- | --- |
| `200` | the tests, and the issue's `body` as GitHub now has it |
| `400` | `invalid_tests`, `invalid_body` when the issue body would pass 65536 characters, `bad_request` |
| `401` | `unauthorized` |
| `403` | `github_not_linked`, `forbidden` |
| `404` | `not_found` |
| `409` | `tests_changed`, and the task's `tests` and `testsError` as they now are, and the issue's `body` |
| `502` | `github_error` |

The issue's Tests section becomes the tests as an unticked checklist, each after its `[workflow]` pin: a script whose name is its command as that command in code, any other script as its name over an `sh` code block, and a check as its name over its indented details. The section is appended when the body has none and removed for no tests. The section runs from its `## Tests` line to the next `#` or `##` heading, and all of it is rewritten: text in it that is no test, such as a note after the list, is dropped. The rest of the body stays as it is, byte for byte. The task keeps the tests as the section reads back, so the answer may differ from the input: a check named only by a code span is a script, for one.

The issue's section is read first. When it changed on GitHub since the task last read it, the task takes it. If that changes the task's tests or gives a new `testsError`, or the task's tests are no longer `base`, such as after a sync took a GitHub edit, nothing is written, and the answer is `409` with the task as it now is. Sending the edit again with those tests as `base` replaces them. A change that leaves the tests as they are, such as a ticked box or a mended section, does not stop the edit. Both answers carry the issue's body: send that, not an older copy, to [Edit task](#edit-task), whose body's Tests section the task takes.

```json
{
  "tests": [
    { "name": "npm test", "kind": "script", "command": "npm test", "details": "", "workflow": null },
    { "name": "The timeline shows one bar per task", "kind": "check", "command": "", "details": "1. Open a board with two tasks.", "workflow": "frontend" }
  ],
  "body": "One bar per task, by week.\n\n## Tests\n- [ ] `npm test`\n- [ ] [frontend] The timeline shows one bar per task\n  1. Open a board with two tasks.\n"
}
```

```json
{
  "error": {
    "code": "tests_changed",
    "message": "The tests changed on GitHub since you loaded them. They are shown as they are now."
  },
  "tests": [
    { "name": "npm test", "kind": "script", "command": "npm test", "details": "", "workflow": null }
  ],
  "testsError": null,
  "body": "One bar per task, by week.\n\n## Tests\n- [x] `npm test`\n"
}
```

## Move task

Puts a task in a column before another task or at the column's end, relabeling its issue on GitHub when the column changes and closing or reopening it when it enters or leaves Closed. A task's first move into In Progress also makes its branch on GitHub off the default branch's head, `feature/{issue number}-{title}`, with the title in at most 40 lowercase letters, digits and hyphens; a branch of that name already there is kept. A failure to make it is logged and leaves `branch` `null`; the move still happens.

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

While the project's [automation](automation.md) is on, a move to another column also follows these rules, unless a member [took the task over](automation.md#take-over-task): then it moves freely and starts no test run. A reorder within a column is not such a move.

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
  "needsAttention": null,
  "assignees": []
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
    "needsAttention": null,
    "assignees": []
  }
}
```
