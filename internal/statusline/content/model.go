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

// standardContextWindowSize is the Anthropic-default cap that AutoCompact is
// calibrated against. Windows at or under this size use percentage-based
// tiers (see contextPercentColor); windows above it (e.g. the 1M extended
// window) use absolute-token tiers (see contextAbsoluteColor) so the warning
// still fires near 200K used — at 1M, 200K is only 20% and would otherwise
// stay green right when the user starts paying for performance and cost
// regressions from a bloated context.
const standardContextWindowSize = 200_000

// contextColor picks the ANSI colour code for the context bar. It dispatches
// on maxTokens so the user sees the right warning for their actual window:
//
//   - maxTokens ≤ 200K (Anthropic default): percentage tiers — see
//     contextPercentColor. AutoCompact at ~75% means red at 60% gives a few
//     turns of warning.
//   - maxTokens > 200K (1M extended window etc.): absolute-token tiers — see
//     contextAbsoluteColor. 200K is the soft compress-now line regardless of
//     how much headroom is left on paper, because beyond ~200K the model
//     starts to drag on speed and cost even if AutoCompact is far away.
//
// Do NOT unify with quotaPercentColor in time.go: there a high percentage
// means "less budget left", here it means "the warning bar is filling" —
// inverted semantics.
func contextColor(tokens, maxTokens int) string {
	if maxTokens <= standardContextWindowSize {
		denom := maxTokens
		if denom <= 0 {
			denom = standardContextWindowSize
		}
		pct := float64(tokens) / float64(denom) * 100
		return contextPercentColor(pct)
	}
	return contextAbsoluteColor(tokens)
}

// contextPercentColor maps a context-window utilisation percentage to its
// ANSI colour code (5 tiers). Used only for windows at or under
// standardContextWindowSize (200K) — see contextColor for the dispatch rule.
// The thresholds are tuned for AutoCompact at 85%: red at 75% gives ~2 turns
// of warning before compaction fires.
func contextPercentColor(pct float64) string {
	switch {
	case pct >= 75:
		return "\x1b[1;31m" // red: AutoCompact imminent
	case pct >= 60:
		return "\x1b[1;33m" // yellow: close to warning zone
	case pct >= 40:
		return "\x1b[1;36m" // cyan: past halfway
	case pct >= 20:
		return "\x1b[1;32m" // green: normal usage
	}
	return "\x1b[1;92m" // bright green: plenty of room
}

// contextAbsoluteColor maps absolute used-token counts to colour tiers for
// extended-window models (>200K total cap). Thresholds are calibrated so the
// 200K mark — where context length starts to degrade speed and inflate cost
// even though the hard cap is far away — lands in yellow ("you should
// compress"), and 250K escalates to red ("compress now"). 180K is the first
// heads-up because below it the user has comfortable headroom.
func contextAbsoluteColor(tokens int) string {
	switch {
	case tokens >= 250_000:
		return "\x1b[1;31m" // red: compress NOW
	case tokens >= 200_000:
		return "\x1b[1;33m" // yellow: should compress soon
	case tokens >= 180_000:
		return "\x1b[1;36m" // cyan: closing in on 200K
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
		maxTokens = standardContextWindowSize
	}
	pct := float64(tokens) / float64(maxTokens) * 100

	barWidth := 10
	fillWidth := int(pct / 100 * float64(barWidth))
	if fillWidth > barWidth {
		fillWidth = barWidth
	}
	// Any non-zero usage must paint at least one filled block, otherwise the
	// tier colour is invisible. This matters most on the 1M extended window,
	// where 9% of 1M (already 90K tokens — not nothing) would otherwise
	// truncate to fillWidth=0 and the bar would render as a bare "░░░░░░░░░░"
	// with no green/cyan/yellow signal at all. Pinned by
	// TestTokenBarCollector_MinimumFillWhenUsed.
	if fillWidth == 0 && tokens > 0 {
		fillWidth = 1
	}
	filled := strings.Repeat("█", fillWidth)
	empty := strings.Repeat("░", barWidth-fillWidth)

	return fmt.Sprintf("[%s%s\x1b[0m%s]", contextColor(tokens, maxTokens), filled, empty), nil
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
		maxTokens = standardContextWindowSize
	}
	pct := float64(tokens) / float64(maxTokens) * 100

	return fmt.Sprintf("%s/%dK (%s%.1f%%\x1b[0m)", formatNumber(tokens), maxTokens/1000, contextColor(tokens, maxTokens), pct), nil
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
