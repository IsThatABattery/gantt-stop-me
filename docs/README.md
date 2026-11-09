# Docs

Every doc in the repository, and what it covers.

## Start here

- [README.md](../README.md): running the app, the dev container, tests, configuration, the repository layout and troubleshooting.
- [AGENTS.md](../AGENTS.md): working notes for AI coding agents: starting the stack, the dev container, tests, the API-docs rule and where design files go.

## System

- [architecture.md](architecture.md): how the frontend, API, runner and database fit together.
- [glossary.md](glossary.md): the app's terms.
- [gaps.md](gaps.md): features not built yet.

## API

One file per feature, each endpoint with its name, one sentence, method and path, input table and output table. Change an endpoint and its doc in the same commit.

- [api/README.md](api/README.md): conventions every endpoint shares (session, origin, errors), health, and an index of the files below.
- [api/accounts.md](api/accounts.md): sign up, log in and out, the current user, display name and password.
- [api/github.md](api/github.md): link a GitHub account with OAuth or a personal access token, and unlink it.
- [api/providers.md](api/providers.md): the user's LLM providers, their keys and chosen models.
- [api/board.md](api/board.md): projects, the board, tasks, their tests, assignees and moves.
- [api/planning.md](api/planning.md): planning settings, and plans from interview to proposed tasks.
- [api/automation.md](api/automation.md): automation, workflows, the webhook, test runs, approve and merge, request changes, taking a task over, and the formats they read.

## Frontend

- [src/front/README.md](../src/front/README.md): the frontend guide: running it, data hooks, tests, layout, themes and routes.
- [src/front/PRODUCT.md](../src/front/PRODUCT.md): product context for design work: users, purpose, positioning and principles.
- [src/front/DESIGN.md](../src/front/DESIGN.md): the design system: colors, type, layout, shapes and components.
- [src/front/.impeccable/surfaces/](../src/front/.impeccable/surfaces/): impeccable's surface briefs, each the design brief for a screen and the surfaces it reaches.
