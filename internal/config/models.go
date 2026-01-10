package config

import "strings"

// ModelInfo contains metadata about a Claude model
type ModelInfo struct {
	Name           string
	ContextWindow  int
	InputPrice     float64 // Price per million tokens
	OutputPrice    float64 // Price per million tokens
	CacheReadPrice float64 // Price per million tokens (cache reads)
}

// ModelInfoRegistry maps model IDs to their metadata
var ModelInfoRegistry = map[string]ModelInfo{
	"claude-sonnet-4-5-20250929": {
		Name:           "Sonnet 4.5",
		ContextWindow:  200000,
		InputPrice:     3.00,
		OutputPrice:    15.00,
		CacheReadPrice: 0.30, // Cache reads are 90% discounted
	},
	"claude-opus-4-5-20251101": {
		Name:           "Opus 4.5",
		ContextWindow:  200000,
		InputPrice:     15.00,
		OutputPrice:    75.00,
		CacheReadPrice: 1.50,
	},
	"claude-haiku-4-5-20250929": {
		Name:           "Haiku 4.5",
		ContextWindow:  200000,
		InputPrice:     0.80,
		OutputPrice:    4.00,
		CacheReadPrice: 0.08,
	},
	// Add fallback for older model names
	"claude-sonnet-4-5": {
		Name:           "Sonnet 4.5",
		ContextWindow:  200000,
		InputPrice:     3.00,
		OutputPrice:    15.00,
		CacheReadPrice: 0.30,
	},
	"claude-opus-4-5": {
		Name:           "Opus 4.5",
		ContextWindow:  200000,
		InputPrice:     15.00,
		OutputPrice:    75.00,
		CacheReadPrice: 1.50,
	},
	"claude-haiku-4-5": {
		Name:           "Haiku 4.5",
		ContextWindow:  200000,
		InputPrice:     0.80,
		OutputPrice:    4.00,
		CacheReadPrice: 0.08,
	},
}

// GetModelInfo returns model info for a given model ID
// Returns default Sonnet 4.5 info if model not found
func GetModelInfo(modelID string) ModelInfo {
	if info, ok := ModelInfoRegistry[modelID]; ok {
		return info
	}
	// Try to find by prefix (for version variations)
	if modelID != "" {
		for k, v := range ModelInfoRegistry {
			if strings.HasPrefix(modelID, k) || strings.HasPrefix(k, modelID) {
				return v
			}
		}
	}
	// Default fallback
	return ModelInfoRegistry["claude-sonnet-4-5-20250929"]
}

// CalculateCost calculates the cost in USD for a given model's token usage
func CalculateCost(modelID string, inputTokens, outputTokens, cacheReadTokens int) float64 {
	info := GetModelInfo(modelID)

	inputCost := float64(inputTokens) / 1_000_000 * info.InputPrice
	outputCost := float64(outputTokens) / 1_000_000 * info.OutputPrice
	cacheCost := float64(cacheReadTokens) / 1_000_000 * info.CacheReadPrice

	return inputCost + outputCost + cacheCost
}

// GetContextWindow returns the context window size for a given model
func GetContextWindow(modelID string) int {
	info := GetModelInfo(modelID)
	return info.ContextWindow
}

// GetModelName returns a human-readable model name
func GetModelName(modelID string) string {
	info := GetModelInfo(modelID)
	return info.Name
}
