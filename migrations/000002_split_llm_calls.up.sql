-- Split run_steps into two layers:
--   run_steps  = step boundaries (one row per agent action: planner, retriever[i],
--                synthesizer, critic), owned by the orchestrator.
--   llm_calls  = raw model calls (one row per call; many per step on retry/re-prompt),
--                owned by the provider client. Carries the reproducibility detail.
--
-- NOTE: this DROPs columns from run_steps. Any rows already logged lose their
-- prompt/raw/token/cost values. run_steps is empty until step logging is wired, so
-- in practice no data is lost — but the down migration cannot restore those values.

-- Layer 2: raw LLM calls -----------------------------------------------------
CREATE TABLE IF NOT EXISTS llm_calls (
    id          BIGSERIAL PRIMARY KEY,
    step_id     BIGINT NOT NULL REFERENCES run_steps(id) ON DELETE CASCADE,
    model       TEXT   NOT NULL,
    prompt      TEXT,
    raw         TEXT,                          -- raw model text, pre-validation
    tokens_in   INT,
    tokens_out  INT,
    cost_usd    NUMERIC,
    latency_ms  INT,
    attempt     INT    NOT NULL DEFAULT 1,     -- 1-based; >1 = retry (e.g. after 429)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS llm_calls_step_idx ON llm_calls (step_id);

-- Layer 1: run_steps becomes pure step boundaries ----------------------------
-- created_at already records step start; add completion + failure, drop call detail.
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ;
ALTER TABLE run_steps ADD COLUMN IF NOT EXISTS error       TEXT;

ALTER TABLE run_steps DROP COLUMN IF EXISTS prompt;
ALTER TABLE run_steps DROP COLUMN IF EXISTS raw;
ALTER TABLE run_steps DROP COLUMN IF EXISTS tokens_in;
ALTER TABLE run_steps DROP COLUMN IF EXISTS tokens_out;
ALTER TABLE run_steps DROP COLUMN IF EXISTS cost_usd;
