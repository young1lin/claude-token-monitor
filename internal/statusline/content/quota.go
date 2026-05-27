package content

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Test injection points used by the quota render path. nowFn keeps
// countdowns deterministic; getSubscriptionUsageFn lets tests bypass the
// provider dispatcher and inject canned UsageData.
var (
	getSubscriptionUsageFn func() *UsageData
	nowFn                  = time.Now
)

// getPlanName determines the plan name from subscriptionType
// Returns empty string for API users (no subscription).
//
// Recognises both Anthropic ("claude-max", "claude-pro", "claude-team") and
// GLM Coding Plan ("max", "pro", "lite") tiers, normalising to title-cased
// names so the renderer can show "[Max]" / "[Pro]" / "[Lite]" / "[Team]"
// without provider-specific branching.
func getPlanName(subscriptionType string) string {
	lower := strings.ToLower(strings.TrimSpace(subscriptionType))
	if strings.Contains(lower, "max") {
		return "Max"
	}
	if strings.Contains(lower, "pro") {
		return "Pro"
	}
	if strings.Contains(lower, "lite") {
		return "Lite"
	}
	if strings.Contains(lower, "team") {
		return "Team"
	}
	// API users don't have subscriptionType or have 'api'
	if subscriptionType == "" || strings.Contains(lower, "api") {
		return ""
	}
	// Unknown subscription type - capitalize first letter
	return strings.ToUpper(subscriptionType[:1]) + subscriptionType[1:]
}

// UsageData holds parsed usage information
type UsageData struct {
	FiveHour        float64
	SevenDay        float64
	FiveHourResetAt time.Time
	SevenDayResetAt time.Time
	APIUnavailable  bool
	APIError        string

	// Provider tags the source so renderer/cache can branch on it. Values:
	// "anthropic", "glm-zhipu", "glm-zai". Empty is treated as "anthropic" so
	// pre-existing cache files load without migration.
	Provider string
	// AccountKey discriminates accounts that share the same Provider. It is
	// the fingerprint computed at the producer (Anthropic: empty;
	// GLM: sha256(token)[:12]). Used by writeUsageCache to route to the
	// right per-account file.
	AccountKey string
	// PlanLevel is the subscription tier in display form ("Max", "Pro",
	// "Lite", ...) or empty for API-key-only accounts.
	PlanLevel string
	// MCP carries the GLM Coding Plan MCP monthly call budget. Nil for
	// providers that don't expose an MCP limit.
	MCP *MCPWindow
	// ExtraWindows holds any limit windows we don't have a dedicated slot
	// for, keeping the renderer forward-compatible with new (unit, number)
	// tuples that show up on future GLM plans.
	ExtraWindows []UsageWindow
}

// UsageWindow is a generic percentage-based usage window for rendering.
type UsageWindow struct {
	Label   string    `json:"label"`
	Percent float64   `json:"percent"`
	ResetAt time.Time `json:"reset_at,omitempty"`
}

// MCPWindow describes a count-denominated budget (e.g. GLM Coding Plan's
// monthly MCP tool-call cap). Separate from UsageWindow because users want to
// see the absolute "42/4000" rather than a near-zero percentage.
type MCPWindow struct {
	Used    int64       `json:"used"`
	Limit   int64       `json:"limit"`
	Percent float64     `json:"percent"`
	ResetAt time.Time   `json:"reset_at,omitempty"`
	Details []MCPDetail `json:"details,omitempty"`
}

// MCPDetail is one row of the per-tool breakdown returned by GLM's
// quota/limit endpoint (modelCode → usage).
type MCPDetail struct {
	Tool  string `json:"tool"`
	Usage int64  `json:"usage"`
}

// QuotaCollector collects subscription quota usage
type QuotaCollector struct {
	*BaseCollector
}

// NewQuotaCollector creates a new quota collector
func NewQuotaCollector() *QuotaCollector {
	return &QuotaCollector{
		BaseCollector: NewBaseCollectorWithTimeout(ContentQuota, 5*time.Minute, 4*time.Second, true),
	}
}

// Collect returns subscription quota usage
func (c *QuotaCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	return getSubscriptionQuota(statusInput), nil
}

// getSubscriptionUsage dispatches to the right provider's usage fetcher based
// on $ANTHROPIC_BASE_URL. Returns nil for "custom" third-party proxies — we
// have no way to query their quota.
func getSubscriptionUsage(input *StatusLineInput) *UsageData {
	switch p := detectProvider(); {
	case p.isGLM():
		return getGLMUsage(input, p)
	case p == providerCustom:
		// Unknown third-party endpoint (router, proxy, mock). Hide the line
		// rather than show stale Anthropic data or fail noisily.
		return nil
	default:
		return getAnthropicUsage(input)
	}
}

