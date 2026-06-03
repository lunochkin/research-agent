package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	openAIEmbedURL = "https://api.openai.com/v1/embeddings"
	openAIChatURL  = "https://api.openai.com/v1/chat/completions"
)

// OpenAIGenerator implements Generator over the raw Chat Completions API (no SDK).
type OpenAIGenerator struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

func NewOpenAIGenerator(apiKey, model string) *OpenAIGenerator {
	return &OpenAIGenerator{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (g *OpenAIGenerator) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if g.APIKey == "" {
		return nil, fmt.Errorf("openai: OPENAI_API_KEY not set")
	}
	// Chat Completions takes a flat messages array; system prompt is a message.
	msgs := make([]map[string]string, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	payload := map[string]any{
		"model":    g.Model,
		"messages": msgs,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	body, _ := json.Marshal(payload)

	raw, err := postJSON(ctx, g.HTTP, openAIChatURL, map[string]string{
		"content-type":  "application/json",
		"authorization": "Bearer " + g.APIKey,
	}, body)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices: %s", raw)
	}
	return &GenerateResponse{
		Text:      out.Choices[0].Message.Content,
		TokensIn:  out.Usage.PromptTokens,
		TokensOut: out.Usage.CompletionTokens,
	}, nil
}

// OpenAIEmbedder implements Embedder over the raw embeddings API (no SDK).
// text-embedding-3-small => 1536 dims (matches the chunks.embedding column).
type OpenAIEmbedder struct {
	APIKey string
	Model  string
	dim    int
	HTTP   *http.Client
}

func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	dim := 1536
	if model == "text-embedding-3-large" {
		dim = 3072
	}
	return &OpenAIEmbedder{
		APIKey: apiKey,
		Model:  model,
		dim:    dim,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (e *OpenAIEmbedder) Dim() int { return e.dim }

func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if e.APIKey == "" {
		return nil, fmt.Errorf("openai: OPENAI_API_KEY not set")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	body, _ := json.Marshal(map[string]any{
		"model": e.Model,
		"input": texts,
	})
	raw, err := postJSON(ctx, e.HTTP, openAIEmbedURL, map[string]string{
		"content-type":  "application/json",
		"authorization": "Bearer " + e.APIKey,
	}, body)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}
	// Guarantee one embedding per input, in request order — callers index by
	// input position, so a partial/empty response must be an error, not a panic.
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("openai: got %d embeddings for %d inputs", len(out.Data), len(texts))
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			return nil, fmt.Errorf("openai: embedding index %d out of range for %d inputs", d.Index, len(texts))
		}
		vecs[d.Index] = d.Embedding
	}
	for i, v := range vecs {
		if v == nil {
			return nil, fmt.Errorf("openai: missing embedding for input %d", i)
		}
	}
	return vecs, nil
}
