# GitHub

## Link with OAuth

Starts linking with OAuth by sending the browser to GitHub.

`GET /api/github/link`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `302` | Location `https://github.com/login/oauth/authorize?client_id={GITHUB_CLIENT_ID}&redirect_uri={PUBLIC_URL}/api/github/callback&scope=repo&state={state}` with each value URL-encoded, and sets the `github_state` cookie, sent only to the callback, to `state` for 10 minutes |
| `401` | `unauthorized` |
| `503` | `oauth_not_configured` |

## OAuth callback

Finishes linking with OAuth, replacing any earlier link, and sends the browser back to Settings with the outcome. The user's access to each project is checked again with the new credential the next time it is used.

`GET /api/github/callback`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required; the session that started the link |
| `github_state` | cookie | string | set by Link with OAuth |
| `code` | query | string | the authorization code GitHub sent |
| `state` | query | string | must equal the `github_state` cookie, which is tied to that session |
| `error` | query | string | optional; any non-empty value fails the link |

**Output**

| Status | Body |
| --- | --- |
| `302` | Location `{PUBLIC_URL}/settings?github=linked`, or `{PUBLIC_URL}/settings?github=error` when the link failed and nothing was saved; either way deletes the `github_state` cookie |
| `401` | `unauthorized` |

## Link with a token

Links the GitHub account a personal access token belongs to, replacing any earlier link. The user's access to each project is checked again with the new token the next time it is used.

`POST /api/github/token`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `token` | body | string | a personal access token, trimmed; blank answers `bad_request`; a control character, or a `401` from GitHub's `GET /user`, answers `invalid_token` |

```json
{ "token": "github_pat_11ABCDEFG0123456789_abcdefghijklmnopqrstuvwxyz" }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | link |
| `400` | `invalid_token`, `bad_request` |
| `401` | `unauthorized` |
| `502` | `github_unavailable`, when GitHub's `GET /user` fails another way or takes over 15 seconds |

```json
{ "kind": "pat", "login": "ada" }
```

## Unlink GitHub

Forgets the linked GitHub account and the user's access to every project, so after linking again their project list starts empty. It succeeds when none is linked.

`DELETE /api/github`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `401` | `unauthorized` |
