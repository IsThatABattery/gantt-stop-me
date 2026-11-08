alter table users add column name text; -- display name, null until set

alter table tasks
  add column issue_node_id    text,                        -- the issue's GraphQL id; null until a sync or a read of it
  add column assignees        jsonb not null default '[]', -- the issue's assignees as [{"login", "avatarUrl"}], cache
  add column pull_requests    jsonb not null default '[]', -- the pull requests closing the issue, as Get task shows them, cache
  add column pull_requests_at timestamptz,                 -- when GitHub last listed them; null = never
  add column taken_over_by    bigint references users on delete set null; -- the member who took it over from automation

-- A sync reads only issues changed since the last one: the flag makes each project's syncs read every issue until
-- one gets through, so tasks from before this take their issue's assignees.
update projects set backfill_pending = true where synced_at is not null;
