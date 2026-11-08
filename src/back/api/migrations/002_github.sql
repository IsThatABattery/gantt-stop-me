create table github_links (
  user_id        bigint primary key references users on delete cascade,
  github_id      bigint not null,
  login          text not null,
  credential_enc bytea not null, -- secret.Box-sealed OAuth token or personal access token
  kind           text not null,  -- 'oauth' or 'pat'
  linked_at      timestamptz not null default now()
);
