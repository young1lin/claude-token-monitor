package content

import (
	"fmt"
	"strings"
	"time"
)

// ANSI colors used to tint the effort tier so the user can tell the levels
// apart at a glance without reading the text label. The scale mirrors the
// token bar's green→cyan→yellow→magenta gradient: low is cheap, max burns
// tokens. Medium is the baseline default but is still rendered (in cyan) so
// the current tier is always visible whenever Claude Code reports one.
const (
	colorEffortMax    = "\x1b[1;35m"   // bright magenta — top tier ("burning tokens")
	colorEffortXHigh  = colorEffortMax // legacy CC name for the same top tier
	colorEffortHigh   = "\x1b[1;33m"   // yellow — elevated cost
	colorEffortMedium = "\x1b[1;36m"   // cyan — baseline default
	colorEffortLow    = "\x1b[1;32m"   // green — cheap
	colorReset        = "\x1b[0m"
)

// ModeFlagsCollector surfaces three small runtime indicators Claude Code has
// been emitting on stdin since 2.1.x: the thinking toggle, the effort tier,
// and fast-mode. Each is independent; the collector concatenates whichever
// are present and hides itself when nothing is worth saying.
//
// Output examples:
//
//	"💭 xhigh"     thinking on, effort xhigh
//	"💭 medium"    thinking on, effort medium (baseline tier, still shown)
//	"⚡ low"        fast mode, effort low
//	"💭 ⚡ high"    full combo
//	""             nothing reported → cell is hidden by the layout (Optional: true)
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

// effortChip renders whichever effort tier Claude Code reports. Every tier —
// including the "medium" baseline — is shown so the current effort is always
// visible; only an absent/empty level (older CC that doesn't emit the field)
// is hidden. Known tiers carry a color so users register the cost before
// reading the word, while an unrecognised future tier surfaces its raw label
// (uncolored) rather than disappearing — so a CC rename never silently breaks
// the chip again.
func effortChip(level string) string {
	norm := strings.ToLower(strings.TrimSpace(level))
	switch norm {
	case "": // older CC didn't report a tier — nothing to show
		return ""
	case "max":
		return colorEffortMax + "max" + colorReset
	case "xhigh":
		return colorEffortXHigh + "xhigh" + colorReset
	case "high":
		return colorEffortHigh + "high" + colorReset
	case "medium":
		return colorEffortMedium + "medium" + colorReset
	case "low":
		return colorEffortLow + "low" + colorReset
	default:
		// A tier CC may add later: show the raw label instead of hiding it.
		return norm
	}
}
