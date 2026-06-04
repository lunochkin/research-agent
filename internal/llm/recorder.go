package llm

import "context"

type CallInfo struct {
	Model     string
	Prompt    string // rendered request (JSON of messages)
	Raw       string // raw response text
	TokensIn  int
	TokensOut int
	CostUSD   float64
	LatencyMS int
	Attempt   int
}
type CallRecorder interface {
	Record(context.Context, CallInfo)
}

// unexported key type - prevents collisions with other ctx values
type recorderKey struct{}

func WithRecorder(ctx context.Context, r CallRecorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, r)
}

func RecorderFrom(ctx context.Context) CallRecorder {
	r, _ := ctx.Value(recorderKey{}).(CallRecorder)
	return r
}
