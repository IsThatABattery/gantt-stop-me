-- Signing in with GitHub finds the user by their GitHub account, and makes one with no password when none is linked
-- to it, until its user sets one.
alter table users alter column password_hash drop not null; -- null: no password

create index on github_links (github_id);
