CREATE TABLE rules (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  priority int NOT NULL,
  predicate jsonb NOT NULL DEFAULT '{}'::jsonb,
  category_id bigint NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, id),
  UNIQUE (user_id, name),
  FOREIGN KEY (user_id, category_id) REFERENCES categories(user_id, id) ON DELETE RESTRICT
);

CREATE INDEX rules_user_id_idx ON rules (user_id);

ALTER TABLE rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE rules FORCE ROW LEVEL SECURITY;

CREATE POLICY rules_tenant_isolation ON rules
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
