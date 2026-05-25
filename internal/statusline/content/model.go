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

// contextPercentColor maps a context-window utilisation percentage to its
// ANSI colour code. The thresholds are tuned for AutoCompact behaviour
// (Claude Code compacts around 75%), so red kicks in at 60% to give the
// user a few turns of warning. This is the OPPOSITE direction from
// quotaPercentColor in time.go: a high context percentage is a warning
// signal, whereas a high quota percentage means "less budget remaining" —
// the two scales must not be unified.
func contextPercentColor(pct float64) string {
	switch {
	case pct >= 60:
		return "\x1b[1;31m" // red: AutoCompact imminent
	case pct >= 40:
		return "\x1b[1;33m" // yellow: heads-up
	case pct >= 20:
		return "\x1b[1;36m" // cyan: filling up
	}
	return "\x1b[1;32m" // green: plenty of room
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

	return fmt.Sprintf("[%s%s\x1b[0m%s]", contextPercentColor(pct), filled, empty), nil
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

// Collect returns token usage information. The percentage in parentheses
// shares the bar's 4-tier colour (see contextPercentColor) so the text and
// the bar tell the same story; the absolute token counts stay uncoloured
// because they are reference values, not warning signals.
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

	return fmt.Sprintf("%s/%dK (%s%.1f%%\x1b[0m)", formatNumber(tokens), maxTokens/1000, contextPercentColor(pct), pct), nil
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

// SessionTotalCollector collects session total cost and token usage
type SessionTotalCollector struct {
	*BaseCollector
}

// NewSessionTotalCollector creates a new session total collector
func NewSessionTotalCollector() *SessionTotalCollector {
	return &SessionTotalCollector{
		BaseCollector: NewBaseCollector(ContentSessionTotal, 5*time.Second, true),
	}
}

// Collect returns session total cost and token usage
func (c *SessionTotalCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	totalIn := statusInput.ContextWindow.TotalInputTokens
	totalOut := statusInput.ContextWindow.TotalOutputTokens
	cost := statusInput.Cost.TotalCostUSD

	if totalIn == 0 && totalOut == 0 && cost == 0 {
		return "", nil
	}

	return fmt.Sprintf("\U0001f4b0 $%.2f \u00b7 I:%s O:%s", cost, formatNumber(totalIn), formatNumber(totalOut)), nil
}
