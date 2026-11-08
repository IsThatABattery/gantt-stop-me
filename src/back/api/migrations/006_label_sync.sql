alter table projects add column labels_etag text;

-- Labels stored before this have no id, which the label sync goes by: the flag makes each project's syncs re-read every issue.
update projects set backfill_pending = true where synced_at is not null;
