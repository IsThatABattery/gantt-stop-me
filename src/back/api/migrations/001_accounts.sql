create extension if not exists citext with schema public;

create table users (
  id            bigserial primary key,
  email         citext unique not null,
  password_hash text not null, -- argon2id, PHC string
  created_at    timestamptz not null default now()
);

create table sessions (
  id         text primary key, -- sha256 of the cookie value, hex
  user_id    bigint not null references users on delete cascade,
  expires_at timestamptz not null
);

create index on sessions (user_id);
