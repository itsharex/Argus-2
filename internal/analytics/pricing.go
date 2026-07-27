package analytics

import "strings"

// Currency represents the currency used for pricing.
type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyCNY Currency = "CNY"
)

// ModelPricing holds per-model pricing per 1M tokens.
type ModelPricing struct {
	Name        string   `json:"name"`
	InputPer1M  float64  `json:"inputPer1M"`
	OutputPer1M float64  `json:"outputPer1M"`
	Currency    Currency `json:"currency"`
}

// CostResult holds the calculated cost and its currency.
type CostResult struct {
	Amount   float64  `json:"amount"`
	Currency Currency `json:"currency"`
}

var defaultPricingTable = map[string]ModelPricing{
	"claude-opus-4-6":          {Name: "Claude Opus 4.6", InputPer1M: 15.0, OutputPer1M: 75.0, Currency: CurrencyUSD},
	"claude-opus-4-20250514":   {Name: "Claude Opus 4", InputPer1M: 15.0, OutputPer1M: 75.0, Currency: CurrencyUSD},
	"claude-opus-4-5-20250514": {Name: "Claude Opus 4.5", InputPer1M: 15.0, OutputPer1M: 75.0, Currency: CurrencyUSD},
	"claude-sonnet-4-6":          {Name: "Claude Sonnet 4.6", InputPer1M: 3.0, OutputPer1M: 15.0, Currency: CurrencyUSD},
	"claude-sonnet-4-20250514":   {Name: "Claude Sonnet 4", InputPer1M: 3.0, OutputPer1M: 15.0, Currency: CurrencyUSD},
	"claude-sonnet-4-5-20250514": {Name: "Claude Sonnet 4.5", InputPer1M: 3.0, OutputPer1M: 15.0, Currency: CurrencyUSD},
	"claude-haiku-4-5-20251001":  {Name: "Claude Haiku 4.5", InputPer1M: 0.80, OutputPer1M: 4.0, Currency: CurrencyUSD},
	"claude-3-5-sonnet-20241022": {Name: "Claude 3.5 Sonnet", InputPer1M: 3.0, OutputPer1M: 15.0, Currency: CurrencyUSD},
	"claude-3-5-haiku-20241022":  {Name: "Claude 3.5 Haiku", InputPer1M: 0.80, OutputPer1M: 4.0, Currency: CurrencyUSD},
	"claude-3-opus-20240229":     {Name: "Claude 3 Opus", InputPer1M: 15.0, OutputPer1M: 75.0, Currency: CurrencyUSD},
	"claude-3-sonnet-20240229":   {Name: "Claude 3 Sonnet", InputPer1M: 3.0, OutputPer1M: 15.0, Currency: CurrencyUSD},
	"claude-3-haiku-20240307":    {Name: "Claude 3 Haiku", InputPer1M: 0.25, OutputPer1M: 1.25, Currency: CurrencyUSD},
	"mimo-v2.5":     {Name: "MiMo V2.5", InputPer1M: 1.00, OutputPer1M: 2.00, Currency: CurrencyCNY},
	"mimo-v2.5-pro": {Name: "MiMo V2.5 Pro", InputPer1M: 3.00, OutputPer1M: 6.00, Currency: CurrencyCNY},
}

// PricingTable holds the current pricing configuration.
type PricingTable struct {
	prices map[string]ModelPricing
}

// NewPricingTable creates a pricing table with default values.
func NewPricingTable() *PricingTable {
	return &PricingTable{prices: defaultPricingTable}
}

// GetPricing returns the pricing for a given model string.
func (pt *PricingTable) GetPricing(model string) ModelPricing {
	normalized := normalizeModelName(model)
	if p, ok := pt.prices[normalized]; ok {
		return p
	}
	return ModelPricing{Name: model, InputPer1M: 3.0, OutputPer1M: 15.0, Currency: CurrencyUSD}
}

// CostForTokens calculates the cost for given token counts.
func (pt *PricingTable) CostForTokens(model string, inputTokens, outputTokens int) CostResult {
	p := pt.GetPricing(model)
	inputCost := float64(inputTokens) / 1_000_000.0 * p.InputPer1M
	outputCost := float64(outputTokens) / 1_000_000.0 * p.OutputPer1M
	return CostResult{Amount: inputCost + outputCost, Currency: p.Currency}
}

// GetCurrency returns the currency for a given model.
func (pt *PricingTable) GetCurrency(model string) Currency {
	return pt.GetPricing(model).Currency
}

func normalizeModelName(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if _, ok := defaultPricingTable[m]; ok {
		return m
	}
	parts := strings.Split(m, "-")
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if len(last) == 8 {
			noDate := strings.Join(parts[:len(parts)-1], "-")
			if _, ok := defaultPricingTable[noDate]; ok {
				return noDate
			}
		}
	}
	return m
}
