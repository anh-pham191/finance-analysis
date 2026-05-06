CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
  id bigserial PRIMARY KEY,
  email citext NOT NULL UNIQUE,
  display_name text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX users_email_idx ON users (email);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

CREATE POLICY users_self_access ON users
  USING (id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (id = current_setting('app.user_id', true)::bigint);
