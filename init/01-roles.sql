CREATE ROLE finance_owner LOGIN PASSWORD 'finance_owner_local_dev_only' NOSUPERUSER NOBYPASSRLS;
CREATE ROLE finance_app LOGIN PASSWORD 'finance_app_local_dev_only';

ALTER DATABASE finance OWNER TO finance_owner;

GRANT CONNECT ON DATABASE finance TO finance_owner;
GRANT CONNECT ON DATABASE finance TO finance_app;
GRANT USAGE ON SCHEMA public TO finance_app;