// formatResetCountdown renders the duration until a reset as a compact,
// timezone-free countdown. The cascade matches the convention shared by
// every mainstream Claude/Codex statusline (ohugonnot, lee-fuhr, et al.):
//
//	d <= 0          → "now"
//	d < 1m          → "<1m"
//	d < 1h          → "Xm"        (e.g. "45m")
//	d < 24h         → "XhYm"      (e.g. "4h32m")
//	d >= 24h        → "XdYh"      (e.g. "1d22h", "6d4h")
func formatResetCountdown(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	if d < time.Minute {
		return "<1m"
	}
	totalHours := int(d / time.Hour)
	totalMinutes := int(d/time.Minute) % 60
	if totalHours >= 24 {
		return fmt.Sprintf("%dd%dh", totalHours/24, totalHours%24)
	}
	if totalHours >= 1 {
		return fmt.Sprintf("%dh%dm", totalHours, totalMinutes)
	}
	return fmt.Sprintf("%dm", totalMinutes)
}

// getSubscriptionQuota renders subscription quota usage as a "·"-joined list
// of windows, optionally prefixed by a "[Plan]" label. Countdown format is
// timezone-free, so no "(UTC±N)" suffix is needed.
//
// Output shape depends on Provider:
//
//   - Anthropic (legacy): always renders both 5h and 7d, even when zero:
//     📊 [Max] 22% 5h ↻ 4h32m · 2% 7d ↻ 1d22h
//     📊 [Pro] 22% 5h ↻ 4h32m · 2% 7d           // only 5h reset known
//     📊 0% 5h · 0% 7d                          // API user (no PlanLevel)
//
//   - GLM (glm-zhipu / glm-zai): renders only the windows actually present
//     on the plan, plus an MCP segment when applicable. MCP counts are
//     compacted with k/M suffixes; the 🧩 glyph stands in for the literal
//     "MCP " text (think of MCP servers as pluggable tools):
//     📊 [Max] 1% 5h ↻ 4h7m · 🧩 42/4k
//     📊 [Lite] 22% 5h ↻ 4h32m · 50% 7d ↻ 3d4h · 🧩 380/4k
//
// PlanLevel is rendered as a bracketed prefix (matching the existing
// "[glm-5.1]" model tag style) and is followed by a single space — NOT a
// "·" — so the label visually groups with the windows rather than becoming
// a separate "column".
func getSubscriptionQuota(input *StatusLineInput) string {
	var usage *UsageData
	if getSubscriptionUsageFn != nil {
		usage = getSubscriptionUsageFn()
	} else {
		usage = getSubscriptionUsage(input)
	}

	if usage == nil {
		return ""
	}

	now := nowFn()
	// Empty provider means "written by pre-multiprovider code" or "Anthropic
	// path didn't bother tagging itself"; either way render the legacy shape.
	isAnthropic := usage.Provider == "" || usage.Provider == "anthropic"
	// For GLM, plan-level metadata tells us which windows the user is
	// supposed to have — without it, the 5h segment would briefly vanish
	// right after a window resets (API returns percentage=0 with
	// nextResetTime=0 because no token has been spent in the new window
	// yet), which looks like a broken display. See glmPlanWindows.
	glmHas5h, glmHas7d := false, false
	if usage.Provider == "glm-zai" || usage.Provider == "glm-zhipu" {
		glmHas5h, glmHas7d = glmPlanWindows(usage.PlanLevel)
	}
	parts := make([]string, 0, 5)

	// 5h: rendered when Anthropic (legacy invariant), the GLM plan is known
	// to have a 5h window, or there's live data / a known reset time.
	if isAnthropic || glmHas5h || usage.FiveHour > 0 || !usage.FiveHourResetAt.IsZero() {
		parts = append(parts, formatPercentWindow(usage.FiveHour, "5h", usage.FiveHourResetAt, now))
	}

	// 7d: same rule. GLM Max accounts have no weekly window so glmHas7d
	// stays false there; GLM Lite/Pro accounts will have unit=6,number=1
	// and surface here exactly like the Anthropic 7-day window.
	if isAnthropic || glmHas7d || usage.SevenDay > 0 || !usage.SevenDayResetAt.IsZero() {
		parts = append(parts, formatPercentWindow(usage.SevenDay, "7d", usage.SevenDayResetAt, now))
	}

	if usage.MCP != nil {
		parts = append(parts, formatMCPWindow(usage.MCP, now))
	}

	for _, w := range usage.ExtraWindows {
		parts = append(parts, formatPercentWindow(w.Percent, w.Label, w.ResetAt, now))
	}

	if len(parts) == 0 {
		return ""
	}

	body := strings.Join(parts, " · ")
	if label := formatPlanLabel(usage.PlanLevel); label != "" {
		return "📊 " + label + " " + body
	}
	return "📊 " + body
}

