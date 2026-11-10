# API

Every endpoint is under `/api` and speaks JSON, unless its doc says otherwise. One file per feature documents its endpoints:

- [Accounts](accounts.md): sign up, log in and out, the current user, display name, and setting or changing the password.
- [GitHub](github.md): whether OAuth is available, link a GitHub account with OAuth or a personal access token, sign in with GitHub, and unlink it.
- [LLM providers](providers.md): the user's OpenAI-compatible endpoints, their keys and chosen models.
- [Board](board.md): projects, the board, tasks, their tests, assignees and moves, and the tasks assigned to the user.
- [Planning](planning.md): planning settings, and plans, where one agent interviews a member into a plan doc and another turns the doc into proposed tasks.
- [Automation](automation.md): turning automation on and off, workflows, the GitHub webhook, test runs, approving and merging, requesting changes, and taking a task over.

Terms are in the [glossary](../glossary.md).

## Conventions

| Rule | Detail |
| --- | --- |
| Session | Endpoints that require the `session` cookie answer `401 unauthorized` without a valid one, but for the two a browser navigates to, [Link with OAuth](github.md#link-with-oauth) and the [OAuth callback](github.md#oauth-callback) of a link, which send it back to Settings. Sign up, log in and [Sign in with GitHub](github.md#sign-in-with-github) set it: `HttpOnly`, `SameSite=Lax`, `Secure` when `PUBLIC_URL` is https, for 30 days. |
| Origin | Every method but `GET`, `HEAD` and `OPTIONS` needs an `Origin` header equal to the origin of `PUBLIC_URL`, else `403 bad_origin`. It is checked before routing, so a write to an unknown route answers `403` too. The one exception is `POST /api/github/webhook/{projectID}`, which checks GitHub's signature instead. |
| Errors | `{"error": {"code": "email_taken", "message": "An account with this email already exists."}}`. Output tables list the `code`. |
| Bad body | A body that is not valid JSON, has a field of the wrong type, or holds more than 1 MiB of JSON answers `400 bad_request`. Unknown fields are ignored. The webhook takes up to 5 MiB. |
| Unknown route | `404 not_found`. A known path with the wrong method answers `405 method_not_allowed` with an `Allow` header. A path with `//`, `.` or `..` segments answers `301` to the cleaned path. |
| Failure | `500 internal`. |

## Health

Reports that the API is serving.

`GET /api/health`

**Input**

None.

**Output**

| Status | Body |
| --- | --- |
| `200` | `ok` (plain text) |
