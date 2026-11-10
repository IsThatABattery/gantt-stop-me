# GanttStopMe

A Kanban board for GitHub repositories, where your team and AI agents work the backlog. Every task is a GitHub issue. Agents plan features into tasks and test each task's pull request in isolated containers. They report what they find, and a person approves and merges with the results in hand.

## Contents

- [What you need](#what-you-need)
- [Run the app with Docker](#run-the-app-with-docker)
- [Develop in the dev container](#develop-in-the-dev-container)
- [Frontend only](#frontend-only)
- [Tests](#tests)
- [Configuration](#configuration)
- [Link GitHub](#link-github)
- [Repository layout](#repository-layout)
- [Troubleshooting](#troubleshooting)
- [Further reading](#further-reading)

## What you need

- **Docker**, with Compose v2 (`docker compose`, not `docker-compose`). On Windows, use Docker Desktop with WSL.
- **Git.**
- For the dev container: **VS Code** with the **Dev Containers** extension.

You don't need Go, Node or PostgreSQL on your machine. Everything builds and runs in Docker containers.

> **Windows tip:** clone the repo inside your WSL filesystem (for example `~/code/gantt-stop-me`), not under `C:\`. Builds are much faster there.

## Run the app with Docker

The quickest way to see the whole app.

```sh
git clone https://github.com/IsThatABattery/gantt-stop-me.git
cd gantt-stop-me
docker compose up --build
```

It's ready when this prints `ok`:

```sh
curl localhost:8080/api/health
```

Then open **http://localhost:8080**.

- `web` (nginx serving the frontend build) is the only published port. It forwards `/api/*` to the API, so reach the API through `localhost:8080/api`.
- It serves a production build, so frontend changes need a rebuild: `docker compose up --build web`.
- The database lives in the `db-data` volume and survives restarts. `docker compose down -v` wipes it.

No `.env` file is needed: `docker-compose.yml` has dev defaults for everything. See [Configuration](#configuration) to change them, and [Link GitHub](#link-github) before you add a project.

## Develop in the dev container

The day-to-day setup, with hot reload and debuggers.

1. Open the repo in VS Code and run **Dev Containers: Reopen in Container**.
2. Wait for the first build. It installs Go, Node and the frontend dependencies.
3. Open **Run and Debug** and start **Full stack**. To run tests on pull requests, start **Runner** as well.
4. Open **http://localhost:5173**.

The dev container uses its own Compose project, `gantt-stop-me-dev`, so its containers and data stay separate from the stack in [Run the app with Docker](#run-the-app-with-docker). Its settings are in `.devcontainer/docker-compose.yml` and the entries in `.vscode/launch.json`, not `.env`. Only `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` come from your environment or the repo-root `.env`.

The **API**, **Web** and **Web (Chrome debug)** entries set `PUBLIC_URL` to `http://localhost:5173`, and Vite's proxy passes it to the API as the `Origin` of writes, so GitHub sign-in comes back to Vite. The **API** entry also sets `RUNNER_URL` and the GitHub OAuth App. The container's shell sets none of these, so a `docker compose up` run there doesn't pass them to the host stack. Vite started there by hand needs `PUBLIC_URL=http://localhost:5173 npm run dev`; the API started by hand needs the variables its entry sets. To use GitHub sign-in, add `http://localhost:5173/api/github/callback` to the OAuth App's callback URLs (see [Link GitHub](#link-github)), set the two GitHub variables, and run **Dev Containers: Rebuild Container** so the API entry gets them.

Run and Debug entries:

| Entry | What it starts |
| --- | --- |
| **Full stack** | DB, API and Web together. Stopping one stops all. Start here. |
| **DB** | The Postgres container, tailing its logs. Stopping it stops the container. |
| **API** | The Go API under the Delve debugger on :8080. Starts the DB first. |
| **Runner** | The test runner under Delve on :8081, using the host's Docker, with prefix `gantt-dev`. Full stack doesn't start it. |
| **Web** | Vite on :5173 with hot reload, forwarding `/api` to :8080. Opens your browser. |
| **Web (Chrome debug)** | As Web, with Chrome attached for breakpoints. Needs Chrome on the host. |

> **After pulling changes to `.devcontainer/`**, run **Dev Containers: Rebuild Container**. A running container never picks up new settings, and a stale one can stop the API from starting (see [Troubleshooting](#troubleshooting)).

## Frontend only

To work on the UI with hot reload outside the dev container, run Vite on your machine. This one needs Node. Its screens call the API through `/api`, which Vite proxies to `localhost:8080`, so keep the Docker stack running too (`docker compose up --build`): nginx on :8080 forwards `/api` to the API.

```sh
cd src/front
npm install
npm run dev      # http://localhost:5173, hot reload
```

| Command | Does |
| --- | --- |
| `npm run dev` | Dev server with hot reload, proxying `/api` to :8080 |
| `npm test` | Vitest and Testing Library against a fake API (`npm run test:watch` to keep it running) |
| `npm run lint` | oxlint |
| `npm run build` | Type check and production build |

Keep the stack's `PUBLIC_URL` at `http://localhost:8080` while you do. Vite's proxy sends `PUBLIC_URL` from its own environment as the `Origin` of `/api` writes, or `http://localhost:8080` when your shell doesn't set it. After GitHub sign-in, GitHub sends you back to :8080, the stack's build. With the dev container, run Vite there from its **Web** entry instead: VS Code forwards the container's :5173, so Vite on the host lands on another port, where GitHub sign-in can't come back.

[`src/front/README.md`](src/front/README.md) covers the frontend: its data hooks, tests, themes and routes.

## Tests

Backend tests run in containers, so they need only Docker.

```sh
# API tests. Run after every change to src/back/api.
docker compose run --rm api-test

# One package or test
docker compose run --rm api-test go test ./internal/accounts -run Login

# Any other go command
docker compose run --rm --no-deps api-test go mod tidy

# Runner tests against real Docker. Run after changes to the runner or to how automation drives a run.
docker compose --profile test run --rm runner-test
```

- Each API endpoint test starts the whole API with its own migrated schema in `test-db`.
- `test-db` stays up between runs. `docker compose --profile test down` stops it.
- Each command is done when it exits 0.
- A live test against a real LLM provider skips without a key. [`AGENTS.md`](AGENTS.md#tests) has the command.

Frontend tests run in a container too:

```sh
# Vitest. Run after every change to src/front.
docker compose run --rm web-test

# Any npm command, such as the linter
docker compose run --rm web-test npm run lint

# Type check and production build
docker compose build web
```

## Configuration

Every setting has a dev default in `docker-compose.yml`. To override a few, put just those lines in a `.env` at the repo root: everything it leaves out keeps its default. To start from the full list, copy the example file and edit it:

```sh
cp .env.example .env
```

Once you copy it, **you must set `ENCRYPTION_KEY` and `RUNNER_TOKEN`**. While they're blank, the API and runner refuse to start rather than fall back to the public dev values. Generate each with:

```sh
openssl rand -base64 32
```

| Variable | Purpose |
| --- | --- |
| `PUBLIC_URL` | The URL users open. The API accepts writes only from this origin, and builds the GitHub callback, the webhook URL and the links it posts to GitHub from it. Keep `http://localhost:8080` when running Vite on the host ([Frontend only](#frontend-only)). The dev container doesn't read it: its Run and Debug entries set `http://localhost:5173`. |
| `DATABASE_URL` | Postgres connection string for the API. |
| `ENCRYPTION_KEY` | 32 random bytes, base64. Encrypts stored GitHub credentials, provider keys and webhook secrets. |
| `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` | A GitHub OAuth App for GitHub sign-in, with callback `{PUBLIC_URL}/api/github/callback`. It asks for the `repo` and `workflow` scopes, and `user:email` to sign in. Leave both empty to link with personal access tokens and sign in with email only. See [Link GitHub](#link-github). |
| `ALLOW_PRIVATE_PROVIDER_URLS` | `true` lets LLM provider URLs reach local addresses, such as a local Ollama. |
| `RUNNER_URL` | Where the API reaches the test runner. Defaults to the `runner` service. Point it at a runner on another machine to keep test containers off this one. |
| `RUNNER_TOKEN` | Shared secret between the API and the runner. |
| `MAX_CONCURRENT_WORKFLOWS` | Workflow executions running at once. Default 1. |
| `RUN_PREFIX` | Prefix for everything the runner creates in Docker. Default `gantt`. 2 to 32 lowercase letters, digits or `-`, starting with a letter and ending in a letter or digit. |

[`.env.example`](.env.example) has the details for each.

## Link GitHub

Every project needs a linked GitHub account. Link one in the app under **Settings → GitHub**, with GitHub sign-in or a personal access token. Or use **Continue with GitHub** on the sign-in or sign-up page: it signs you in, or makes your account, and links GitHub at once.

Linking with GitHub sign-in in Settings asks for your password first when you signed in, or last entered it there, over 10 minutes ago: a linked GitHub account signs you in, so someone holding a stolen session can't add theirs as a way into your account. An account with no password isn't asked.

### GitHub sign-in

**Link with GitHub** in Settings and **Continue with GitHub** need an OAuth App, registered once per install. Without one, Settings offers only the token, and the sign-in page only email and password. Both come back to the one callback URL below, so signing in needs no other.

1. Open https://github.com/settings/applications/new.
2. Set **Homepage URL** to `PUBLIC_URL`, such as `http://localhost:8080`.
3. Set **Authorization callback URL** to `{PUBLIC_URL}/api/github/callback`, such as `http://localhost:8080/api/github/callback`. For the dev container, also add `http://localhost:5173/api/github/callback`: an app takes up to 10 callback URLs.
4. Leave **Expire user access tokens** on. The API refreshes the tokens before they expire.
5. Register the app, then generate a client secret on its page.
6. Put both in `.env` at the repo root:

   ```sh
   GITHUB_CLIENT_ID=<the app's client ID>
   GITHUB_CLIENT_SECRET=<the client secret>
   ```

   A `.env` holding only these two lines keeps the dev defaults for everything else. One copied from `.env.example` needs `ENCRYPTION_KEY` and `RUNNER_TOKEN` too (see [Configuration](#configuration)).
7. Recreate the API container so it reads them: `docker compose up -d api`. In the dev container, run **Dev Containers: Rebuild Container** instead.

The app asks for the `repo` and `workflow` scopes. Automation merges pull requests, and GitHub refuses to merge one that changes `.github/workflows` with a token that lacks `workflow`. Continue with GitHub also asks for `user:email`, to read your GitHub account's verified primary email.

Continue with GitHub signs you in as the user your GitHub account is linked to. When none is, it makes one with that email, linked to the account, with no password until you set one in **Settings → Account**. Until then you can't unlink GitHub. It won't sign you in to an account that already has that email: sign in to that one with its password, then link GitHub in Settings.

If your repositories belong to an organization that restricts OAuth Apps, an organization owner has to approve the app. Request it on GitHub under **Settings → Applications → Authorized OAuth Apps**: open the app, then **Request access** next to the organization.

### Personal access token

Paste one into **Or use a personal access token**. Make a fine-grained token: set **Resource owner** to whoever owns your repositories, give it access to them, and give it these repository permissions. Settings links to GitHub's token page with them filled in.

| Permission | Access |
| --- | --- |
| Contents | Read and write |
| Issues | Read and write |
| Pull requests | Read and write |
| Workflows | Read and write, to merge changes to workflow files |
| Metadata | Read-only, added automatically |

A classic token needs the `repo` and `workflow` scopes.

## Repository layout

Only what has to be at the root is there.

```
├── README.md               this file
├── LICENSE                 MIT
├── AGENTS.md               working notes for AI coding agents
├── docker-compose.yml      every service: db, api, runner, web, and the test profile
├── .env.example            every setting, documented; copy it to .env to override
├── .gitignore              .env, debug builds and other local files
├── .devcontainer/          the VS Code dev container, layered on docker-compose.yml
├── .vscode/                Run and Debug entries, and the Dev Containers recommendation
├── .impeccable/            config.json for the impeccable design tool; its edit hook reads it only here
├── .gantt/                 workflows.yml, this repo's test workflows; the app reads it at a repository's root
├── docs/
│   ├── README.md           index of every doc
│   ├── architecture.md     how the frontend, API, runner and database fit together
│   ├── glossary.md         the app's terms
│   ├── gaps.md             features not built yet
│   └── api/                every endpoint, one file per feature
└── src/
    ├── front/              React + TypeScript (Vite), served by nginx in Docker
    │   ├── PRODUCT.md      product context for design work
    │   ├── DESIGN.md       the design system
    │   ├── .impeccable/    DESIGN.md's sidecar and the surface briefs
    │   └── src/            the app
    └── back/api/           Go module
        ├── cmd/api/        the API
        ├── cmd/runner/     the test runner
        ├── internal/       accounts, github, board, projects, planning, automation, runner, ...
        └── migrations/     SQL, embedded in the binary and run at startup
```

## Troubleshooting

**`localhost:5173` hangs in the browser (Windows).** Vite was listening only on IPv6, which VS Code's port forwarding can't reach. `vite.config.ts` binds `127.0.0.1` to prevent this, so pull the latest code and restart the **Web** entry. If it still hangs, remove port 5173 in VS Code's **Ports** panel and forward it again.

**`504 (Outdated Optimize Dep)` in the browser console.** Vite's dependency cache is out of sync, usually because two Vite processes shared it. Stop Vite, delete the cache, start Vite again, then hard-reload with Ctrl+Shift+R:

```sh
rm -rf src/front/node_modules/.vite
```

**The API exits with `ENCRYPTION_KEY: secret: key must be 32 bytes`.** The key is missing. In the dev container, the container is older than its config: run **Dev Containers: Rebuild Container**. With Docker on the host, either delete `.env` to use the defaults or set `ENCRYPTION_KEY` in it.

**`/api` requests return 502.** The API isn't running. In the dev container, start the **API** entry (or **Full stack**) and check its Debug Console for the error. With Docker on the host, check `docker compose logs api`.

**Saves fail with `403 bad_origin`.** The `Origin` the API gets isn't `PUBLIC_URL`. Open the app at `PUBLIC_URL`, or through Vite on :5173, whose proxy sends `PUBLIC_URL` from its own environment, else `http://localhost:8080`. On the host, that means leaving the stack's `PUBLIC_URL` at `http://localhost:8080` and Vite's unset. Against the dev container's **API** entry, which expects `http://localhost:5173`, start Vite from a **Web** entry, or by hand in the container's terminal with `PUBLIC_URL=http://localhost:5173 npm run dev`.

**GitHub says "The redirect_uri is not associated with this application."** The OAuth App's callback URL must be `{PUBLIC_URL}/api/github/callback` exactly, scheme and port included: `http://localhost:8080/api/github/callback` for the Docker stack, `http://localhost:5173/api/github/callback` for the dev container. Add the one you need in the app's settings on GitHub.

**Settings or the sign-in page says "GitHub sign-in isn't set up on this server."** The API has no OAuth App. Set both `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` (see [Link GitHub](#link-github)), then recreate the API: `docker compose up -d api`. In the dev container, run **Dev Containers: Rebuild Container** instead. Or link with a personal access token.

**Settings says "GitHub didn't link.", or the sign-in page "GitHub sign-in didn't work."** The API logs why, such as a callback URL GitHub doesn't accept, which the log names. With Docker on the host, check `docker compose logs api`. In the dev container, check the **API** entry's Debug Console.

**Back from GitHub, the browser shows a 404 on `localhost:8080` (dev container).** The API ran with `PUBLIC_URL` at `http://localhost:8080`, where the dev container serves only the API. Start it from the **API** entry, which sets `http://localhost:5173`, or by hand with `PUBLIC_URL=http://localhost:5173`.

**The API panics with `PUBLIC_URL must be an absolute URL` (dev container).** It started without `PUBLIC_URL`, which only the **API** entry sets there. Start it from that entry, or by hand with `PUBLIC_URL=http://localhost:5173`.

**"Port in use" when starting an entry.** Something else is already on 5173, 8080 or 8081, often a server from an earlier session. Stop it, then try again:

```sh
pkill -f vite
pkill -f 'cmd/api'
pkill -f 'cmd/runner'
```

**Frontend edits don't show up.** On some Docker and WSL setups, Vite misses file changes. Restart the **Web** entry, then hard-reload.

**Start over with an empty database.** For the Docker stack, run `docker compose down -v`. For the dev container, remove only the DB container and its data volume. Don't run `down -v` there, because it also removes the dev container itself. The next DB start creates a fresh database:

```sh
docker rm -f gantt-stop-me-dev-db-1
docker volume rm gantt-stop-me-dev_db-data
```

## Further reading

- [`docs/README.md`](docs/README.md): an index of every doc.
- [`docs/architecture.md`](docs/architecture.md): how the frontend, API, runner and database fit together.
- [`docs/glossary.md`](docs/glossary.md): the app's terms.
- [`docs/api/`](docs/api/README.md): every API endpoint, one file per feature, and the conventions they share.
- [`docs/gaps.md`](docs/gaps.md): features not built yet.
- [`src/front/README.md`](src/front/README.md): the frontend guide.
- [`src/front/PRODUCT.md`](src/front/PRODUCT.md): product context for design work.
- [`src/front/DESIGN.md`](src/front/DESIGN.md): the frontend design system.
- [`AGENTS.md`](AGENTS.md): working notes for AI coding agents in this repo.

## License

[MIT](LICENSE)
