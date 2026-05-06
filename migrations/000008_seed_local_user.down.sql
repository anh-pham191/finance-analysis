BEGIN;
SELECT set_config('app.user_id', '1', true);

DELETE FROM categories WHERE user_id = 1 AND name = 'Uncategorised';
DELETE FROM users WHERE id = 1;

COMMIT;
