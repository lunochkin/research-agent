package llm

import (
	"fmt"

	"github.com/lunochkin/research-agent/internal/config"
)

// NewGenerator picks a Generator implementation from config (provider swappable).
func NewGenerator(p config.Provider) (Generator, error) {
	switch p.Name {
	case "anthropic":
		return NewAnthropic(p.APIKey, p.Model), nil
	case "openai":
		return NewOpenAIGenerator(p.APIKey, p.Model), nil
	default:
		return nil, fmt.Errorf("llm: unknown generation provider %q", p.Name)
	}
}

// NewEmbedder picks an Embedder implementation from config.
func NewEmbedder(p config.Provider) (Embedder, error) {
	switch p.Name {
	case "openai":
		return NewOpenAIEmbedder(p.APIKey, p.Model), nil
	default:
		return nil, fmt.Errorf("llm: unknown embed provider %q", p.Name)
	}
}
