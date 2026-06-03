-- P1 schema: corpus (papers + chunks), full request logging (runs + steps),
-- evals (thumbs + comment). Embedding dim 1536 = OpenAI text-embedding-3-small.

CREATE EXTENSION IF NOT EXISTS vector;

-- Corpus -------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS papers (
    id          TEXT PRIMARY KEY,        -- arXiv id
    title       TEXT NOT NULL,
    abstract    TEXT NOT NULL,
    authors     TEXT[],
    categories  TEXT[],
    published   DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chunks (
    id         BIGSERIAL PRIMARY KEY,
    paper_id   TEXT NOT NULL REFERENCES papers(id) ON DELETE CASCADE,
    seq        INT  NOT NULL,
    content    TEXT NOT NULL,
    embedding  vector(1536),
    tsv        tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    UNIQUE (paper_id, seq)
);

-- keyword (BM25-ish via ts_rank) + vector (cosine) indexes for hybrid retrieval
CREATE INDEX IF NOT EXISTS chunks_tsv_idx       ON chunks USING GIN  (tsv);
CREATE INDEX IF NOT EXISTS chunks_embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);

-- Full request logging -----------------------------------------------------
-- One run per question; one row per agent step, raw model text kept pre-validation.
CREATE TABLE IF NOT EXISTS runs (
    id          BIGSERIAL PRIMARY KEY,
    question    TEXT NOT NULL,
    config      JSONB,
    answer      TEXT,
    cost_usd    NUMERIC,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS run_steps (
    id         BIGSERIAL PRIMARY KEY,
    run_id     BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    agent      TEXT   NOT NULL,          -- planner|retriever|synthesizer|critic
    round      INT    NOT NULL DEFAULT 0,
    input      JSONB,
    output     JSONB,                    -- validated structured output
    prompt     TEXT,
    raw        TEXT,                     -- raw model text before validation
    tokens_in  INT,
    tokens_out INT,
    cost_usd   NUMERIC,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS run_steps_run_idx ON run_steps (run_id);

-- Evals --------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS evals (
    id         BIGSERIAL PRIMARY KEY,
    run_id     BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    thumb      SMALLINT,                 -- +1 / -1
    comment    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS evals_run_idx ON evals (run_id);
