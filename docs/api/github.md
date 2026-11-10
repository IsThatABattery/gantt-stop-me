# GitHub

## OAuth availability

Reports whether this server can link and sign in with OAuth: it can when both `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` are set. It needs no session, since the sign-in page asks it too.

`GET /api/github/oauth`

**Input**

None.

**Output**

| Status | Body |
| --- | --- |
| `200` | availability |

```json
{ "enabled": true }
```

## Link with OAuth

Starts linking with OAuth by sending the browser to GitHub. The browser navigates here, so it answers only with redirects, never JSON. A redirect to Settings, here or from the callback, ends in `#github`, so the page opens at its GitHub section, which shows the outcome.

`GET /api/github/link`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required; for a user with a password, started or [confirmed with it](accounts.md#confirm-password) in the last 10 minutes |

**Output**

| Status | Body |
| --- | --- |
| `302` | Location `https://github.com/login/oauth/authorize?client_id={GITHUB_CLIENT_ID}&redirect_uri={PUBLIC_URL}/api/github/callback&scope=repo+workflow&state={state}` with each value URL-encoded, and sets the `github_state` cookie, sent only to the callback, to `state` for 10 minutes. The `workflow` scope lets automation merge pull requests that change `.github/workflows` |
| `302` | Location `{PUBLIC_URL}/settings?github=unavailable#github` while OAuth is not set up (see OAuth availability), setting no cookie |
| `302` | Location `{PUBLIC_URL}/settings?github=confirm#github` when the user has a password and the session neither started nor confirmed it in the last 10 minutes, setting no cookie and starting nothing. Settings asks for the password, [confirms it](accounts.md#confirm-password), and comes back here |
| `302` | Location `{PUBLIC_URL}/settings?github=error#github` without a valid `session`, deleting the `github_state` cookie |

An OAuth link signs in, so a stolen session must not add one: the thief's GitHub account would go on signing in as the user after they changed their password, which ends every other session. A user with no password is not asked, since they have none to confirm and the callback lets them link only the GitHub account they sign in with.

A GitHub account is linked with OAuth to one user at most: [Sign in with GitHub](#sign-in-with-github) signs in as the user linked to it, so the callback refuses one another user is linked to with OAuth. A link made with a personal access token signs no one in, so it does not count, and the callback does not refuse a GitHub account linked so. A user with no password signs in only with the GitHub account they are linked to, so until they set one the callback links that one again, as when its token expired, and refuses another.

## Sign in with GitHub

Starts signing in with GitHub by sending the browser to GitHub, for the sign-in page's Continue with GitHub, and for Settings to start a fresh session that can set a first password. It needs no session, and a browser that has one gets a new one when the sign-in works. It finishes at the same callback as linking, so the OAuth App registers nothing more. The browser navigates here, so it answers only with redirects, never JSON.

`GET /api/auth/github`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `next` | query | string | optional; the path on the app a sign-in that works sends the browser to, such as `/settings`. It counts only when it matches `^/[A-Za-z0-9/_-]*$` and does not start with `//`, so it names no other host, query or fragment; any other value is ignored |

**Output**

| Status | Body |
| --- | --- |
| `302` | Location `https://github.com/login/oauth/authorize?client_id={GITHUB_CLIENT_ID}&redirect_uri={PUBLIC_URL}/api/github/callback&scope=repo+workflow+user%3Aemail&state={state}` with each value URL-encoded, and sets the `github_state` cookie as Link with OAuth does. There is no session to tie `state` to, so the cookie alone ties it to the browser. `user:email` reads the verified email a new user is made with. A `next` that counts sets the `github_next` cookie to it, as `github_state` is set: `HttpOnly`, `SameSite=Lax`, `Secure` on https, sent only to the callback, for 10 minutes. Without one, it deletes `github_next`, so an abandoned sign-in's does not apply |
| `302` | Location `{PUBLIC_URL}/login?github=unavailable` while OAuth is not set up (see OAuth availability), setting no cookie |

## OAuth callback

Finishes linking or signing in with OAuth, whichever `state` was issued for, or, without `state`, whichever the `github_state` cookie was. A link replaces any earlier link and sends the browser back to Settings with the outcome. A sign-in signs the browser in, or sends it back to the sign-in page with the outcome. Either way the user's access to each project is checked again with the new credential the next time it is used.

`GET /api/github/callback`

**Input**

| Name | In | Type | Rules |
| --- | --- | --- | --- |
| `session` | cookie | string | required for a link: the session that started it; not read for a sign-in |
| `github_state` | cookie | string | set by Link with OAuth or Sign in with GitHub |
| `github_next` | cookie | string | optional, set by Sign in with GitHub; read only by a sign-in that works, and only when it still counts as `next` does |
| `code` | query | string | the authorization code GitHub sent |
| `state` | query | string | must equal the `github_state` cookie, which a link's ties to its session |
| `error` | query | string | optional, and read only once `state` checks out; `access_denied` means the user cancelled on GitHub, and any other value fails the flow |
| `error_description` | query | string | optional; logged with `error`, both quoted |

**Output**

Every answer deletes the `github_state` and `github_next` cookies and is a redirect, never JSON. The API logs why a flow failed, quoting the values GitHub or the query sent, and never the code, the tokens or the client secret; for `redirect_uri_mismatch` the log names the callback URL the OAuth App must register, `{PUBLIC_URL}/api/github/callback`.

A link saves nothing unless the outcome is `linked`. It takes turns with the GitHub account's sign-ins and other links, so none of them finds it unlinked while it links it.

| Status | Body |
| --- | --- |
| `302` | Location `{PUBLIC_URL}/settings?github=linked#github` |
| `302` | Location `{PUBLIC_URL}/settings?github=denied#github` when `state` checks out and `error` is `access_denied` |
| `302` | Location `{PUBLIC_URL}/settings?github=taken#github` when another user is linked to the GitHub account with OAuth |
| `302` | Location `{PUBLIC_URL}/settings?github=password_required#github` when the user has no password and is linked to another GitHub account, which is how they sign in |
| `302` | Location `{PUBLIC_URL}/settings?github=confirm#github` when the user has a password and the session neither started nor confirmed it in the last 20 minutes: Link with OAuth's 10, plus the state's 10 to finish. Whoever holds a session can mint its link state without Link with OAuth, so the rule holds here too |
| `302` | Location `{PUBLIC_URL}/settings?github=error#github` without a valid `session`, for a missing or mismatched `state` whatever `error` says, for any other `error`, or when GitHub refuses the code or cannot be reached |

A sign-in loads the GitHub account with `GET /user`, then signs in as the one user linked to it with OAuth, saving the new credential as linking again does. A link made with a personal access token signs no one in: any token of the account, such as one a third-party app was given, makes one. When no user is linked to it with OAuth, it reads the account's emails with `GET /user/emails` and makes a user with its primary email, which GitHub must have verified, and no password, linked to the account. One GitHub account's sign-ins and links take turns, so two at once make or link one user. Only a sign-in that works saves anything, except when starting the session fails after the save: the browser then gets `error`, and signing in again works.

| Status | Body |
| --- | --- |
| `302` | Location `{PUBLIC_URL}{next}`, the `next` the sign-in started with, else `{PUBLIC_URL}/`, and sets the `session` cookie as [Log in](accounts.md#log-in) does |
| `302` | Location `{PUBLIC_URL}/login?github=denied` when `state` checks out and `error` is `access_denied` |
| `302` | Location `{PUBLIC_URL}/login?github=ambiguous` when more than one user is linked to the GitHub account with OAuth, as only links from before the callback refused a GitHub account another user is linked to can be |
| `302` | Location `{PUBLIC_URL}/login?github=no_email` when no user is linked to it and it has no verified primary email |
| `302` | Location `{PUBLIC_URL}/login?github=account_exists` when no user is linked to it and a user has its email, ignoring case. They sign in with their password and link GitHub in Settings: signing in as them would let whoever holds a GitHub account with that email into theirs |
| `302` | Location `{PUBLIC_URL}/login?github=error` for a missing or mismatched `state` whatever `error` says, for any other `error`, or when GitHub refuses the code or cannot be reached |

GitHub may issue a token that expires, after 8 hours by default for a new OAuth App, with a refresh token that works once: the API stores both and, whenever it reads the token within 5 minutes of its expiry, first trades the refresh token for new ones, one refresh per user at a time. When that fails:

- GitHub refuses the refresh token, as `bad_refresh_token`: the API forgets it, keeps the link, and uses the stored token as it is from then on, so GitHub's `401` for it asks the user to link again, as `403 github_not_linked` on the routes that use it.
- GitHub refuses the refresh another way, such as `incorrect_client_credentials` for the OAuth App, which linking again does not mend, or GitHub cannot be reached, or answers `5xx` or `429`: the stored token is used until it expires, and after that routes that need it answer `502 github_error`.
- Another API instance has held the link for its own refresh for over 5 seconds: as when GitHub cannot be reached.

After a refusal other than `bad_refresh_token`, or GitHub out of reach, the API instance tries no refresh for that user for 30 seconds: their reads meanwhile use the stored token until it expires, then answer `502 github_error`. Linking again ends that wait. A token stored with no refresh token, or while the OAuth App is not set up, is used as it is.

GitHub retires a token as it refreshes it, so another request's refresh can make GitHub answer `401` to a call made with the token read just before. The access check every project route starts with then asks GitHub again with the new token. A later call's `401` for a token since replaced answers `502 github_error`, for the user to try again, not `403 github_not_linked`.

## Link with a token

Links the GitHub account a personal access token belongs to, replacing any earlier link. The user's access to each project is checked again with the new token the next time it is used. A user with no password, made by signing in with GitHub, signs in only with their OAuth link, which a token link would replace, so they set a password first.

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
| `409` | `password_required`, when the user has no password; nothing changes |
| `502` | `github_unavailable`, when GitHub's `GET /user` fails another way or takes over 15 seconds |

```json
{ "kind": "pat", "login": "ada" }
```

## Unlink GitHub

Forgets the linked GitHub account and the user's access to every project, so after linking again their project list starts empty. It succeeds when none is linked. A user with no password, as one made by signing in with GitHub, sets one first, so they can still sign in.

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
| `409` | `password_required`, when the user has no password; nothing changes |
