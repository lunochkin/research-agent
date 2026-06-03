package agent

import (
	"context"

	"github.com/lunochkin/research-agent/internal/llm"
)

// Critic verifies grounding and detects gaps that drive re-retrieval.
type Critic struct {
	gen llm.Generator
}

func NewCritic(g llm.Generator) *Critic { return &Critic{gen: g} }

// Critique checks the answer against the evidence:
//   - check every claim in the answer is backed by a retrieved chunk
//   - detect gaps (claims unsupported, or question facets unanswered)
//   - emit FollowupQueries for the next round when grounding is incomplete
//   - validate the structured verdict before returning
//
// The orchestrator bounds how many times this can trigger re-retrieval.
func (c *Critic) Critique(ctx context.Context, question string, ans *Answer, ev Evidence) (*Critique, error) {
	_ = ctx
	_ = question
	_ = ans
	_ = ev
	return nil, ErrNotImplemented
}
