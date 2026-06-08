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
- **Critic** — grounding check + gap detection → bounded re-retrieval (max 2 total rounds).
- **Orchestrator** — wires the agents, runs the bounded re-retrieval loop, enforces the budget (max rounds / cost), and records each run with full per-step logging.

## Retrieval

Hybrid over Postgres:

- **Vector** — pgvector (cosine) over embedded abstracts.
- **Keyword** — Postgres full-text search (FTS).
- **Fusion** — Reciprocal Rank Fusion (RRF) merges the two ranked lists.

## Stack

- Go monolith, CLI (no UI).
- Postgres + pgvector + FTS.
- LLMs through a thin, swappable provider client (Anthropic / OpenAI).
- Structured outputs with validation · full per-step + per-LLM-call request logging · evals (thumbs + comment).

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

Work in progress. The full loop is wired end-to-end: plan → parallel hybrid
retrieve → synthesize an answer with validated citations → critic grounding check
+ gap detection → bounded re-retrieval (max 2 total rounds), with budget
enforcement (max rounds / cost) and full per-step + per-LLM-call logging recorded
to Postgres.

The critic's grounding gate is not yet calibrated — it currently over-rejects, so
runs tend to use their full round budget. Tuning the critic and end-to-end
evaluation are the open work. See `TODO.md`.
