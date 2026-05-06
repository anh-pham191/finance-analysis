CREATE TABLE transactions (
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  id text NOT NULL,
  account_id text NOT NULL,
  posted_at timestamptz NOT NULL,
  amount numeric(14,2) NOT NULL,
  direction text NOT NULL CHECK (direction IN ('DEBIT','CREDIT')),
  description text NOT NULL DEFAULT '',
  merchant text NOT NULL DEFAULT '',
  akahu_category text NOT NULL DEFAULT '',
  raw_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, id),
  FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id) ON DELETE CASCADE
);

CREATE INDEX transactions_user_posted_idx ON transactions (user_id, posted_at DESC);

ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions FORCE ROW LEVEL SECURITY;

CREATE POLICY transactions_tenant_isolation ON transactions
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
