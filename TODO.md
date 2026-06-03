# TODO

v1.0 — ship: cited, grounded answers over arXiv:
- [x] scaffold Go monolith + CLI skeleton
- [x] add Postgres + pgvector schema (papers, chunks, runs, run_steps, evals)
- [x] add docker-compose + Makefile + .env config
- [x] add config loader (topic, providers, budget from env)
- [x] add thin LLM provider client (Generator + Embedder interfaces)
- [x] implement Anthropic generation client
- [x] implement OpenAI generation client
- [x] implement OpenAI embedding client
- [x] add provider factory (swappable via config)
- [x] add API retry with backoff (honor Retry-After)
- [x] implement Kaggle arXiv ingestion (parse → filter → chunk → embed → index)
- [x] add topic filter (category + keyword), keep agent code topic-agnostic
- [x] parallelize embedding with a worker pool
- [x] skip already-ingested papers + add --force flag
- [x] add ingest progress logging
- [x] ingest starter corpus (~50-60k papers)
- [x] implement vector search primitive (pgvector cosine)
- [x] implement keyword search primitive (Postgres FTS)
- [x] implement retriever worker with RRF hybrid fusion
- [x] implement planner (question → sub-queries + filters)
- [x] implement synthesizer (evidence → JSON answer)
- [x] wire orchestrator: plan → parallel retrieve → synth
- [x] redact API keys from run-config logging
- [x] run end-to-end ask producing a grounded answer
- [x] add eval command (thumbs + comment per run)
- [x] strip code fences before parsing synthesizer JSON
- [x] validate planner output (sub-query count, non-empty, topic filters)
- [x] pass chunk ids into the synthesizer prompt
- [x] print a resolved source list under the answer
- [x] populate structured Citations from the answer
- [x] validate citations reference retrieved chunks (anti-hallucination)
- [ ] implement critic: grounding check (every claim backed by a chunk)
- [ ] implement critic: gap detection → follow-up sub-queries
- [ ] add bounded re-retrieval loop (max 2 rounds)
- [ ] enforce budget (max rounds + max cost)
- [ ] add token/cost accounting (per-model price table)
- [ ] log every agent step (LogStep: input, output, prompt, raw, tokens, cost)
- [ ] ask 10-15 real research questions end-to-end
- [ ] collect eval thumbs/comments as a seed set

v1.1 — deepenings:
- [ ] ingest full-text PDFs with real chunking
- [ ] build labeled eval set (retrieval recall/precision, grounding accuracy)
- [ ] force structured output via provider JSON mode / prefill
- [ ] set up golangci-lint
