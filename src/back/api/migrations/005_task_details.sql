alter table tasks
  add column body   text not null default '',   -- cache, overwritten by sync
  add column labels jsonb not null default '[]'; -- the issue's labels as [{"name", "color"}], cache

-- Tasks from before this have no body or labels: an epoch cutoff and no ETag make each project's next sync read every issue once.
update projects set synced_at = 'epoch', issues_etag = null where synced_at is not null;
