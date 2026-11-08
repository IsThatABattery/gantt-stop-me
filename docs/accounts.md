# Accounts

## Sign up

Creates an account and signs it in.

`POST /api/auth/signup`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `email` | body | string | a bare address, at most 254 characters, unique ignoring case |
| `password` | body | string | at least 8 characters |

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

Signs in with email and password.

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
| `200` | user, and sets the `session` cookie |
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

## Current user

Returns the signed-in user, their display name and their linked GitHub account.

`GET /api/me`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |

**Output**

| Status | Body |
| --- | --- |
| `200` | user, with `name` their display name, `null` until set, and `github` the linked login and its `kind` (`oauth` or `pat`), or `null` when none is linked |
| `401` | `unauthorized` |

```json
{ "id": 1, "email": "ada@example.com", "name": "Ada Lovelace", "github": { "login": "ada", "kind": "oauth" } }
```

```json
{ "id": 1, "email": "ada@example.com", "name": null, "github": null }
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
{ "id": 1, "email": "ada@example.com", "name": "Ada Lovelace", "github": { "login": "ada", "kind": "oauth" } }
```

## Change password

Replaces the password and signs out every other session.

`PATCH /api/me/password`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required |
| `currentPassword` | body | string | the password in use |
| `newPassword` | body | string | at least 8 characters |

```json
{ "currentPassword": "correct horse", "newPassword": "battery staple" }
```

**Output**

| Status | Body |
| --- | --- |
| `204` | empty |
| `400` | `weak_password`, `bad_request` |
| `401` | `unauthorized` |
| `403` | `wrong_password` |
