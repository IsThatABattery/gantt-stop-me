# Agents

Design lives in `ARCHITECTURE.md`. Automation (test runs, the runner, approve and merge) lives in `AUTOMATION.md`, which starts with its build order. Terms are in `GLOSSARY.md`. Code lives in `src/`: `src/front` (React + Vite) and `src/back/api` (Go).

## Starting the stack

```sh
docker compose up --build
```

Done when `curl localhost:8080/api/health` prints `ok`. The app is at http://localhost:8080.

- `web` (nginx serving the Vite build) is the only published port. It proxies `/api/*` to `api`, so hit the API through `localhost:8080/api`.
- `db` data persists in the `db-data` volume. `docker compose down -v` wipes it.
- `web` serves a production build, so frontend changes need a rebuild: `docker compose up --build web`. For hot reload, run `npm run dev` in `src/front` instead.

## Dev container

Needs only Docker and VS Code (it recommends the Dev Containers extension). Open the repo and run **Dev Containers: Reopen in Container**. The repo mounts live at `/workspaces/gantt-stop-me`, and only the Go + Node workspace starts. Compose project `gantt-stop-me-dev` keeps its containers and data apart from the host stack.

Run and Debug entries:

- **Full stack**: DB + API + Web; stopping one stops all.
- **DB**: starts the `db` container and tails its logs; stopping it stops `db`.
- **API**: Go under Delve on :8080; starts `db` first.
- **Runner**: the runner under Delve on :8081 with prefix `gantt-dev`, driving the host's Docker. The API entry reaches it at `RUNNER_URL`.
- **Web**: Vite on :5173, proxying `/api` to :8080, opened in your default browser.
- **Web (Chrome debug)**: as Web, with Chrome attached for breakpoints. Needs Chrome on the host.

## API docs

`docs/` documents every endpoint, one file per feature, in the shape of `docs/accounts.md`: name, one sentence, method and path, input table, output table. Change an endpoint and its doc in the same commit.

## Tests

```sh
docker compose run --rm api-test
```

Run it after every change to `src/back/api`. Done when it exits 0.

- Tests and every `go` command run in the `api-test` container, so the host needs only Docker. Append a command to run something else: `docker compose run --rm api-test go test ./internal/accounts -run Login`, or `docker compose run --rm --no-deps api-test go mod tidy`.
- Every endpoint's handler file has a test file beside it: `signup.go`, `signup_test.go`. Endpoint tests start the whole API with `apptest.New(t)`, which gives each test its own migrated schema in `test-db`.
- `test-db` stays up between runs. `docker compose --profile test down` stops it.
- The Docker tests run only in `runner-test`, which has the Docker socket: `docker compose --profile test run --rm runner-test`. They are the runner's and `TestRunOnDocker`, one test run from a ready PR to its merge through `internal/automation` on the real runner. Run it after every change to `internal/runner`, `cmd/runner` or how `internal/automation` drives a run. Done when it exits 0.
- `TestLiveLLMOnDocker` routes and runs two LLM checks against a real provider, so it skips without a key: `LLM_LIVE_KEY=… docker compose --profile test run --rm -e LLM_LIVE_KEY -e LLM_LIVE_DECISION_MODEL=… -e LLM_LIVE_CHAT_MODEL=… runner-test go test -count=1 -v -run TestLiveLLMOnDocker ./internal/automation`. `LLM_LIVE_BASE_URL` defaults to OpenAI; `LLM_LIVE_ARTIFACTS` names a dir for the checks' screenshots and videos.
