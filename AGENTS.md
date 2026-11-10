# Agents

How the system fits together is in `docs/architecture.md`, and terms are in `docs/glossary.md`. Automation's endpoints and contracts (test runs, the runner, approve and merge, the workflows file) are in `docs/api/automation.md`. `docs/README.md` indexes every doc. Code lives in `src/`: `src/front` (React + Vite, guide in `src/front/README.md`) and `src/back/api` (Go).

## Starting the stack

```sh
docker compose up --build
```

Done when `curl localhost:8080/api/health` prints `ok`. The app is at http://localhost:8080.

- `web` (nginx serving the Vite build) is the only published port. It proxies `/api/*` to `api`, so hit the API through `localhost:8080/api`.
- `db` data persists in the `db-data` volume. `docker compose down -v` wipes it.
- `web` serves a production build, so frontend changes need a rebuild: `docker compose up --build web`. For hot reload, run `npm run dev` in `src/front` instead; it proxies `/api` to the stack on :8080.

## Dev container

Needs only Docker and VS Code (it recommends the Dev Containers extension). Open the repo and run **Dev Containers: Reopen in Container**. The repo mounts at `/workspaces/gantt-stop-me`, and only the Go + Node workspace starts. Compose project `gantt-stop-me-dev` keeps its containers and data apart from the host stack.

Run and Debug entries:

- **Full stack**: DB + API + Web; stopping one stops all.
- **DB**: starts the `db` container and tails its logs; stopping it stops `db`.
- **API**: Go under Delve on :8080; starts `db` first.
- **Runner**: the runner under Delve on :8081 with prefix `gantt-dev`, driving the host's Docker. The API entry reaches it at `RUNNER_URL`. Full stack does not start it.
- **Web**: Vite on :5173, proxying `/api` to :8080, opened in your default browser.
- **Web (Chrome debug)**: as Web, with Chrome attached for breakpoints. Needs Chrome on the host.

**API**, **Web** and **Web (Chrome debug)** set `PUBLIC_URL=http://localhost:5173` in `.vscode/launch.json`, and **API** also sets `RUNNER_URL` and the GitHub OAuth App (from the container's `DEV_GITHUB_*`). The container's environment sets none of them, because its shell drives the host's Docker and `docker compose up` there would pass them to the host stack. Started by hand in the container, Vite needs `PUBLIC_URL=http://localhost:5173 npm run dev`, and the API the variables its entry sets.

## Branches

Every branch pushed to GitHub, so every PR's branch, is named `<type>/<slug>`:

| Type | For |
| --- | --- |
| `feature/` | New behaviour |
| `fix/` | A bug fix |
| `docs/` | Docs only |
| `chore/` | Config, tooling, cleanup: no change in behaviour |

The slug is a few lowercase words joined by hyphens, such as `fix/backend-endpoints` or `docs/branch-naming`. Branches the app makes for tasks are `feature/<issue>-<slug>`, such as `feature/29-add-a-persistent-user-facing-theme`.

- Never push a tool's own branch name, such as T3 Code's worktree branch `t3/…`. Push the work under its own name instead: `git push -u origin HEAD:docs/branch-naming`, then open the PR from that branch.

## API docs

`docs/api/` documents every endpoint, one file per feature, in the shape of `docs/api/accounts.md`: name, one sentence, method and path, input table, output table. Change an endpoint and its doc in the same commit. Features not built yet are listed in `docs/gaps.md`.

## Design

Frontend design work uses the impeccable skill. Its product context and design system are `src/front/PRODUCT.md` and `src/front/DESIGN.md`, with DESIGN.md's sidecar and the surface briefs in `src/front/.impeccable/`. They are not at the root.

- `.impeccable/config.json` stays at the root, because impeccable's edit hook reads it only there. Its `projectRoots` names `src/front` as the workspace.
- Run the skill's `impeccable` script (`scripts/impeccable` in its skill folder) with a `--target` under `src/front`, such as `impeccable context --target src/front/src/pages/board/BoardPage.tsx`, or from `src/front`. From the root without a target, it asks which workspace.
- When impeccable regenerates DESIGN.md or writes a brief, it belongs in `src/front`. Never add a PRODUCT.md or DESIGN.md at the root.

## Tests

```sh
docker compose run --rm api-test
```

Run it after every change to `src/back/api`. Done when it exits 0.

- Tests and every `go` command run in the `api-test` container, so the host needs only Docker. Append a command to run something else: `docker compose run --rm api-test go test ./internal/accounts -run Login`, or `docker compose run --rm --no-deps api-test go mod tidy`.
- Every endpoint's handler file has a test file beside it: `signup.go`, `signup_test.go`. Endpoint tests start the whole API with `apptest.New(t)`, which gives each test its own migrated schema in `test-db`.
- `test-db` stays up between runs. `docker compose --profile test down` stops it.
- The Docker tests run only in `runner-test`, which has the Docker socket: `docker compose --profile test run --rm runner-test`. They are the runner's and `TestRunOnDocker`, one test run from a ready PR to its merge through `internal/automation` on the real runner. Run it after every change to `internal/runner`, `cmd/runner` or how `internal/automation` drives a run. Done when it exits 0.
- `TestLiveLLMOnDocker` routes and runs two LLM checks against a real provider, so it skips without a key: `LLM_LIVE_KEY=… docker compose --profile test run --rm -e LLM_LIVE_KEY -e LLM_LIVE_DECISION_MODEL=… -e LLM_LIVE_CHAT_MODEL=… runner-test go test -count=1 -v -run TestLiveLLMOnDocker ./internal/automation`. `LLM_LIVE_BASE_URL` defaults to OpenAI; `-e LLM_LIVE_ARTIFACTS=/src/live-artifacts` saves the checks' screenshots and videos to `src/back/api/live-artifacts`, which git ignores.

Frontend tests:

```sh
docker compose run --rm web-test
```

Run it after every change to `src/front`. Done when it exits 0.

- Append an npm command to run something else: `docker compose run --rm web-test npm run lint`. `docker compose build web` typechecks and builds.
- `node_modules` lives in the `web-node-modules` volume, so the host needs none.
