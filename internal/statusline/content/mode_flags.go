package content

import (
	"fmt"
	"strings"
	"time"
)

// ANSI colors used to tint the effort tier so the user can tell xhigh/high/low
// apart at a glance without reading the text label. Medium is intentionally
// not colored because we never render it (see effortChip below).
const (
	colorEffortXHigh = "\x1b[1;35m" // bright magenta — "burning tokens"
	colorEffortHigh  = "\x1b[1;33m" // yellow — elevated cost
	colorEffortLow   = "\x1b[1;32m" // green — cheap
	colorReset       = "\x1b[0m"
)

// ModeFlagsCollector surfaces three small runtime indicators Claude Code has
// been emitting on stdin since 2.1.x: the thinking toggle, the effort tier,
// and fast-mode. Each is independent; the collector concatenates whichever
// are non-default and hides itself when nothing is worth saying.
//
// Output examples:
//
//	"💭 xhigh"   thinking on, effort xhigh
//	"💭"          thinking on, effort medium (default → label suppressed)
//	"⚡"          fast mode only
//	"💭 ⚡ high"  full combo
//	""           all defaults → cell is hidden by the layout (Optional: true)
type ModeFlagsCollector struct {
	*BaseCollector
}

// NewModeFlagsCollector creates a new mode-flags collector. Cache TTL is
// short because the flags can flip every prompt (e.g. user toggling thinking
// mid-session); Optional=true lets the grid hide the cell when output is
// empty so default-config users don't see a stray separator.
func NewModeFlagsCollector() *ModeFlagsCollector {
	return &ModeFlagsCollector{
		BaseCollector: NewBaseCollector(ContentModeFlags, 1*time.Second, true),
	}
}

// Collect builds the indicator string from the stdin payload. Returns an
// empty string when no flag is worth showing so the cell drops out of the
// rendered grid.
func (c *ModeFlagsCollector) Collect(input interface{}, _ interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok || statusInput == nil {
		return "", fmt.Errorf("invalid input type")
	}
	return buildModeFlags(statusInput), nil
}

// buildModeFlags is the pure-function core of the collector, exported to
// the package so tests can hit it without constructing the BaseCollector.
func buildModeFlags(in *StatusLineInput) string {
	parts := make([]string, 0, 3)
	if in.Thinking.Enabled {
		parts = append(parts, "💭")
	}
	if in.FastMode {
		parts = append(parts, "⚡")
	}
	if chip := effortChip(in.Effort.Level); chip != "" {
		parts = append(parts, chip)
	}
	return strings.Join(parts, " ")
}

// effortChip renders the effort tier when it diverges from the implicit
// "medium" default. Returns "" for medium / empty / unknown so we don't
// pollute the statusline with no-op chips. Tiers are colored so users
// register the warning before reading the word.
func effortChip(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "xhigh":
		return colorEffortXHigh + "xhigh" + colorReset
	case "high":
		return colorEffortHigh + "high" + colorReset
	case "low":
		return colorEffortLow + "low" + colorReset
	default:
		// "medium", "", or anything CC adds in the future we don't recognise.
		return ""
	}
}
