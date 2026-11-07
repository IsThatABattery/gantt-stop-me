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

## Dev container

Open the repo in VS Code and run **Dev Containers: Reopen in Container**. It starts a Go + Node workspace and `db` under the compose project `gantt-stop-me-dev`, so its data stays apart from the host stack. In Run and Debug, pick **Full stack**, **API** (Go under Delve, :8080), or **Web** (Vite on :5173, proxying `/api` to :8080, with Chrome attached).
