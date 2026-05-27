package content

import (
	"fmt"
	"os"
	"strings"
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

// Collect returns the current time
func (c *CurrentTimeCollector) Collect(input interface{}, summary interface{}) (string, error) {
	return fmt.Sprintf("🕐 %s", time.Now().Format("2006-01-02 15:04")), nil
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
