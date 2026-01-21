// Package composers provides built-in content composers for the statusline
package composers

import (
	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
)

// TimeQuotaComposer combines current time and quota into a single display
// Default format: time | quota
type TimeQuotaComposer struct {
	composer content.Composer
}

// NewTimeQuotaComposer creates a new time quota composer with the default format
func NewTimeQuotaComposer() *TimeQuotaComposer {
	return &TimeQuotaComposer{
		composer: content.NewFormatComposer("time-quota", []content.ContentType{
			content.ContentCurrentTime,
			content.ContentQuota,
		}, func(contents map[content.ContentType]string) string {
			timeStr := contents[content.ContentCurrentTime]
			quota := contents[content.ContentQuota]

			line := timeStr
			if quota != "" {
				if line != "" {
					line += " | " + quota
				} else {
					line = quota
				}
			}
			return line
		}),
	}
}

// NewTimeQuotaComposerTimeOnly creates a time quota composer that only shows time
func NewTimeQuotaComposerTimeOnly() *TimeQuotaComposer {
	return &TimeQuotaComposer{
		composer: content.NewFormatComposer("time-only", []content.ContentType{
			content.ContentCurrentTime,
		}, func(contents map[content.ContentType]string) string {
			return contents[content.ContentCurrentTime]
		}),
	}
}

// Name returns the composer's name
func (c *TimeQuotaComposer) Name() string {
	return c.composer.Name()
}

// InputTypes returns the content types this composer consumes
func (c *TimeQuotaComposer) InputTypes() []content.ContentType {
	return c.composer.InputTypes()
}

// Compose combines the time-related contents
func (c *TimeQuotaComposer) Compose(contents map[content.ContentType]string) string {
	return c.composer.Compose(contents)
}

// TimeQuotaComposerConfig represents configuration for a custom time quota composer
type TimeQuotaComposerConfig struct {
	Name      string
	ShowQuota bool
	Separator string // Default: " | "
}

// NewTimeQuotaComposerFromConfig creates a time quota composer from configuration
func NewTimeQuotaComposerFromConfig(cfg TimeQuotaComposerConfig) *TimeQuotaComposer {
	inputTypes := []content.ContentType{content.ContentCurrentTime}
	if cfg.ShowQuota {
		inputTypes = append(inputTypes, content.ContentQuota)
	}

	sep := cfg.Separator
	if sep == "" {
		sep = " | "
	}

	return &TimeQuotaComposer{
		composer: content.NewFormatComposer(cfg.Name, inputTypes, func(contents map[content.ContentType]string) string {
			timeStr := contents[content.ContentCurrentTime]
			quota := contents[content.ContentQuota]

			line := timeStr
			if cfg.ShowQuota && quota != "" {
				if line != "" {
					line += sep + quota
				} else {
					line = quota
				}
			}
			return line
		}),
	}
}
