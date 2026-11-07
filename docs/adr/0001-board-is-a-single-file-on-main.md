# The Board is a single file in the Project's repository

The Board needed a store that humans can edit locally and push alongside code, and that agents can read and write without a service in between. We keep it as one `BOARD.yaml` at the root of the Project's repository, and the file at the head of `main` is the only record of Board state: no database, no cache, every view is a stateless render of that file. Task branches may edit the file, so a pull request can carry new Features or notes and the merge brings them in; ordinary git conflicts are the price of concurrent edits, which a team of two to five accepts over running a server.

## Considered options

- One file per Feature in a `board/` directory: fewer conflicts and a per-Feature diff, but a less familiar shape than a single root file.
- A database with the file as an export: richer queries, but two sources of truth and a service to host.

## Consequences

- Fields have owners. Agents own `status`, `branch`, `pr` and anything derived from a run; humans own titles, descriptions, acceptance criteria and notes, and may add Features or Tasks on any branch. A human rarely needs to touch `status`, because pushing a task branch and opening its pull request already move it.
- Conflicts in `BOARD.yaml` are ordinary git conflicts, resolved by whoever is merging. The file keeps one Task per block so unrelated edits land in separate hunks. There is no bot that pushes into human branches and no custom merge driver.
- Features are keyed by a human-chosen Slug and Tasks are numbered under it (`login-page-1`), so the before-and-after diff of the file and the link from a `task/<id>` branch to its Task need no lookup table.
