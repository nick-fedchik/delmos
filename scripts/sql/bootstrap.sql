-- Початкове налаштування СУБД для DELMOS.
-- Виконує DBA з правами суперкористувача: make db-setup
--
-- Паролі ролей не задаються тут і ніколи не зберігаються в репозиторії:
-- для локального підключення використовується peer/trust автентифікація,
-- для мережевого — пароль задає DBA окремо (\password delmos).

\set ON_ERROR_STOP on

SELECT format('CREATE ROLE %I LOGIN', :'migrator_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'migrator_role') \gexec

SELECT format('CREATE ROLE %I LOGIN', :'app_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_role') \gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'db_name', :'migrator_role')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'db_name') \gexec

\connect :db_name

-- Розширення встановлює суперкористувач: vector не є trusted-розширенням.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS vector;

-- Робітнича роль не має права змінювати схему (SYSTEM_REQUIREMENTS §3.3).
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO :"app_role";
GRANT CONNECT ON DATABASE :"db_name" TO :"app_role";
