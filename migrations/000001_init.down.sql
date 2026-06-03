-- Reverse of 000001_init.up.sql. Drop dependents before their parents, and the
-- vector extension last (chunks.embedding depends on it). Indexes drop with their tables.

DROP TABLE IF EXISTS evals;
DROP TABLE IF EXISTS run_steps;
DROP TABLE IF EXISTS runs;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS papers;

DROP EXTENSION IF EXISTS vector;
