package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/lunochkin/research-agent/internal/config"
	"github.com/lunochkin/research-agent/internal/store"
)

// Orchestrator wires the bounded multi-agent loop and logs every step.
type Orchestrator struct {
	planner *Planner
	retr    *Retriever
	synth   *Synthesizer
	critic  *Critic
	config  *config.Config
	store   *store.Store // run/step logging
}

func NewOrchestrator(p *Planner, r *Retriever, s *Synthesizer, c *Critic, cfg *config.Config, st *store.Store) *Orchestrator {
	return &Orchestrator{planner: p, retr: r, synth: s, critic: c, config: cfg, store: st}
}

// Result is the outcome of one run.
type Result struct {
	RunID   int64
	Answer  *Answer
	Rounds  int
	CostUSD float64
}

// Run executes one question through the pipeline:
//
//	plan → fan out retriever workers in parallel → fan in → synthesize cited answer
//
// Currently a single pass. The run is recorded (CreateRun/FinishRun) and a failed
// retriever is skipped without killing the run. The critic, bounded re-retrieval,
// per-step logging, and budget/cost enforcement are not yet implemented — see the
// TODOs in the body.
func (o *Orchestrator) Run(ctx context.Context, question string) (*Result, error) {
	cfgJson, err := json.Marshal(*o.config)
	if err != nil {
		return nil, err
	}

	runID, err := o.store.CreateRun(ctx, question, cfgJson)
	if err != nil {
		return nil, err
	}

	plan, err := o.planner.Plan(ctx, question)
	if err != nil {
		return nil, err
	}
	// TODO(logging): LogStep the planner call — input, output, prompt, raw, tokens, cost.

	var chunkEvidences = make([]Evidence, len(plan.SubQueries))

	var g sync.WaitGroup

	for i, subQuery := range plan.SubQueries {
		g.Go(func() {
			ev, err := o.retr.Retrieve(ctx, subQuery)
			if err != nil {
				slog.Warn("Retrieval failed", "subQuery", subQuery, "err", err)
				return
			}
			// TODO(logging): LogStep this retriever call.
			chunkEvidences[i] = ev
		})
	}
	g.Wait()

	var evidence Evidence
	for _, ev := range chunkEvidences {
		evidence.Chunks = append(evidence.Chunks, ev.Chunks...)
	}
	if len(evidence.Chunks) == 0 {
		return nil, errors.New("no evidence collected")
	}

	ans, err := o.synth.Synthesize(ctx, question, evidence)
	if err != nil {
		return nil, err
	}
	// TODO(logging): LogStep the synthesizer call.

	// TODO(critic): run o.critic.Critique on the answer + evidence; on detected
	// gaps, re-retrieve with the follow-up sub-queries (bounded re-retrieval).
	// TODO(budget): loop the above until grounded OR rounds == o.config.Budget.MaxRounds
	// OR spend == o.config.Budget.MaxCostUSD — stop when either is hit.

	// TODO(cost): accumulate real token cost from each agent call instead of 0.
	var costUSD float64 = 0

	err = o.store.FinishRun(ctx, runID, ans.Text, costUSD)
	if err != nil {
		return nil, err
	}

	res := Result{
		RunID:   runID,
		Rounds:  1, // TODO(critic): real round count once re-retrieval is implemented.
		Answer:  ans,
		CostUSD: costUSD,
	}

	return &res, nil
}
