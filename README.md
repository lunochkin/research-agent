# Multi-Agent RAG Research Agent (arXiv)

A CLI research agent that answers questions over an arXiv corpus with **cited,
grounded** answers. It plans, retrieves in parallel, synthesizes, and verifies —
a hand-rolled multi-agent loop in Go, no agent framework.

## What it does

Ask a research question → get a synthesized answer with citations back to the
arXiv papers it used. Retrieval is hybrid (semantic + keyword); answers are
grounded in retrieved chunks and validated before returning.

## Architecture

Hand-rolled Go orchestrator — no LangChain / LlamaIndex / CrewAI.

- **Planner** — decomposes the question into 2–4 sub-queries + filters.
- **Retrievers (2–4, parallel)** — each runs hybrid retrieval for one sub-query.
- **Synthesizer** — composes a cited answer from the fused evidence.
- **Critic** *(planned)* — grounding check + gap detection → bounded re-retrieval (max 2 rounds).
- **Orchestrator** — wires the agents and records each run. The bounded re-retrieval loop, budget enforcement (max rounds / cost), and per-step logging are planned (see Status).

## Retrieval

Hybrid over Postgres:

- **Vector** — pgvector (cosine) over embedded abstracts.
- **Keyword** — Postgres full-text search (FTS).
- **Fusion** — Reciprocal Rank Fusion (RRF) merges the two ranked lists.

## Stack

- Go monolith, CLI (no UI).
- Postgres + pgvector + FTS.
- LLMs through a thin, swappable provider client (Anthropic / OpenAI).
- Structured outputs with validation · per-run request logging (per-step planned) · evals (thumbs + comment).

## Corpus

arXiv metadata + abstracts (Kaggle dump). Topic is config — an arXiv category
filter + keyword filter; agent code stays topic-agnostic. Abstracts-first
(full-text PDF parsing is out of scope).

## Setup

Prerequisites: Go 1.26+, Docker (or local Postgres 16 with pgvector), an OpenAI
and/or Anthropic API key.

1. **Config**
   ```
   cp .env.example .env
   # set OPENAI_API_KEY (embeddings + optional generation) and/or ANTHROPIC_API_KEY
   ```
2. **Database**
   ```
   make db        # start postgres + pgvector (docker-compose)
   make migrate   # apply schema
   ```
3. **Ingest** a Kaggle arXiv metadata dump (JSON-lines)
   ```
   make ingest FILE=data/arxiv-metadata-oai-snapshot.json
   ```
   Filters to the configured category + keyword, embeds, indexes. Re-runnable:
   already-ingested papers are skipped (`--force` to re-embed).
4. **Ask**
   ```
   make ask Q="latest ReAct pattern updates"
   ```
5. **Rate** an answer
   ```
   ./bin/research-agent eval --run <id> --thumb +1 --comment "..."
   ```

## Configuration (.env)

- `LLM_PROVIDER` — `anthropic` | `openai` (generation)
- `OPENAI_API_KEY` / `ANTHROPIC_API_KEY`
- `EMBED_PROVIDER` / `EMBED_MODEL` — embeddings (OpenAI; Anthropic has no embeddings API)
- `ARXIV_CATEGORIES` / `KEYWORD_FILTER` — the corpus slice
- `MAX_ROUNDS` / `MAX_COST_USD` — per-run budget

## Status

Work in progress. The core single-pass pipeline runs end-to-end: plan → parallel
hybrid retrieve → synthesize an answer with validated citations, with each run
recorded to Postgres.

Not yet implemented: the critic / grounding check and the bounded re-retrieval
loop (currently a single pass), budget and cost enforcement, and full per-step
logging. See `TODO.md`.
