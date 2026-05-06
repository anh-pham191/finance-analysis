CREATE TABLE category_assignments (
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  txn_id text NOT NULL,
  category_id bigint NOT NULL,
  source text NOT NULL CHECK (source IN ('RULE','MANUAL','AKAHU')),
  rule_id bigint,
  assigned_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, txn_id),
  FOREIGN KEY (user_id, txn_id) REFERENCES transactions(user_id, id) ON DELETE CASCADE,
  FOREIGN KEY (user_id, category_id) REFERENCES categories(user_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (user_id, rule_id) REFERENCES rules(user_id, id) ON DELETE SET NULL (rule_id)
);

CREATE INDEX category_assignments_user_idx ON category_assignments (user_id);

ALTER TABLE category_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE category_assignments FORCE ROW LEVEL SECURITY;

CREATE POLICY category_assignments_tenant_isolation ON category_assignments
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
