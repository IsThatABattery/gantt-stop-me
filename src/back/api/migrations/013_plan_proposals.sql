create table plan_proposals (
  id                  bigserial primary key,
  plan_id             bigint not null references feature_plans on delete cascade,
  position            integer not null,  -- from 1, in the order the agent proposed them
  status              text not null,     -- 'needs_review', 'approved', 'approved_edited', 'sent_back', 'dropped'
  title               text not null,
  description         text not null,
  acceptance_criteria jsonb not null default '[]', -- [string]
  tests               jsonb not null default '[]', -- tasktests.Test list
  depends_on          integer[] not null default '{}', -- positions of earlier proposals in this plan
  source              text not null default '',    -- the doc section it came from, such as "Success criteria, item 2"
  agent_note          text not null default '',
  feedback            text,                        -- a denial's
  edited              boolean not null default false, -- a person edited it since the agent wrote it: approving it is 'approved_edited'
  task_id             bigint references tasks on delete set null, -- the task created on approval
  created_at          timestamptz not null default now(),
  unique (plan_id, position)
);
