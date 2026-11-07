create table projects (
  id             bigserial primary key,
  github_repo_id bigint unique not null,
  owner          text not null,
  name           text not null,
  added_by       bigint references users on delete set null,
  issues_etag    text,        -- ETag of the issue list the last sync applied
  synced_at      timestamptz, -- last sync; null until initialization finishes
  created_at     timestamptz not null default now()
);

create table project_members (
  project_id bigint references projects on delete cascade,
  user_id    bigint references users on delete cascade,
  permission text not null, -- 'read', 'write' or 'admin', as GitHub last reported it
  checked_at timestamptz not null,
  primary key (project_id, user_id)
);

create table columns (
  id           bigserial primary key,
  project_id   bigint not null references projects on delete cascade,
  key          text not null, -- 'backlog', 'in-progress', 'testing', 'user-check', 'closed'
  name         text not null,
  position     integer not null,
  closes_issue boolean not null default false,
  unique (project_id, key)
);

create table tasks (
  id              bigserial primary key,
  project_id      bigint not null references projects on delete cascade,
  issue_number    integer not null,
  github_issue_id bigint unique not null,
  column_id       bigint not null references columns,
  position        double precision not null,
  version         integer not null default 1,
  title           text not null, -- cache, overwritten by sync
  state           text not null, -- 'open' or 'closed', cache
  updated_at      timestamptz not null default now(),
  unique (project_id, issue_number)
);
