CREATE TABLE sync_state (
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  account_id text NOT NULL,
  last_synced_at timestamptz,
  last_cursor text,
  PRIMARY KEY (user_id, account_id),
  FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id) ON DELETE CASCADE
);

ALTER TABLE sync_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE sync_state FORCE ROW LEVEL SECURITY;

CREATE POLICY sync_state_tenant_isolation ON sync_state
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
