create table providers (
  id       bigserial primary key,
  user_id  bigint not null references users on delete cascade,
  name     text not null,
  base_url text not null,  -- no trailing slash
  key_enc  bytea not null, -- API key sealed with secret.Box
  models   text[] not null default '{}',
  unique (user_id, name)
);
