package content

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Test injection points for the timezone-name resolution path. Kept here
// (not in quota_*) so reviewers see "tz logic and its overrides together".
var (
	readlinkFn = os.Readlink                                       // Override os.Readlink in tests
	timeZoneFn = func() (string, int) { return time.Now().Zone() } // Override in tests
)

// CurrentTimeCollector collects the current time
type CurrentTimeCollector struct {
	*BaseCollector
}

// NewCurrentTimeCollector creates a new current time collector
func NewCurrentTimeCollector() *CurrentTimeCollector {
	return &CurrentTimeCollector{
		BaseCollector: NewBaseCollector(ContentCurrentTime, 1*time.Second, false),
	}
}

// idleWarnThreshold is how long a session can sit with no transcript activity
// before the time cell flags it as stale. Default 5 minutes; replaced at
// startup via SetIdleWarnThreshold from config (STATUSLINE_IDLE_WARN_SECONDS
// env or format.idleWarnSeconds YAML). <= 0 disables the marker entirely.
//
// Note: the statusline only refreshes on Claude Code triggers (new message /
// token change), so this is a staleness hint seen on the next refresh — not a
// live watchdog that fires while idle.
var (
	idleWarnThreshold   = 5 * time.Minute
	idleWarnThresholdMu sync.RWMutex
)

// SetIdleWarnThreshold configures the inactivity threshold for the time-cell
// ⏰ marker. Called once at startup from main.go with the resolved config.
func SetIdleWarnThreshold(d time.Duration) {
	idleWarnThresholdMu.Lock()
	defer idleWarnThresholdMu.Unlock()
	idleWarnThreshold = d
}

func getIdleWarnThreshold() time.Duration {
	idleWarnThresholdMu.RLock()
	defer idleWarnThresholdMu.RUnlock()
	return idleWarnThreshold
}

// Collect returns the current time, with a "⏰ Xm" staleness marker appended
// when no transcript activity has been seen for more than idleWarnThreshold.
// "Last activity" is the newest transcript entry timestamp (SessionEnd); a
// zero SessionEnd (no transcript parsed yet) yields no marker rather than a
// false alarm. nowFn (not time.Now) is used so the displayed time and the
// idle calculation stay consistent and both are pinnable in tests.
func (c *CurrentTimeCollector) Collect(_ *StatusLineInput, summary *TranscriptSummary) (string, error) {
	return fmt.Sprintf("🕐 %s%s", nowFn().Format("2006-01-02 15:04"), idleSuffix(summary)), nil
}

// idleSuffix returns the coloured staleness marker for the time cell, or ""
// when the session is still active, the marker is disabled, or there is no
// known last-activity timestamp. Yellow once past the threshold, red once
// notably stale (3× threshold — 15 min at the default 5 min, scales if the
// threshold is tuned).
func idleSuffix(summary *TranscriptSummary) string {
	if summary == nil || summary.SessionEnd.IsZero() {
		return ""
	}
	threshold := getIdleWarnThreshold()
	if threshold <= 0 {
		return "" // disabled via config (STATUSLINE_IDLE_WARN_SECONDS=0)
	}
	idle := nowFn().Sub(summary.SessionEnd)
	if idle <= threshold {
		return ""
	}
	color := "\x1b[1;33m" // yellow: session has gone quiet
	if idle >= 3*threshold {
		color = "\x1b[1;31m" // red: notably stale
	}
	return fmt.Sprintf(" %s⏰ %s\x1b[0m", color, formatDuration(idle))
}

// getLocalTimeZoneName attempts to get the IANA timezone name.
//
// Resolution order:
//  1. $TZ env var (stripping the leading ":" some systems use)
//  2. /etc/localtime symlink target — pulls the IANA name out of the path
//     (e.g. /usr/share/zoneinfo/Asia/Shanghai → "Asia/Shanghai")
//  3. UTC if the system reports zero offset
//  4. Synthetic "UTC±H[:MM]" string from the runtime offset as a last resort
//
// The renderer uses this as a tooltip / debug hint, not as a primary signal,
// so we never error out — even an unhelpful "UTC+8" is better than nothing.
func getLocalTimeZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}

	if linkTarget, err := readlinkFn("/etc/localtime"); err == nil {
		if idx := strings.LastIndex(linkTarget, "zoneinfo/"); idx >= 0 {
			return linkTarget[idx+9:]
		}
	}

	_, zoneOffset := timeZoneFn()
	if zoneOffset == 0 {
		return "UTC"
	}

	sign := "+"
	if zoneOffset < 0 {
		sign = "-"
		zoneOffset = -zoneOffset
	}
	zoneHours := zoneOffset / 3600
	zoneMinutes := (zoneOffset % 3600) / 60

	if zoneMinutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, zoneHours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, zoneHours, zoneMinutes)
}
