-- 0001: базові розширення та схема ядра DELMOS.
--
-- Розширення мають бути доступні в інсталяції PostgreSQL (див. make db-setup).
-- Якщо їх уже створив DBA з правами суперкористувача, ці команди є no-op.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE SCHEMA IF NOT EXISTS core;

COMMENT ON SCHEMA core IS 'Реляційні сутності ядра DELMOS (ADR-001: All-in-PostgreSQL).';
