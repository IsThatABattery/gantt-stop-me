alter table tasks
  add column body   text not null default '',   -- cache, overwritten by sync
  add column labels jsonb not null default '[]'; -- the issue's labels as [{"name", "color"}], cache

alter table projects add column backfill_pending boolean not null default false;

-- Tasks from before this have no body or labels: the flag makes each project's syncs read every issue until one gets through.
update projects set backfill_pending = true where synced_at is not null;
