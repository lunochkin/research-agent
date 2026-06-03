-- Reverse of 000002_split_llm_calls.up.sql: fold llm_calls back into run_steps.
-- The moved values are NOT restored (llm_calls rows are dropped, not merged) —
-- the columns come back empty. Acceptable: this is dev-only rollback.

-- Restore the call-detail columns on run_steps.
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS prompt     TEXT;
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS raw        TEXT;
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS tokens_in  INT;
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS tokens_out INT;
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS cost_usd   NUMERIC;

-- Drop the step-boundary columns added by the up migration.
ALTER TABLE run_steps DROP COLUMN IF EXISTS finished_at;
ALTER TABLE run_steps DROP COLUMN IF EXISTS error;

-- Drop Layer 2 (index drops with the table).
DROP TABLE IF EXISTS llm_calls;
