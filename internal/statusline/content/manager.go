package content

import (
	"fmt"
	"sync"
	"time"

	"github.com/young1lin/claude-token-monitor/internal/statusline/layout"
)

// Manager manages content collectors, composers, and caching
type Manager struct {
	collectors   map[ContentType]ContentCollector
	composers    *Registry
	cache        map[ContentType]*cachedContent
	cacheMu      sync.RWMutex
}

// NewManager creates a new content manager
func NewManager() *Manager {
	return &Manager{
		collectors: make(map[ContentType]ContentCollector),
		composers:  NewRegistry(),
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

// RegisterComposer registers a composer
func (m *Manager) RegisterComposer(composer Composer) {
	m.composers.Register(composer)
}

// RegisterComposers registers multiple composers at once
func (m *Manager) RegisterComposers(composers ...Composer) {
	for _, c := range composers {
		m.RegisterComposer(c)
	}
}

// GetComposer retrieves a composer by name
func (m *Manager) GetComposer(name string) (Composer, bool) {
	return m.composers.Get(name)
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

// Compose retrieves all content and applies composers to generate combined content
// This returns a CellContent map suitable for use with the layout system
func (m *Manager) Compose(input interface{}, summary interface{}) layout.CellContent {
	// First, get all individual content pieces
	individualContent := m.GetAll(input, summary)

	// Also get optional content
	optionalContent := m.GetOptionalContent(input, summary)

	// Merge them together
	for k, v := range optionalContent {
		individualContent[k] = v
	}

	// Build the result map with ALL individual content
	result := make(layout.CellContent)
	for contentType, value := range individualContent {
		result[string(contentType)] = value
	}

	// Apply composers to generate combined content (kept alongside individual content)
	for _, composerName := range m.composers.List() {
		composer, _ := m.composers.Get(composerName)

		// Gather input content for this composer
		inputContent := make(map[ContentType]string)
		for _, ct := range composer.InputTypes() {
			if val, ok := individualContent[ct]; ok {
				inputContent[ct] = val
			}
		}

		// Compose and add to result (alongside individual items)
		composed := composer.Compose(inputContent)
		if composed != "" {
			result[composerName] = composed
			// NOTE: We keep individual items - layout system can choose which to display
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