// formatPlanLabel turns a raw PlanLevel into the bracketed display form
// ("Max" → "[Max]") used as the leading tag on the quota line. Empty input
// produces an empty string so API-key-only accounts render without a label.
// The plan name is normalized through the same heuristic used for Anthropic
// subscription types, so GLM's "max"/"pro"/"lite" come out title-cased and
// stay visually consistent across providers.
func formatPlanLabel(plan string) string {
	if plan == "" {
		return ""
	}
	name := getPlanName(plan)
	if name == "" {
		return ""
	}
	return "[" + name + "]"
}

// quotaPercentColor returns the ANSI prefix used to colour a quota
// percentage. Thresholds are intentionally inverted relative to the context
// progress bar: for quota, "high percentage" means "less budget remaining",
// so red kicks in at 80% and below that we descend through yellow / cyan /
// green into bright green as the user has more headroom. The bar in
// model.go uses the opposite mapping because there 60% is already near the
// AutoCompact line — do NOT unify the two scales.
//
// Returns the empty string for negative inputs (shouldn't occur in practice,
// but keeps the formatter total).
func quotaPercentColor(pct float64) string {
	switch {
	case pct >= 80:
		return "\x1b[1;31m" // red: out-of-budget warning
	case pct >= 60:
		return "\x1b[1;33m" // yellow: heads-up
	case pct >= 40:
		return "\x1b[1;36m" // cyan: past halfway
	case pct >= 20:
		return "\x1b[1;32m" // green: normal usage
	case pct >= 0:
		return "\x1b[1;92m" // bright green: plenty of headroom
	}
	return ""
}

// colouredPercent renders "%.0f%%" with the quota colour applied. The reset
// code follows so the surrounding text stays uncoloured.
func colouredPercent(pct float64) string {
	if c := quotaPercentColor(pct); c != "" {
		return fmt.Sprintf("%s%.0f%%\x1b[0m", c, pct)
	}
	return fmt.Sprintf("%.0f%%", pct)
}

// formatPercentWindow renders a single "X% label ↻ countdown" segment used
// by both Anthropic and GLM percentage-based windows. The percentage is
// wrapped in a 5-tier ANSI colour (see quotaPercentColor).
//
// A zero resetAt means "we don't have a reset timestamp yet" — typically
// because the API briefly returns nextResetTime=0 right after a window
// resets, before any token has been spent in the new window. We pass that
// through to formatResetCountdown so it renders as "↻ now" rather than
// suppressing the countdown entirely; the latter looked like a partial
// failure (just "0% 5h" with no arrow) and matched neither the Anthropic
// default nor what users expect to see during a fresh window.
func formatPercentWindow(percent float64, label string, resetAt, now time.Time) string {
	var countdown string
	if resetAt.IsZero() {
		countdown = "now"
	} else {
		countdown = formatResetCountdown(resetAt.Sub(now))
	}
	return fmt.Sprintf("%s %s ↻ %s", colouredPercent(percent), label, countdown)
}

// formatMCPWindow renders the GLM Coding Plan MCP segment. MCP is
// count-denominated so "42/4k" is preferred over "1%"; we fall back to
// percentage (with colour) when the absolute limit is unknown. The reset
// countdown is suppressed because the budget rolls monthly — 21 days out is
// not useful statusline info, and showing it just eats horizontal space.
//
// The 🧩 prefix evokes a USB / plug-in, since MCP servers behave like
// pluggable tools attached to the model — it replaces the prior "MCP " text
// to save horizontal space.
func formatMCPWindow(m *MCPWindow, _ time.Time) string {
	if m.Limit > 0 {
		return fmt.Sprintf("🧩 %s/%s", compactCount(m.Used), compactCount(m.Limit))
	}
	return fmt.Sprintf("🧩 %s", colouredPercent(m.Percent))
}

// compactCount formats a non-negative integer with a k/M suffix for values
// at or above 1000. Whole thousands drop the decimal ("4000"→"4k") while
// fractional values get one digit ("1234"→"1.2k"). Negative inputs are
// returned unchanged via the default integer path — they shouldn't occur in
// practice but we don't want a surprise crash.
func compactCount(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs < 1000:
		return strconv.FormatInt(n, 10)
	case abs < 1_000_000:
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		if n%1_000_000 == 0 {
			return fmt.Sprintf("%dM", n/1_000_000)
		}
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}
