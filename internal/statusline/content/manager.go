package content

import (
	"fmt"
	"sync"
	"time"
)

// Manager manages content collectors and caching
type Manager struct {
	collectors map[ContentType]ContentCollector
	cache      map[ContentType]*cachedContent
	cacheMu    sync.RWMutex
}

// NewManager creates a new content manager
func NewManager() *Manager {
	return &Manager{
		collectors: make(map[ContentType]ContentCollector),
		cache:      make(map[ContentType]*cachedContent),
	}
}

// Register registers a content collector
func (m *Manager) Register(collector ContentCollector) {
	m.collectors[collector.Type()] = collector
}

// RegisterAll registers multiple collectors at once
func (m *Manager) RegisterAll(collectors ...ContentCollector) {
	for _, c := range collectors {
		m.Register(c)
	}
}

// Get retrieves a single content item with caching
func (m *Manager) Get(contentType ContentType, input interface{}, summary interface{}) (string, error) {
	collector, ok := m.collectors[contentType]
	if !ok {
		return "", fmt.Errorf("no collector registered for type: %s", contentType)
	}

	// Check cache
	m.cacheMu.RLock()
	cached, exists := m.cache[contentType]
	m.cacheMu.RUnlock()

	if exists && !cached.isExpired() {
		return cached.value, nil
	}

	// Collect fresh data
	value, err := collector.Collect(input, summary)
	if err != nil {
		return "", err
	}

	// Update cache
	m.cacheMu.Lock()
	m.cache[contentType] = &cachedContent{
		value:     value,
		expiresAt: time.Now().Add(collector.CacheTTL()),
	}
	m.cacheMu.Unlock()

	return value, nil
}

// GetAll retrieves all content items
func (m *Manager) GetAll(input interface{}, summary interface{}) map[ContentType]string {
	result := make(map[ContentType]string)

	for contentType := range m.collectors {
		if value, err := m.Get(contentType, input, summary); err == nil && value != "" {
			result[contentType] = value
		}
	}

	return result
}

// GetOptionalContent returns content for optional collectors that have values
func (m *Manager) GetOptionalContent(input interface{}, summary interface{}) map[ContentType]string {
	result := make(map[ContentType]string)

	for contentType, collector := range m.collectors {
		if collector.Optional() {
			if value, err := m.Get(contentType, input, summary); err == nil && value != "" {
				result[contentType] = value
			}
		} else {
			if value, err := m.Get(contentType, input, summary); err == nil {
				result[contentType] = value
			}
		}
	}

	return result
}

// ClearCache clears all cached content
func (m *Manager) ClearCache() {
	m.cacheMu.Lock()
	m.cache = make(map[ContentType]*cachedContent)
	m.cacheMu.Unlock()
}

// ClearTypeCache clears cache for a specific content type
func (m *Manager) ClearTypeCache(contentType ContentType) {
	m.cacheMu.Lock()
	delete(m.cache, contentType)
	m.cacheMu.Unlock()
}
