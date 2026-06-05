package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lunochkin/research-agent/internal/config"
	"github.com/lunochkin/research-agent/internal/llm"
)

// Critic verifies grounding and detects gaps that drive re-retrieval.
type Critic struct {
	gen   llm.Generator
	topic config.Topic
}

func NewCritic(g llm.Generator, t config.Topic) *Critic { return &Critic{gen: g, topic: t} }

// Critique checks the answer against the evidence.
func (c *Critic) Critique(ctx context.Context, question string, ans *Answer, ev *Evidence) (*Critique, error) {
	if ev == nil {
		return nil, errors.New("no evidence provided")
	}
	if ans == nil {
		return nil, errors.New("no answer provided")
	}

	exampleJSON, err := json.Marshal(Critique{
		Grounded: false,
		Gaps:     []string{"The claim that the method reduces hallucination is not supported by any retrieved chunk."},
		FollowupQueries: []SubQuery{
			{
				Query: "hallucination reduction benchmark results",
				Filters: Filters{
					Categories: []string{"cs.AI"},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	evidenceJSON, err := json.Marshal(ev)
	if err != nil {
		return nil, err
	}

	answerJSON, err := json.Marshal(ans)
	if err != nil {
		return nil, err
	}

	systemPrompt := fmt.Sprintf(
		`
		You are a grounding critic for a retrieval-augmented answer. Output a single JSON
		object and nothing else — no prose, no markdown, no code fences. Your entire
		response must be valid JSON matching this schema:

		%s

		Judge the answer ONLY against the provided evidence chunks. Do NOT use your own
		knowledge: a claim that is true in the world but not supported by a chunk is NOT
		grounded. The answer cites chunks inline as [chunkId:N]; for each claim, check that
		that chunk's content actually supports it.

		Fill the JSON:
		- grounded: true only if EVERY claim is supported by a chunk in the evidence. One
			unsupported or miscited claim makes it false.
		- gaps: each unsupported claim or unanswered facet, stated specifically (name the
			missing or unsupported fact). Empty list when grounded is true.
		- followup_queries: up to 4 search queries that would retrieve the missing evidence,
			one per gap. Use only these categories: %s. Empty list when grounded is true.

		Consistency: grounded true → gaps and followup_queries both empty. grounded false →
		gaps non-empty.
		`,
		exampleJSON,
		strings.Join(c.topic.Categories, ", "),
	)

	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n\n", question)
	fmt.Fprintf(&b, "Evidence: %s\n\n", evidenceJSON)
	fmt.Fprintf(&b, "Answer: %s\n", answerJSON)

	messages := []llm.Message{
		{
			Role:    "user",
			Content: b.String(),
		},
	}

	req := llm.GenerateRequest{
		System:    systemPrompt,
		Messages:  messages,
		MaxTokens: 10000,
	}
	output, err := c.gen.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	txt := strings.TrimSpace(output.Text)
	txt = strings.TrimPrefix(txt, "```json")
	txt = strings.TrimPrefix(txt, "```")
	txt = strings.TrimSuffix(txt, "```")

	var critique Critique
	err = json.Unmarshal([]byte(txt), &critique)
	if err != nil {
		return nil, fmt.Errorf("critic: bad JSON: %w", err)
	}

	slog.Debug("critique", "result", critique)

	return &critique, nil
}
