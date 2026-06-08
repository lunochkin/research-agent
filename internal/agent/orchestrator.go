package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/lunochkin/research-agent/internal/config"
	"github.com/lunochkin/research-agent/internal/llm"
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

// Run executes one question through the pipeline
func (o *Orchestrator) Run(ctx context.Context, question string) (*Result, error) {
	if o.config.Budget.MaxRounds == 0 {
		return nil, errors.New("MaxRounds is invalid")
	}

	cfgJson, err := json.Marshal(*o.config)
	if err != nil {
		return nil, err
	}

	runID, err := o.store.CreateRun(ctx, question, cfgJson)
	if err != nil {
		return nil, err
	}

	round := 1
	plan, err := o.plan(ctx, question, runID, round)
	if err != nil {
		return nil, err
	}

	var costUSD float64
	var answer *Answer

	subqueries := plan.SubQueries
	var evidence *Evidence

	for i := range o.config.Budget.MaxRounds {
		if i > 0 {
			round += 1
		}
		loopEvidence, err := o.retrieve(ctx, &Plan{
			SubQueries: subqueries,
		}, runID, round)

		if err != nil {
			return nil, err
		}

		evidence = mergeEvidence(evidence, loopEvidence)

		answer, err = o.synthesize(ctx, question, evidence, runID, round)
		if err != nil {
			return nil, err
		}

		critique, err := o.critique(ctx, question, evidence, answer, runID, round)
		if err != nil {
			return nil, err
		}
		costUSD, err = o.store.RunCost(ctx, runID)
		if err != nil {
			slog.Warn("run cost query failed", "run", runID, "err", err)
		}
		slog.Info("Accumulated cost", "costUSD", costUSD)

		if critique.Grounded {
			break
		}

		if len(critique.FollowupQueries) == 0 {
			slog.Warn("Answer isn't considered grounded but critique didn't provide followup queries")
			break
		}

		if costUSD > o.config.Budget.MaxCostUSD {
			slog.Warn("MaxCostUSD reached")
			break
		}

		subqueries = critique.FollowupQueries
		if err := validateSubQueries(subqueries, o.config.Topic.Categories); err != nil {
			slog.Warn("FollowUps issue", "error", err)
			break
		}
	}

	err = o.store.FinishRun(ctx, runID, answer.Text, costUSD)
	if err != nil {
		return nil, err
	}

	res := Result{
		RunID:   runID,
		Rounds:  round,
		Answer:  answer,
		CostUSD: costUSD,
	}

	return &res, nil
}

func (o *Orchestrator) plan(ctx context.Context, question string, runID int64, round int) (*Plan, error) {
	qJSON, _ := json.Marshal(question)
	var plan *Plan
	err := o.logStep(ctx, runID, "planner", round, qJSON, func(childCtx context.Context) ([]byte, error) {
		p, err := o.planner.Plan(childCtx, question)
		if err != nil {
			return nil, err
		}
		plan = p
		return json.Marshal(p)
	})
	return plan, err
}

func (o *Orchestrator) retrieve(ctx context.Context, plan *Plan, runID int64, round int) (*Evidence, error) {
	var chunkEvidences = make([]*Evidence, len(plan.SubQueries))

	var g sync.WaitGroup

	for i, subQuery := range plan.SubQueries {
		g.Go(func() {
			sqJSON, _ := json.Marshal(subQuery)
			var ev *Evidence
			err := o.logStep(ctx, runID, "retriever", round, sqJSON, func(childCtx context.Context) ([]byte, error) {
				e, err := o.retr.Retrieve(childCtx, subQuery)
				if err != nil {
					return nil, err
				}
				ev = e
				return json.Marshal(e)
			})
			if err != nil {
				slog.Warn("Retrieval failed", "subQuery", subQuery, "err", err)
				return
			}
			chunkEvidences[i] = ev
		})
	}
	g.Wait()

	var evidence Evidence
	for _, ev := range chunkEvidences {
		if ev != nil {
			evidence.Chunks = append(evidence.Chunks, ev.Chunks...)
		}
	}
	if len(evidence.Chunks) == 0 {
		return nil, errors.New("no evidence collected")
	}

	return &evidence, nil
}

func (o *Orchestrator) synthesize(ctx context.Context, question string, evidence *Evidence, runID int64, round int) (*Answer, error) {
	var answer *Answer
	evJSON, _ := json.Marshal(evidence)
	err := o.logStep(ctx, runID, "synthesizer", round, evJSON, func(childCtx context.Context) ([]byte, error) {
		ans, err := o.synth.Synthesize(childCtx, question, evidence)
		if err != nil {
			return nil, err
		}
		answer = ans
		return json.Marshal(ans)
	})
	return answer, err
}

func (o *Orchestrator) critique(ctx context.Context, question string, evidence *Evidence, answer *Answer, runID int64, round int) (*Critique, error) {
	var critique *Critique
	anJSON, _ := json.Marshal(answer)
	err := o.logStep(ctx, runID, "critic", round, anJSON, func(childCtx context.Context) ([]byte, error) {
		cr, err := o.critic.Critique(childCtx, question, answer, evidence)
		if err != nil {
			return nil, err
		}
		critique = cr
		return json.Marshal(cr)
	})
	return critique, err
}

func (o *Orchestrator) logStep(ctx context.Context, runID int64, agent string, round int, input []byte, fn func(context.Context) ([]byte, error)) error {
	stepID, err := o.store.CreateStep(ctx, store.Step{
		RunID: runID,
		Agent: agent,
		Round: round,
		Input: input,
	})
	if err != nil {
		slog.Warn("create step failed", "agent", agent, "err", err)
	}
	ctx = llm.WithRecorder(ctx, llmRecorder{o.store, stepID})

	out, agentErr := fn(ctx)

	errMsg := ""
	if agentErr != nil {
		errMsg = agentErr.Error()
	}
	if stepID != 0 {
		if e := o.store.FinishStep(ctx, stepID, out, errMsg); e != nil {
			slog.Warn("finish step failed", "agent", agent, "err", e)
		}
	}
	return agentErr
}

func mergeEvidence(ev1 *Evidence, ev2 *Evidence) *Evidence {
	if ev1 == nil && ev2 == nil {
		return &Evidence{}
	}

	if ev1 == nil {
		return ev2
	}

	if ev2 == nil {
		return ev1
	}

	chunks := ev1.Chunks
	chunkIdsMap := map[int64]bool{}
	for _, c := range ev1.Chunks {
		chunkIdsMap[c.ChunkID] = true
	}

	for _, c := range ev2.Chunks {
		if _, ok := chunkIdsMap[c.ChunkID]; !ok {
			chunks = append(chunks, c)
		}
	}

	return &Evidence{
		Chunks: chunks,
	}
}
