package agent

import (
	"context"

	"github.com/lunochkin/research-agent/internal/llm"
)

// fakeGen is a Generator that returns a canned response, for testing the
// parse/validate logic of the agents without a real LLM.
type fakeGen struct {
	text string
	err  error
}

func (f fakeGen) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &llm.GenerateResponse{Text: f.text}, nil
}
