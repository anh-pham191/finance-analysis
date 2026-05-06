BEGIN;
SELECT set_config('app.user_id', '1', true);

INSERT INTO users (id, email, display_name)
OVERRIDING SYSTEM VALUE
VALUES (1, 'local@finance-analysis', 'Local Dev')
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  display_name = EXCLUDED.display_name;

SELECT setval(pg_get_serial_sequence('users', 'id'), (SELECT COALESCE(MAX(id), 1) FROM users));

INSERT INTO categories (user_id, name, kind)
VALUES (1, 'Uncategorised', 'expense')
ON CONFLICT (user_id, name) DO NOTHING;

COMMIT;
