# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

See @README.md for the full spec. This file pins the constraints that are easy to violate.

## What this is

A multi-agent RAG research agent answering questions over an arXiv corpus with cited, grounded answers. Favor the simplest thing that ships end-to-end. Don't gold-plate.

This file describes the **target** design and required practices — not all of it is built yet. Current implementation state lives in @README.md (Status) and `TODO.md`; the topology, budget enforcement, and full per-step logging below are what to build toward.

## Hard constraints (do not violate without asking)

- **No agent framework.** The orchestrator is hand-rolled Go — no LangChain, LlamaIndex, CrewAI, etc.
- **Go monolith.** Single Go service. Postgres + pgvector (vector) + Postgres FTS/BM25 (keyword) for **hybrid** retrieval. CLI only, no UI.
- **Scope limits:** one corpus slice · abstracts-first (defer full-text PDF parsing) · 2–4 parallel retrievers · **max 2 retrieval rounds**. Topic is config (arXiv category + keyword filter) — keep agent code topic-agnostic.
- **Deferred — do not build:** citation graph, multi-topic, fancy reranking.

## Agent topology

Planner (question → sub-queries + filters) → 2–4 parallel Retriever workers (hybrid retrieve per sub-query) → Synthesizer (evidence → cited answer) → Critic/verifier (grounding check + gap detection → triggers re-retrieval, bounded). Orchestrator wires the loop, enforces a budget (max rounds / cost), and **logs every step in full**.

## Non-negotiable engineering practices

- Structured LLM outputs **with validation** — don't trust raw model text.
- **Full request logging** of every agent step (reproducibility is a known pain point — parallel retrievers + non-deterministic synthesis make runs hard to reproduce).
- Evals + thumbs-up/down + comment on every answer.
- LLM access through a **thin provider client** (Anthropic / OpenAI) — keep provider choice swappable, don't hardcode one SDK across the agents.
