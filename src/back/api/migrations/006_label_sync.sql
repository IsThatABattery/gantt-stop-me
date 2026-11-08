alter table projects add column labels_etag text;
-- 005 gained this column after it first shipped, so databases that ran the first 005 lack it.
alter table projects add column if not exists backfill_pending boolean not null default false;

-- Labels stored before this have no id, which the label sync goes by: the flag makes each project's syncs re-read every issue.
update projects set backfill_pending = true where synced_at is not null;
