package llm

// Model pricing for cost estimation. Rates are USD per 1,000,000 tokens, input and
// output billed separately. This is a hand-maintained snapshot, not a billing source
// of truth — the provider invoice is authoritative; these values drift when vendors
// reprice. Recheck against the vendor pricing pages periodically.
//
// Prices as of 2026-06.
//   OpenAI:    https://openai.com/api/pricing/
//   Anthropic: https://www.anthropic.com/pricing

type price struct {
	inPerM  float64 // USD per 1M input (prompt) tokens
	outPerM float64 // USD per 1M output (completion) tokens
}

var prices = map[string]price{
	// OpenAI
	"gpt-4o":       {2.50, 10.00},
	"gpt-4o-mini":  {0.15, 0.60},
	"gpt-4.1":      {2.00, 8.00},
	"gpt-4.1-mini": {0.40, 1.60},
	"gpt-4.1-nano": {0.10, 0.40},

	// Anthropic
	"claude-opus-4":   {15.00, 75.00},
	"claude-sonnet-4": {3.00, 15.00},
	"claude-haiku-4":  {1.00, 5.00},
}

// CostUSD estimates the dollar cost of one model call from its token counts.
// The bool is false when the model has no price entry (caller may warn and treat
// the cost as 0 rather than failing — an unknown model under-counts, not breaks).
func CostUSD(model string, tokensIn, tokensOut int) (float64, bool) {
	p, ok := prices[model]
	if !ok {
		return 0, false
	}
	return float64(tokensIn)/1e6*p.inPerM + float64(tokensOut)/1e6*p.outPerM, true
}
