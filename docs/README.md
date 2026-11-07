# API

Every endpoint is under `/api` and speaks JSON. One file per feature documents its endpoints.

## Conventions

| Rule | Detail |
| --- | --- |
| Session | Endpoints that list the `session` cookie as input answer `401 unauthorized` without a valid one. Sign up and log in set it. |
| Origin | `POST`, `PATCH` and `DELETE` need an `Origin` header equal to `PUBLIC_URL`, else `403 bad_origin`. |
| Errors | `{"error": {"code": "email_taken", "message": "An account with this email already exists."}}`. Output tables list the `code`. |
| Bad body | A body that is not valid JSON, or over 1 MiB, answers `400 bad_request`. |
| Unknown route | `404 not_found`. A known path with the wrong method answers `405 method_not_allowed` with an `Allow` header. |
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
