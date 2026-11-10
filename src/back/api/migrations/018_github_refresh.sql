-- An OAuth App with "Expire user access tokens" on issues tokens that last 8 hours, with a refresh token for the next.
alter table github_links
  add column refresh_enc bytea,       -- secret.Box-sealed OAuth refresh token; null when credential_enc does not expire, or once GitHub refused it
  add column expires_at  timestamptz; -- when credential_enc expires; null when it does not
