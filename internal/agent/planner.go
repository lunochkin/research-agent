package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/lunochkin/research-agent/internal/config"
	"github.com/lunochkin/research-agent/internal/llm"
)

// Planner decomposes a question into sub-queries + filters.
type Planner struct {
	gen   llm.Generator
	topic config.Topic
}

func NewPlanner(g llm.Generator, t config.Topic) *Planner {
	return &Planner{
		gen:   g,
		topic: t,
	}
}

// Plan turns the question into a validated Plan:
//   - prompt the generator to decompose the question into 2–4 sub-queries
//   - parse the raw text into Plan
//   - validate (don't trust raw model text): non-empty queries, sane count,
//     filters within the configured topic
//   - return a typed error on validation failure
func (p *Planner) Plan(ctx context.Context, question string) (*Plan, error) {
	shapeJSON, err := json.Marshal(Plan{
		SubQueries: []SubQuery{
			{
				Query: "<< Single purpose question >>",
				Filters: Filters{
					Categories: []string{"<< Single category from allowed categories list. Could be empty >>"},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	const minSubqueries = 2
	const maxSubqueries = 4

	systemPrompt := fmt.Sprintf(
		`
		You decompose a question into %d-%d sub-queries.
		Valid categories: %s
		Return ONLY JSON matching this shape: %s
		`,
		minSubqueries,
		maxSubqueries,
		strings.Join(p.topic.Categories, ", "),
		shapeJSON,
	)

	output, err := p.gen.Generate(ctx, llm.GenerateRequest{
		System: systemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: question},
		},
	})
	if err != nil {
		return nil, err
	}

	slog.Debug("llm answered", "text output", output.Text)

	txt := strings.TrimSpace(output.Text)
	txt = strings.TrimPrefix(txt, "```json")
	txt = strings.TrimPrefix(txt, "```")
	txt = strings.TrimSuffix(txt, "```")

	var plan Plan
	err = json.Unmarshal([]byte(txt), &plan)
	if err != nil {
		return nil, err
	}

	if len(plan.SubQueries) < minSubqueries || len(plan.SubQueries) > maxSubqueries {
		return nil, fmt.Errorf("planner: %d sub-queries (want %d-%d)", len(plan.SubQueries), minSubqueries, maxSubqueries)
	}

	for _, q := range plan.SubQueries {
		if strings.TrimSpace(q.Query) == "" {
			return nil, fmt.Errorf("planner: empty sub-query")
		}
	}

	for _, q := range plan.SubQueries {
		for _, c := range q.Filters.Categories {
			if !slices.Contains(p.topic.Categories, c) {
				return nil, fmt.Errorf("planner: category %q not in topic", c)
			}
		}
	}

	return &Plan{SubQueries: plan.SubQueries}, nil
}
