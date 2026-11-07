# Agents

Design lives in `ARCHITECTURE.md`. Code lives in `src/`: `src/front` (React + Vite) and `src/back/api` (Go).

## Starting the stack

```sh
docker compose up --build
```

Done when `curl localhost:8080/api/health` prints `ok`. The app is at http://localhost:8080.

- `web` (nginx serving the Vite build) is the only published port. It proxies `/api/*` to `api`, so hit the API through `localhost:8080/api`.
- `db` data persists in the `db-data` volume. `docker compose down -v` wipes it.
- `web` serves a production build, so frontend changes need a rebuild: `docker compose up --build web`. For hot reload, run `npm run dev` in `src/front` instead.

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
