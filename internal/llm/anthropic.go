package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const anthropicURL = "https://api.anthropic.com/v1/messages"

// Anthropic implements Generator over the raw Messages API (no SDK).
type Anthropic struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *Anthropic) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("anthropic: ANTHROPIC_API_KEY not set")
	}
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 1024
	}
	msgs := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	body, _ := json.Marshal(map[string]any{
		"model":      a.Model,
		"max_tokens": maxTok,
		"system":     req.System,
		"messages":   msgs,
	})

	raw, err := postJSON(ctx, a.HTTP, anthropicURL, map[string]string{
		"content-type":      "application/json",
		"x-api-key":         a.APIKey,
		"anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}

	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("anthropic: decode: %w", err)
	}
	var text strings.Builder
	for _, c := range out.Content {
		text.WriteString(c.Text)
	}
	return &GenerateResponse{
		Text:      text.String(),
		TokensIn:  out.Usage.InputTokens,
		TokensOut: out.Usage.OutputTokens,
	}, nil
}
