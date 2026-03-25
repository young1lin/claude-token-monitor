package content

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubCollector is a test stub that implements ContentCollector.
type stubCollector struct {
	contentType ContentType
	cacheTTL    time.Duration
	timeout     time.Duration
	optional    bool
	collectFunc func(input interface{}, summary interface{}) (string, error)
	callCount   int
	mu          sync.Mutex
}

func (s *stubCollector) Type() ContentType {
	return s.contentType
}

func (s *stubCollector) Collect(input interface{}, summary interface{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	if s.collectFunc != nil {
		return s.collectFunc(input, summary)
	}
	return fmt.Sprintf("stub-%s", s.contentType), nil
}

func (s *stubCollector) CacheTTL() time.Duration {
	return s.cacheTTL
}

func (s *stubCollector) Optional() bool {
	return s.optional
}

func (s *stubCollector) Timeout() time.Duration {
	return s.timeout
}

func (s *stubCollector) getCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

func newStubCollector(ct ContentType, ttl time.Duration, optional bool) *stubCollector {
	return &stubCollector{
		contentType: ct,
		cacheTTL:    ttl,
		optional:    optional,
	}
}

func TestNewManager(t *testing.T) {
	// Act
	m := NewManager()

	// Assert
	require.NotNil(t, m)
	assert.NotNil(t, m.collectors)
	assert.Empty(t, m.collectors)
	assert.NotNil(t, m.composers)
	assert.NotNil(t, m.cache)
	assert.Empty(t, m.cache)
}

func TestManager_Register(t *testing.T) {
	// Arrange
	m := NewManager()
	collector := newStubCollector(ContentModel, 5*time.Second, false)

	// Act
	m.Register(collector)

	// Assert
	assert.Equal(t, collector, m.collectors[ContentModel])
	assert.Len(t, m.collectors, 1)
}

func TestManager_RegisterAll(t *testing.T) {
	// Arrange
	m := NewManager()
	c1 := newStubCollector(ContentModel, 5*time.Second, false)
	c2 := newStubCollector(ContentTokenBar, 5*time.Second, false)
	c3 := newStubCollector(ContentAgent, 5*time.Second, true)

	// Act
	m.RegisterAll(c1, c2, c3)

	// Assert
	assert.Len(t, m.collectors, 3)
	assert.Equal(t, c1, m.collectors[ContentModel])
	assert.Equal(t, c2, m.collectors[ContentTokenBar])
	assert.Equal(t, c3, m.collectors[ContentAgent])
}

func TestManager_Get(t *testing.T) {
	t.Run("unregistered type returns error", func(t *testing.T) {
		// Arrange
		m := NewManager()

		// Act
		_, err := m.Get(ContentModel, nil, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no collector registered for type")
	})

	t.Run("cache miss collects fresh data", func(t *testing.T) {
		// Arrange
		m := NewManager()
		stub := newStubCollector(ContentModel, 5*time.Second, false)
		m.Register(stub)

		// Act
		result, err := m.Get(ContentModel, nil, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "stub-model", result)
		assert.Equal(t, 1, stub.getCallCount())
	})

	t.Run("cache hit returns cached data without re-collecting", func(t *testing.T) {
		// Arrange
		m := NewManager()
		stub := newStubCollector(ContentModel, 5*time.Second, false)
		m.Register(stub)

		// Act - first call populates cache
		result1, err1 := m.Get(ContentModel, nil, nil)
		require.NoError(t, err1)

		// Act - second call should use cache
		result2, err2 := m.Get(ContentModel, nil, nil)
		require.NoError(t, err2)

		// Assert
		assert.Equal(t, result1, result2)
		assert.Equal(t, 1, stub.getCallCount(), "should not re-collect on cache hit")
	})

	t.Run("expired cache re-collects", func(t *testing.T) {
		// Arrange - use a collector with very short TTL
		m := NewManager()
		stub := newStubCollector(ContentModel, 1*time.Nanosecond, false)
		m.Register(stub)

		// Act - first call
		_, err1 := m.Get(ContentModel, nil, nil)
		require.NoError(t, err1)

		// Wait for cache to expire
		time.Sleep(10 * time.Millisecond)

		// Act - second call after expiry
		_, err2 := m.Get(ContentModel, nil, nil)
		require.NoError(t, err2)

		// Assert
		assert.Equal(t, 2, stub.getCallCount(), "should re-collect after cache expiry")
	})

	t.Run("collector error is returned", func(t *testing.T) {
		// Arrange
		m := NewManager()
		stub := newStubCollector(ContentModel, 5*time.Second, false)
		stub.collectFunc = func(input interface{}, summary interface{}) (string, error) {
			return "", fmt.Errorf("collection failed")
		}
		m.Register(stub)

		// Act
		_, err := m.Get(ContentModel, nil, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "collection failed")
	})
}

func TestManager_GetAll(t *testing.T) {
	// Arrange
	m := NewManager()
	m.RegisterAll(
		newStubCollector(ContentModel, 5*time.Second, false),
		newStubCollector(ContentTokenBar, 5*time.Second, false),
		newStubCollector(ContentAgent, 5*time.Second, true),
	)

	// Act
	result := m.GetAll(nil, nil)

	// Assert
	assert.Len(t, result, 3)
	assert.Contains(t, result, ContentModel)
	assert.Contains(t, result, ContentTokenBar)
	assert.Contains(t, result, ContentAgent)
}

func TestManager_Compose(t *testing.T) {
	// Arrange
	m := NewManager()
	m.RegisterAll(
		newStubCollector(ContentModel, 5*time.Second, false),
		newStubCollector(ContentTokenBar, 5*time.Second, false),
	)

	// Act
	result := m.Compose(nil, nil)

	// Assert
	require.NotNil(t, result)
	assert.Contains(t, result, "model", "result should contain model content key")
	assert.Contains(t, result, "token-bar", "result should contain token-bar content key")
}

func TestManager_ClearCache(t *testing.T) {
	// Arrange
	m := NewManager()
	stub := newStubCollector(ContentModel, 5*time.Second, false)
	m.Register(stub)

	// Populate cache
	_, err := m.Get(ContentModel, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, stub.getCallCount())

	// Act
	m.ClearCache()

	// Assert - next call should re-collect
	_, err = m.Get(ContentModel, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, stub.getCallCount(), "should re-collect after cache clear")
}

func TestManager_ClearTypeCache(t *testing.T) {
	// Arrange
	m := NewManager()
	stub1 := newStubCollector(ContentModel, 5*time.Second, false)
	stub2 := newStubCollector(ContentTokenBar, 5*time.Second, false)
	m.RegisterAll(stub1, stub2)

	// Populate caches
	_, err1 := m.Get(ContentModel, nil, nil)
	require.NoError(t, err1)
	_, err2 := m.Get(ContentTokenBar, nil, nil)
	require.NoError(t, err2)

	// Act - clear only one type
	m.ClearTypeCache(ContentModel)

	// Assert - model should re-collect, token-bar should use cache
	_, err3 := m.Get(ContentModel, nil, nil)
	require.NoError(t, err3)
	assert.Equal(t, 2, stub1.getCallCount(), "model should re-collect after partial cache clear")

	_, err4 := m.Get(ContentTokenBar, nil, nil)
	require.NoError(t, err4)
	assert.Equal(t, 1, stub2.getCallCount(), "token-bar should still use cache")
}

// --- Timeout and panic recovery tests ---

func TestGetAll_PanicRecovery(t *testing.T) {
	// Arrange
	m := NewManager()
	normal := newStubCollector(ContentModel, 5*time.Second, false)
	panicking := newStubCollector(ContentAgent, 5*time.Second, false)
	panicking.collectFunc = func(input, summary interface{}) (string, error) {
		panic("collector exploded")
	}
	m.RegisterAll(normal, panicking)

	// Act — should NOT panic
	result := m.GetAll(nil, nil)

	// Assert
	assert.Contains(t, result, ContentModel, "normal collector should succeed")
	assert.NotContains(t, result, ContentAgent, "panicking collector should be skipped")
}

func TestGetAll_TimeoutSkipsSlowCollector(t *testing.T) {
	// Arrange — use a very short timeout so we don't sleep
	old := collectorTimeout
	collectorTimeout = 10 * time.Millisecond
	defer func() { collectorTimeout = old }()

	m := NewManager()
	fast := newStubCollector(ContentModel, 5*time.Second, false)

	slow := newStubCollector(ContentAgent, 5*time.Second, false)
	blockCh := make(chan struct{})
	slow.collectFunc = func(input, summary interface{}) (string, error) {
		<-blockCh // blocks until channel is closed
		return "never", nil
	}
	defer close(blockCh) // cleanup: unblock the goroutine on test exit

	m.RegisterAll(fast, slow)

	// Act
	result := m.GetAll(nil, nil)

	// Assert
	assert.Contains(t, result, ContentModel, "fast collector should succeed")
	assert.NotContains(t, result, ContentAgent, "slow collector should be timed out")
}

func TestGetOptionalContent_PanicRecovery(t *testing.T) {
	// Arrange
	m := NewManager()
	normal := newStubCollector(ContentModel, 5*time.Second, false)
	panicking := newStubCollector(ContentAgent, 5*time.Second, true)
	panicking.collectFunc = func(input, summary interface{}) (string, error) {
		panic("optional collector exploded")
	}
	m.RegisterAll(normal, panicking)

	// Act — should NOT panic
	result := m.GetOptionalContent(nil, nil)

	// Assert
	assert.Contains(t, result, ContentModel, "normal collector should succeed")
	assert.NotContains(t, result, ContentAgent, "panicking optional collector should be skipped")
}

func TestGetOptionalContent_TimeoutSkipsSlowCollector(t *testing.T) {
	// Arrange
	old := collectorTimeout
	collectorTimeout = 10 * time.Millisecond
	defer func() { collectorTimeout = old }()

	m := NewManager()
	fast := newStubCollector(ContentModel, 5*time.Second, false)

	slow := newStubCollector(ContentAgent, 5*time.Second, true)
	blockCh := make(chan struct{})
	slow.collectFunc = func(input, summary interface{}) (string, error) {
		<-blockCh
		return "never", nil
	}
	defer close(blockCh)

	m.RegisterAll(fast, slow)

	// Act
	result := m.GetOptionalContent(nil, nil)

	// Assert
	assert.Contains(t, result, ContentModel, "fast collector should succeed")
	assert.NotContains(t, result, ContentAgent, "slow optional collector should be timed out")
}

func TestCollectWithTimeout_NormalCollector(t *testing.T) {
	// Arrange
	m := NewManager()
	stub := newStubCollector(ContentModel, 5*time.Second, false)
	m.Register(stub)

	// Act
	value, ok := m.collectWithTimeout(ContentModel, nil, nil)

	// Assert
	assert.True(t, ok)
	assert.Equal(t, "stub-model", value)
}

func TestCollectWithTimeout_ErrorCollector(t *testing.T) {
	// Arrange
	m := NewManager()
	stub := newStubCollector(ContentModel, 5*time.Second, false)
	stub.collectFunc = func(input, summary interface{}) (string, error) {
		return "", fmt.Errorf("something broke")
	}
	m.Register(stub)

	// Act
	value, ok := m.collectWithTimeout(ContentModel, nil, nil)

	// Assert
	assert.False(t, ok, "error collector should return ok=false")
	assert.Empty(t, value)
}

func TestCollectWithTimeout_CustomCollectorTimeout(t *testing.T) {
	// Arrange — global timeout is very short, but collector has its own longer timeout
	old := collectorTimeout
	collectorTimeout = 10 * time.Millisecond
	defer func() { collectorTimeout = old }()

	m := NewManager()

	// This collector has 2s custom timeout and completes in 50ms
	custom := newStubCollector(ContentQuota, 5*time.Minute, true)
	custom.timeout = 2 * time.Second
	blockCh := make(chan struct{})
	custom.collectFunc = func(input, summary interface{}) (string, error) {
		<-time.After(50 * time.Millisecond)
		close(blockCh)
		return "quota-data", nil
	}
	m.Register(custom)

	// Act — should NOT timeout because collector uses its own 2s timeout
	value, ok := m.collectWithTimeout(ContentQuota, nil, nil)

	// Assert
	assert.True(t, ok, "collector with custom timeout should succeed")
	assert.Equal(t, "quota-data", value)
}

func TestCollectWithTimeout_CustomCollectorTimeoutExceeded(t *testing.T) {
	// Arrange — global timeout is long, but collector has its own short timeout
	old := collectorTimeout
	collectorTimeout = 5 * time.Second
	defer func() { collectorTimeout = old }()

	m := NewManager()

	// This collector has 20ms custom timeout and blocks for 5s
	custom := newStubCollector(ContentQuota, 5*time.Minute, true)
	custom.timeout = 20 * time.Millisecond
	blockCh := make(chan struct{})
	custom.collectFunc = func(input, summary interface{}) (string, error) {
		<-blockCh // blocks
		return "never", nil
	}
	defer close(blockCh)
	m.Register(custom)

	// Act — should timeout because collector's own timeout is 20ms
	value, ok := m.collectWithTimeout(ContentQuota, nil, nil)

	// Assert
	assert.False(t, ok, "collector should timeout based on its custom timeout")
	assert.Empty(t, value)
}

// --- Composer tests ---

func TestManager_RegisterComposer(t *testing.T) {
	m := NewManager()
	c := NewSimpleComposer("test", []ContentType{ContentModel}, " ", "", "")

	m.RegisterComposer(c)

	result, ok := m.GetComposer("test")
	assert.True(t, ok)
	assert.Equal(t, c, result)
}

func TestManager_RegisterComposers(t *testing.T) {
	m := NewManager()
	c1 := NewSimpleComposer("a", []ContentType{ContentModel}, " ", "", "")
	c2 := NewSimpleComposer("b", []ContentType{ContentTokenBar}, " ", "", "")

	m.RegisterComposers(c1, c2)

	_, ok1 := m.GetComposer("a")
	_, ok2 := m.GetComposer("b")
	assert.True(t, ok1)
	assert.True(t, ok2)
}

func TestManager_GetComposer_NotFound(t *testing.T) {
	m := NewManager()

	_, ok := m.GetComposer("nonexistent")
	assert.False(t, ok)
}

func TestManager_GetOptionalContent(t *testing.T) {
	t.Run("optional collector with empty value is excluded", func(t *testing.T) {
		m := NewManager()
		stub := newStubCollector(ContentAgent, 5*time.Second, true)
		stub.collectFunc = func(input interface{}, summary interface{}) (string, error) {
			return "", nil // empty optional value
		}
		m.Register(stub)

		result := m.GetOptionalContent(nil, nil)
		assert.Empty(t, result)
	})

	t.Run("optional collector with value is included", func(t *testing.T) {
		m := NewManager()
		stub := newStubCollector(ContentAgent, 5*time.Second, true)
		m.Register(stub)

		result := m.GetOptionalContent(nil, nil)
		assert.Contains(t, result, ContentAgent)
		assert.Equal(t, "stub-agent", result[ContentAgent])
	})

	t.Run("non-optional collector is always included even if empty", func(t *testing.T) {
		m := NewManager()
		stub := newStubCollector(ContentModel, 5*time.Second, false)
		stub.collectFunc = func(input interface{}, summary interface{}) (string, error) {
			return "", nil // empty non-optional value
		}
		m.Register(stub)

		result := m.GetOptionalContent(nil, nil)
		assert.Contains(t, result, ContentModel, "non-optional should be included even if empty")
		assert.Equal(t, "", result[ContentModel])
	})
}

func TestManager_Compose_WithComposers(t *testing.T) {
	m := NewManager()
	m.RegisterAll(
		newStubCollector(ContentGitBranch, 5*time.Second, false),
		newStubCollector(ContentGitStatus, 5*time.Second, false),
	)

	// Register a composer that combines git-branch and git-status
	m.RegisterComposer(NewSimpleComposer("git-info", []ContentType{ContentGitBranch, ContentGitStatus}, " | ", "", ""))

	result := m.Compose(nil, nil)

	require.NotNil(t, result)
	assert.Contains(t, result, "git-branch", "should contain individual content key")
	assert.Contains(t, result, "git-status", "should contain individual content key")
	assert.Contains(t, result, "git-info", "should contain composed content key")
	assert.Equal(t, "stub-git-branch | stub-git-status", result["git-info"])
}
