package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lunochkin/research-agent/internal/llm"
)

// Synthesizer turns fused evidence into a cited, grounded answer.
type Synthesizer struct {
	gen llm.Generator
}

func NewSynthesizer(g llm.Generator) *Synthesizer { return &Synthesizer{gen: g} }

// Synthesize composes the answer from evidence:
//   - build a prompt that asks the model to cite the chunks it used
//   - parse raw text into Answer
//   - validate: every Citation references a chunk present in evidence; reject
//     answers that cite chunks not retrieved (anti-hallucination)
func (s *Synthesizer) Synthesize(ctx context.Context, question string, ev *Evidence) (*Answer, error) {
	if ev == nil {
		return nil, errors.New("no evidence provided")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n\nSources:\n", question)

	for _, chunk := range ev.Chunks {
		fmt.Fprintf(&b, "[chunkId:%d] [paperId:%s] %s\n\n", chunk.ChunkID, chunk.PaperID, chunk.Content)
	}

	messages := []llm.Message{
		{
			Role:    "user",
			Content: b.String(),
		},
	}

	exampleJson, err := json.Marshal(Answer{
		Text: "<< Synthesized Answer Text >>",
		Citations: []Citation{
			{ChunkID: 1042, PaperID: "paper-id"},
		},
	})
	if err != nil {
		return nil, err
	}
	systemPrompt := fmt.Sprintf(
		`
		You receive a user question and multiple retrieved chunks from the db.
		Your role is to synthesize the retrieved information into one coherent answer.
		Cite sources using their chunkId and paperId exactly as shown in Sources.
		Do not use the [N] list position. Only cite sources you actually used.

		Return ONLY JSON matching this shape: %s
		`,
		exampleJson,
	)

	req := llm.GenerateRequest{
		System:    systemPrompt,
		Messages:  messages,
		MaxTokens: 10000,
	}
	resp, err := s.gen.Generate(ctx, req)
	if err != nil {
		return nil, err
	}
	slog.Debug("llm answered", "resp.Text", resp.Text)

	txt := strings.TrimSpace(resp.Text)
	txt = strings.TrimPrefix(txt, "```json")
	txt = strings.TrimPrefix(txt, "```")
	txt = strings.TrimSuffix(txt, "```")

	var answer Answer
	err = json.Unmarshal([]byte(txt), &answer)
	if err != nil {
		return nil, fmt.Errorf("synth: bad JSON: %w", err)
	}

	if answer.Text == "" {
		return nil, fmt.Errorf("synth: empty answer: %+v", answer)
	}

	for _, c := range answer.Citations {
		valid := false
		for _, e := range ev.Chunks {
			if e.ChunkID == c.ChunkID && e.PaperID == c.PaperID {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("synth: invalid citation: %+v", c)
		}
	}

	return &answer, nil
}
