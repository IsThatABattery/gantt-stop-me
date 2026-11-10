-- Linking GitHub with OAuth, a way to sign in, needs a session started, or that confirmed its user's password, in the
-- last few minutes.
alter table sessions add column confirmed_at timestamptz; -- null until the session confirms the password
