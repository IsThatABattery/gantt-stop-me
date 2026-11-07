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

Needs only Docker and VS Code (it recommends the Dev Containers extension). Open the repo and run **Dev Containers: Reopen in Container**. The repo mounts live at `/workspaces/gantt-stop-me`, and only the Go + Node workspace starts. Compose project `gantt-stop-me-dev` keeps its containers and data apart from the host stack.

Run and Debug entries:

- **Full stack**: DB + API + Web; stopping one stops all.
- **DB**: starts the `db` container and tails its logs; stopping it stops `db`.
- **API**: Go under Delve on :8080; starts `db` first.
- **Web**: Vite on :5173, proxying `/api` to :8080, opened in your default browser.
- **Web (Chrome debug)**: as Web, with Chrome attached for breakpoints. Needs Chrome on the host.
