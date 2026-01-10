package parser

import (
	"fmt"

	"github.com/young1lin/claude-token-monitor/internal/config"
)

// TokenStats represents aggregated token statistics
type TokenStats struct {
	InputTokens  int
	OutputTokens int
	CacheTokens  int
	TotalTokens  int
}

// CalculateStats calculates effective token statistics from an AssistantMessage
func CalculateStats(msg *AssistantMessage) TokenStats {
	// Effective input = regular input + cache creation (full price)
	// Cache reads are counted separately (discounted price)
	effectiveInput := msg.Message.Usage.InputTokens

	return TokenStats{
		InputTokens:  effectiveInput,
		OutputTokens: msg.Message.Usage.OutputTokens,
		CacheTokens:  msg.Message.Usage.CacheReadInputTokens,
		TotalTokens:  effectiveInput + msg.Message.Usage.OutputTokens,
	}
}

// CalculateContextPercentage calculates the percentage of context window used
func CalculateContextPercentage(model string, totalTokens int) float64 {
	return CalculateContextPercentageWithGetter(model, totalTokens, config.GetContextWindow)
}

// CalculateContextPercentageWithGetter allows injecting a custom context window getter for testing
func CalculateContextPercentageWithGetter(
	model string,
	totalTokens int,
	getContextWindow func(string) int,
) float64 {
	contextWindow := getContextWindow(model)
	if contextWindow == 0 {
		return 0
	}
	return float64(totalTokens) / float64(contextWindow) * 100
}

// CalculateCost calculates the cost in USD for token usage
func CalculateCost(model string, inputTokens, outputTokens, cacheTokens int) float64 {
	return config.CalculateCost(model, inputTokens, outputTokens, cacheTokens)
}

// FormatTokens formats a token count for display (e.g., 1234 -> "1.2K")
func FormatTokens(count int) string {
	switch {
	case count >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	case count >= 1_000:
		return fmt.Sprintf("%.1fK", float64(count)/1_000)
	default:
		return fmt.Sprintf("%d", count)
	}
}

// FormatCost formats a cost in USD for display
func FormatCost(cost float64) string {
	if cost >= 1.0 {
		return fmt.Sprintf("$%.2f", cost)
	}
	return fmt.Sprintf("$%.4f", cost)
}
