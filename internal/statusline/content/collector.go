package content

import (
	"time"
)

// BaseCollector provides common functionality for collectors
type BaseCollector struct {
	contentType ContentType
	cacheTTL     time.Duration
	optional     bool
}

// Type returns the content type
func (b *BaseCollector) Type() ContentType {
	return b.contentType
}

// CacheTTL returns the cache TTL
func (b *BaseCollector) CacheTTL() time.Duration {
	return b.cacheTTL
}

// Optional returns whether the content is optional
func (b *BaseCollector) Optional() bool {
	return b.optional
}

// NewBaseCollector creates a new base collector
func NewBaseCollector(contentType ContentType, cacheTTL time.Duration, optional bool) *BaseCollector {
	return &BaseCollector{
		contentType: contentType,
		cacheTTL:     cacheTTL,
		optional:     optional,
	}
}
