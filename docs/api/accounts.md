# Accounts

## Sign up

Creates an account and signs it in.

`POST /api/auth/signup`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `email` | body | string | a bare address, with no display name or surrounding spaces, at most 254 bytes, unique ignoring case |
| `password` | body | string | at least 8 bytes |

```json
{ "email": "ada@example.com", "password": "correct horse" }
```

**Output**

| Status | Body |
| --- | --- |
| `201` | user, and sets the `session` cookie |
| `400` | `invalid_email`, `weak_password`, `bad_request` |
| `409` | `email_taken` |

```json
{ "id": 1, "email": "ada@example.com" }
```

## Log in

Signs in with email and password. An account with no password, as one made by [signing in with GitHub](github.md#sign-in-with-github) until its user sets one, answers as a wrong password does, and takes as long.

`POST /api/auth/login`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `email` | body | string | matched ignoring case |
| `password` | body | string | |

```json
{ "email": "ada@example.com", "password": "correct horse" }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | user, with `email` as it was signed up, and sets the `session` cookie |
| `400` | `bad_request` |
| `401` | `invalid_credentials` |

```json
{ "id": 1, "email": "ada@example.com" }
```

## Log out

Ends the current session.

`POST /api/auth/logout`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | optional |

**Output**

| Status | Body |
| --- | --- |
| `204` | empty, and deletes the `session` cookie |

## Confirm password

Checks the signed-in user's password and, when it is right, makes the current session recent again for 10 minutes, as a new session is for its first 10, so it can [link GitHub with OAuth](github.md#link-with-oauth). An OAuth link signs in, so a user with a password adds one only from a recent session: an older one may be stolen, and the thief's GitHub account would go on signing in as the user after they change their password. It marks only the current session, and a wrong password takes as long to refuse as a right one to accept, as at [Log in](#log-in).

`POST /api/auth/confirm`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `password` | body | string | the password in use |

```json
{ "password": "correct horse" }
```

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `400` | `bad_request` |
| `401` | `unauthorized` |
| `403` | `wrong_password`, "Your password is wrong."; nothing changes |
| `409` | `password_required`, "This account has no password. Sign in with GitHub again instead.", for an account with none, as one made by signing in with GitHub, whose only recent session is one it just started; nothing changes |

## Current user

Returns the signed-in user, their display name, their linked GitHub account and whether they have a password.

`GET /api/me`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `200` | user, with `name` their display name, `null` until set, `github` the linked login and its `kind` (`oauth` or `pat`), or `null` when none is linked, and `hasPassword`, `false` for an account made by signing in with GitHub until its user sets one |
| `401` | `unauthorized` |

```json
{ "id": 1, "email": "ada@example.com", "name": "Ada Lovelace", "github": { "login": "ada", "kind": "oauth" }, "hasPassword": true }
```

```json
{ "id": 1, "email": "ada@example.com", "name": null, "github": null, "hasPassword": true }
```

```json
{ "id": 2, "email": "octo@example.com", "name": null, "github": { "login": "octocat", "kind": "oauth" }, "hasPassword": false }
```

## Edit profile

Sets or clears the signed-in user's display name.

`PATCH /api/me`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `name` | body | string or null | trimmed, 1 to 100 characters, not all whitespace, no control, format (such as zero-width or right-to-left marks) or line and paragraph separator characters; `null` or left out clears it |

```json
{ "name": "Ada Lovelace" }
```

**Output**

| Status | Body |
| --- | --- |
| `200` | user, as [Current user](#current-user) answers it |
| `400` | `invalid_name`, `bad_request` |
| `401` | `unauthorized` |

```json
{ "id": 1, "email": "ada@example.com", "name": "Ada Lovelace", "github": { "login": "ada", "kind": "oauth" }, "hasPassword": true }
```

## Change password

Replaces the password and signs out every other session. For an account with no password, as one made by signing in with GitHub, it sets the first one instead, which needs no current password but a session started in the last 10 minutes, and signs out every other session too. Such an account signs in only with GitHub, so a session that new shows its user holds the GitHub account, and an older one, perhaps stolen, cannot set a password and then unlink GitHub. It has no password to [confirm](#confirm-password), so its user starts one with [Sign in with GitHub](github.md#sign-in-with-github), with `next=/settings` to come back to Settings.

`PATCH /api/me/password`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required; while the account has no password, started in the last 10 minutes |
| `currentPassword` | body | string | the password in use; ignored, and may be left out, while the account has none |
| `newPassword` | body | string | at least 8 bytes |

```json
{ "currentPassword": "correct horse", "newPassword": "battery staple" }
```

```json
{ "newPassword": "battery staple" }
```

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `400` | `weak_password`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `wrong_password`; or `reauth_required`, "Sign in with GitHub again, then set your password.", when the account has no password and the session started over 10 minutes ago; nothing changes |
