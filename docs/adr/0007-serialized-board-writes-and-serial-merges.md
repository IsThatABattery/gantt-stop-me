# Board writes are serialized and guarded; merges land one at a time against a fresh main

Several jobs can fire within seconds of each other and all of them touch the same file or the same branch, so four races needed an answer.

- **Concurrent Board writes.** Every Board write loads the latest `main`, applies one transition, commits and pushes, with pull-rebase and retry on rejection; it is never a textual patch. All Board-writing steps share one Actions concurrency group per repository, queued rather than cancelled. Each transition is guarded by its source state in the ADR 0005 table: a job that finds the Task in any other state does nothing, and closed is terminal unless a human reopens it. A late job from an old push therefore cannot regress a status.
- **Stale merges.** The ruleset requires a branch to be up to date before merging, so a PR tested against an older `main` cannot land. GitHub's auto-merge does not update branches itself, so the push-to-main job calls the update-branch API for every approved PR with auto-merge enabled; that reruns its tests and auto-merge proceeds one PR at a time, each tested against the `main` it lands on. Merge queue is the upgrade if the repository's plan allows it.
- **Overlapping runs on one PR.** One concurrency group per pull request with cancel-in-progress, so only the head commit is tested and the sticky comment always describes it.
- **A red main.** The test commands also run on every push to `main`, with no Board move; the result shows as the commit's check and as a banner in the local tool.

The alternative of trusting git retries alone was rejected because out-of-order jobs would silently regress statuses, and accepting stale merges was rejected because a broken `main` would only surface on the next PR.
