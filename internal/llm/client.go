// Package llm is the thin provider client. Agents depend on these interfaces,
// never on a concrete SDK — provider choice stays swappable (CLAUDE.md).
package llm

import "context"

type Message struct {
	Role    string // "user" | "assistant"
	Content string
}

type GenerateRequest struct {
	System    string
	Messages  []Message
	MaxTokens int
}

type GenerateResponse struct {
	Text      string // raw model text — validate before trusting (structured outputs)
	TokensIn  int
	TokensOut int
}

// Generator produces text. Implementations: Anthropic (default), OpenAI.
type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

// Embedder turns text into vectors for pgvector. Separate from Generator
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dim reports the embedding dimension (must match the chunks.embedding column).
	Dim() int
}
