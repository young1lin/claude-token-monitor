package content

import (
	"fmt"
	"strings"
	"time"
)

// ModelCollector collects the model display name
type ModelCollector struct {
	*BaseCollector
}

// NewModelCollector creates a new model collector
func NewModelCollector() *ModelCollector {
	return &ModelCollector{
		BaseCollector: NewBaseCollector(ContentModel, 5*time.Second, false),
	}
}

// Collect returns the model display name
func (c *ModelCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	modelName := statusInput.Model.DisplayName
	if modelName == "" {
		modelName = "Claude"
	}
	return modelName, nil
}

// TokenBarCollector collects the token progress bar
type TokenBarCollector struct {
	*BaseCollector
}

// NewTokenBarCollector creates a new token bar collector
func NewTokenBarCollector() *TokenBarCollector {
	return &TokenBarCollector{
		BaseCollector: NewBaseCollector(ContentTokenBar, 5*time.Second, false),
	}
}

// Collect returns the token progress bar
func (c *TokenBarCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	tokens := statusInput.ContextWindow.CurrentUsage.InputTokens +
		statusInput.ContextWindow.CurrentUsage.CacheReadInputTokens +
		statusInput.ContextWindow.CurrentUsage.OutputTokens
	maxTokens := statusInput.ContextWindow.ContextWindowSize
	if maxTokens == 0 {
		maxTokens = 200000
	}
	pct := float64(tokens) / float64(maxTokens) * 100

	barWidth := 10
	fillWidth := int(pct / 100 * float64(barWidth))
	if fillWidth > barWidth {
		fillWidth = barWidth
	}
	filled := strings.Repeat("█", fillWidth)
	empty := strings.Repeat("░", barWidth-fillWidth)

	var colorCode, resetCode string
	resetCode = "\x1b[0m"
	if pct >= 60 {
		colorCode = "\x1b[1;31m"
	} else if pct >= 40 {
		colorCode = "\x1b[1;33m"
	} else if pct >= 20 {
		colorCode = "\x1b[1;36m"
	} else {
		colorCode = "\x1b[1;32m"
	}

	return fmt.Sprintf("[%s%s%s%s]", colorCode, filled, resetCode, empty), nil
}

// TokenInfoCollector collects token usage information
type TokenInfoCollector struct {
	*BaseCollector
}

// NewTokenInfoCollector creates a new token info collector
func NewTokenInfoCollector() *TokenInfoCollector {
	return &TokenInfoCollector{
		BaseCollector: NewBaseCollector(ContentTokenInfo, 5*time.Second, false),
	}
}

// Collect returns token usage information
func (c *TokenInfoCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	tokens := statusInput.ContextWindow.CurrentUsage.InputTokens +
		statusInput.ContextWindow.CurrentUsage.CacheReadInputTokens +
		statusInput.ContextWindow.CurrentUsage.OutputTokens
	maxTokens := statusInput.ContextWindow.ContextWindowSize
	if maxTokens == 0 {
		maxTokens = 200000
	}
	pct := float64(tokens) / float64(maxTokens) * 100

	return fmt.Sprintf("%s/%dK (%.1f%%)", formatNumber(tokens), maxTokens/1000, pct), nil
}

// formatNumber formats a number with K/M suffixes
func formatNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
