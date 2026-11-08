create table project_automation (
  project_id         bigint primary key references projects on delete cascade,
  owner_id           bigint not null references users on delete cascade,
  decision_provider  bigint not null references providers on delete cascade,
  decision_model     text not null,
  chat_provider      bigint not null references providers on delete cascade,
  chat_model         text not null,
  paused_reason      text,               -- null while running
  webhook_secret_enc bytea,              -- sealed with secret.Box
  pulls_etag         text,
  enabled_at         timestamptz not null default now()
);

-- Last seen state of each open PR, so the poller and webhook act on each change once.
create table pull_requests (
  project_id    bigint references projects on delete cascade,
  number        integer,
  head_sha      text not null,
  head_repo_id  bigint,                  -- null when the fork was deleted
  author_id     bigint not null,         -- GitHub user id
  draft         boolean not null,
  linked_issues integer[] not null default '{}',
  refused       boolean not null default false, -- the "not run" comment was posted
  primary key (project_id, number)
);

create table test_runs (
  id          bigserial primary key,
  task_id     bigint not null references tasks on delete cascade,
  pr_number   integer,                   -- null when no linked PR was found
  head_sha    text,
  status      text not null,             -- 'queued', 'running', 'passed', 'failed', 'error', 'cancelled'
  reason      text,                      -- runner error or cancel reason for the whole run
  approved_by bigint references users on delete set null,
  approved_at timestamptz,
  created_at  timestamptz not null default now(),
  finished_at timestamptz
);

create table workflow_executions (
  id          bigserial primary key,
  run_id      bigint not null references test_runs on delete cascade,
  project_id  bigint not null references projects on delete cascade,
  workflow    text not null,
  spec        jsonb not null,            -- as sent to the runner
  status      text not null,             -- 'queued', 'running', 'passed', 'failed', 'error', 'cancelled'
  attempts    integer not null default 0,
  log_tail    text not null default '',  -- last 64 KB
  started_at  timestamptz,
  finished_at timestamptz
);
create index on workflow_executions (id) where status = 'queued';

create table test_results (
  id            bigserial primary key,
  run_id        bigint not null references test_runs on delete cascade,
  execution_id  bigint references workflow_executions on delete cascade, -- null when routing refused it
  position      integer not null,        -- order in the Tests section
  text          text not null,           -- the line's text and its block
  kind          text not null,           -- 'script' or 'llm'
  pinned        boolean not null,
  probabilities jsonb not null default '{}', -- workflow name to probability
  status        text not null,           -- 'pending', 'passed', 'failed', 'manual', 'error', 'skipped'
  detail        text not null default '',    -- exit code and output, or the model's reason
  transcript    jsonb,                   -- an LLM check's tool calls and answers, cut to 64 KB
  screenshot    bytea
);

alter table tasks add column needs_attention text; -- a runner error's reason; cleared when a run starts or the task leaves Testing
