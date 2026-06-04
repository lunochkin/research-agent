package agent

import (
	"context"
	"log/slog"

	"github.com/lunochkin/research-agent/internal/llm"
	"github.com/lunochkin/research-agent/internal/store"
)

type llmRecorder struct {
	st     *store.Store
	stepID int64
}

func (r llmRecorder) Record(ctx context.Context, ci llm.CallInfo) {
	err := r.st.LogLlmCall(ctx, store.LlmCall{
		StepID:    r.stepID,
		Model:     ci.Model,
		Prompt:    ci.Prompt,
		Raw:       ci.Raw,
		TokensIn:  ci.TokensIn,
		TokensOut: ci.TokensOut,
		LatencyMS: ci.LatencyMS,
		Attempt:   ci.Attempt,
	})
	if err != nil {
		slog.Warn("logLlmCall failed", "error", err)
	}
}
