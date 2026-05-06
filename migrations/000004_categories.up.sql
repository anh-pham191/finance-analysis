CREATE TABLE categories (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  parent_id bigint,
  kind text NOT NULL CHECK (kind IN ('income','expense','transfer')),
  UNIQUE (user_id, id),
  UNIQUE (user_id, name),
  FOREIGN KEY (user_id, parent_id) REFERENCES categories(user_id, id) ON DELETE SET NULL (parent_id)
);

CREATE INDEX categories_user_id_idx ON categories (user_id);

ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories FORCE ROW LEVEL SECURITY;

CREATE POLICY categories_tenant_isolation ON categories
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
