create table planning_settings (
  user_id     bigint primary key references users on delete cascade,
  provider_id bigint references providers,  -- a provider in use cannot be deleted; null with model for none
  model       text,
  review      boolean not null default true
);

create table feature_plans (
  id          bigserial primary key,
  project_id  bigint not null references projects on delete cascade,
  created_by  bigint references users on delete set null,
  title       text not null,
  description text not null,             -- the feature as first described
  status      text not null,             -- 'briefing', 'needs_review', 'approved', 'done', 'discarded'
  doc         jsonb not null,            -- [{key, title, body, state}], state 'covered' | 'asking' | 'not_yet'
  pending     text,                      -- the agent's next job: 'ask', 'draft', 'generate', 'revise'; null while it waits on a person
  feedback    text,                      -- a denial's feedback, for the next agent turn
  error       text,                      -- why the agent's last turn failed; cleared when one succeeds
  version     integer not null default 1,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now()
);
create index on feature_plans (project_id);

create table plan_messages (
  id         bigserial primary key,
  plan_id    bigint not null references feature_plans on delete cascade,
  role       text not null,              -- 'agent' or 'user'
  text       text not null,
  options    jsonb not null default '[]', -- an agent question's suggested answers
  skipped    boolean not null default false,
  created_at timestamptz not null default now()
);
create index on plan_messages (plan_id);
