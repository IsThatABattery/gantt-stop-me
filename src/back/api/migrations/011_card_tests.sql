alter table tasks
  add column tests        jsonb not null default '[]', -- the card's tests, the source of truth
  add column tests_synced text,                        -- the issue's Tests section as last read or written; null = never seen
  add column tests_error  text;                        -- why GitHub's last edit to the section was not taken; null when it was

-- A sync reads only issues changed since the last one: the flag makes each project's syncs read every issue until
-- one gets through, so tasks from before this take their issue's tests.
update projects set backfill_pending = true where synced_at is not null;
