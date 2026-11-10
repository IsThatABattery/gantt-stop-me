-- Setting a first password needs a session started in the last few minutes.
alter table sessions add column created_at timestamptz not null default now();
-- Sessions last 30 days, so one from before this started 30 days before it expires, not now.
update sessions set created_at = expires_at - interval '30 days';
