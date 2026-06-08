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
- [x] add golang-migrate for schema management
- [x] log every agent step (LogStep: input, output)
- [x] log every LLM call (LogLlmCall: stepID, model, prompt, raw, tokens, cost)
- [x] track run cost (RunCost: runID → sum(cost_usd))
- [x] implement critic: grounding check (every claim backed by a chunk)
- [x] implement critic: gap detection → follow-up sub-queries
- [x] add bounded re-retrieval loop (max 2 total retrieval rounds)
- [x] enforce budget (max rounds + max cost)
- [ ] fix critic Failure B: synth/critic citation contract mismatch — synth emits
      no inline [chunkId:N] markers, critic assumes them → always grounded=false,
      every run burns to MaxRounds (pick: synth emits inline markers, or critic
      judges the Citations array + evidence)
- [ ] soften grounding bar: gap only genuinely-unsupported core claims, don't flip
      the whole verdict on one debatable claim
- [ ] verify the fix: easy in-corpus question → grounded round 1; out-of-corpus
      question → grounded=false → followup → terminal at budget
- [ ] ask 10-15 real research questions end-to-end
- [ ] collect eval thumbs/comments as a seed set
- [ ] test reproducibility and repeatability of runs

v1.1 — deepenings:
- [ ] ingest full-text PDFs with real chunking
- [ ] build labeled eval set (retrieval recall/precision, grounding accuracy)
- [ ] force structured output via provider JSON mode / prefill
- [ ] set up golangci-lint
