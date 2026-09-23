-- Підготовка середовища інтеграційних тестів: make db-test-setup
--
-- Тестова роль отримує CREATEDB, щоб кожен тест створював власну тимчасову базу
-- (docs/testing/TEST_STRATEGY.md). Шаблон містить розширення, які не є trusted.

\set ON_ERROR_STOP on

SELECT format('CREATE ROLE %I LOGIN CREATEDB', :'test_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'test_role') \gexec

SELECT format('ALTER ROLE %I CREATEDB', :'test_role') \gexec

SELECT format('CREATE DATABASE delmos_test_template OWNER %I', :'test_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'delmos_test_template') \gexec

\connect delmos_test_template

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS vector;

ALTER DATABASE delmos_test_template WITH IS_TEMPLATE TRUE;
