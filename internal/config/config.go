// Package config loads runtime configuration from environment (.env optional).
// Topic is config from day one so agent code stays topic-agnostic.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Provider struct {
	Name   string // anthropic | openai
	APIKey string `json:"-"`
	Model  string
}

type Topic struct {
	Categories []string // e.g. cs.AI, cs.LG
	Keywords   []string // OR-matched against title+abstract
}

type Budget struct {
	MaxRounds  int     // hard cap on re-retrieval rounds
	MaxCostUSD float64 // hard cap on spend per run
}

type Config struct {
	DatabaseURL string
	Gen         Provider // generation
	Embed       Provider // embeddings (separate: Anthropic has no embeddings API)
	Topic       Topic
	Budget      Budget
}

// Load reads config from the environment. Call godotenv (or `source .env`)
// before this if you keep secrets in a file.
func Load() (*Config, error) {
	c := &Config{
		DatabaseURL: env("DATABASE_URL", "postgres://rag:rag@localhost:5432/rag?sslmode=disable"),
		Gen:         genProvider(env("LLM_PROVIDER", "anthropic")),
		Embed: Provider{
			Name:   env("EMBED_PROVIDER", "openai"),
			APIKey: os.Getenv("OPENAI_API_KEY"),
			Model:  env("EMBED_MODEL", "text-embedding-3-small"),
		},
		Topic: Topic{
			Categories: splitCSV(env("ARXIV_CATEGORIES", "cs.AI,cs.LG")),
			Keywords:   splitCSV(env("KEYWORD_FILTER", "agent,LLM")),
		},
		Budget: Budget{
			MaxRounds:  atoi(env("MAX_ROUNDS", "2"), 2),
			MaxCostUSD: atof(env("MAX_COST_USD", "1.0"), 1.0),
		},
	}
	if c.Budget.MaxRounds > 2 {
		return nil, fmt.Errorf("MAX_ROUNDS=%d exceeds scope cap of 2", c.Budget.MaxRounds)
	}
	return c, nil
}

// genProvider resolves the generation provider's key + model from its name, so
// switching LLM_PROVIDER also swaps which API key and model env vars are read.
func genProvider(name string) Provider {
	switch name {
	case "openai":
		return Provider{
			Name:   "openai",
			APIKey: os.Getenv("OPENAI_API_KEY"),
			Model:  env("OPENAI_GEN_MODEL", "gpt-4o"),
		}
	default: // anthropic
		return Provider{
			Name:   "anthropic",
			APIKey: os.Getenv("ANTHROPIC_API_KEY"),
			Model:  env("ANTHROPIC_MODEL", "claude-opus-4-8"),
		}
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func atof(s string, def float64) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return def
}
