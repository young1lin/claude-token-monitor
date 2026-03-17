package content

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTempHomeDir sets a temp home dir, restores the old value on test cleanup.
func setupTempHomeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := overrideHomeDir
	overrideHomeDir = dir
	t.Cleanup(func() { overrideHomeDir = old })
	return dir
}

// writeTestCacheFile serialises cache into <homeDir>/.claude/<usageCacheFile>.
func writeTestCacheFile(t *testing.T, homeDir string, cache *usageCacheData) {
	t.Helper()
	claudeDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	data, err := json.Marshal(cache)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, usageCacheFile), data, 0644))
}

// setupTestAPIServer starts an httptest.Server and overrides usageAPIURL.
// The original URL is restored automatically via t.Cleanup.
func setupTestAPIServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	oldURL := usageAPIURL
	usageAPIURL = srv.URL
	t.Cleanup(func() {
		usageAPIURL = oldURL
		srv.Close()
	})
	return srv
}

// ---------------------------------------------------------------------------
// fallbackOrNil
// ---------------------------------------------------------------------------

func TestFallbackOrNil(t *testing.T) {
	t.Run("Nil cache returns nil", func(t *testing.T) {
		// Arrange / Act / Assert
		assert.Nil(t, fallbackOrNil(nil))
	})

	t.Run("Zero data returns nil", func(t *testing.T) {
		cache := &usageCacheData{FiveHour: 0, SevenDay: 0}
		assert.Nil(t, fallbackOrNil(cache))
	})

	t.Run("Only FiveHour set returns data", func(t *testing.T) {
		cache := &usageCacheData{FiveHour: 42.5}
		result := fallbackOrNil(cache)
		require.NotNil(t, result)
		assert.Equal(t, 42.5, result.FiveHour)
		assert.Equal(t, 0.0, result.SevenDay)
	})

	t.Run("Only SevenDay set returns data", func(t *testing.T) {
		cache := &usageCacheData{SevenDay: 75.0}
		result := fallbackOrNil(cache)
		require.NotNil(t, result)
		assert.Equal(t, 75.0, result.SevenDay)
	})

	t.Run("APIError field is preserved", func(t *testing.T) {
		cache := &usageCacheData{FiveHour: 10.0, APIError: "rate-limited"}
		result := fallbackOrNil(cache)
		require.NotNil(t, result)
		assert.Equal(t, "rate-limited", result.APIError)
	})

	t.Run("APIUnavailable field is preserved", func(t *testing.T) {
		cache := &usageCacheData{SevenDay: 5.0, APIUnavailable: true}
		result := fallbackOrNil(cache)
		require.NotNil(t, result)
		assert.True(t, result.APIUnavailable)
	})
}

// ---------------------------------------------------------------------------
// getCachePath
// ---------------------------------------------------------------------------

func TestGetCachePath(t *testing.T) {
	// Arrange
	homeDir := "/tmp/testhome"

	// Act
	got := getCachePath(homeDir)

	// Assert
	expected := filepath.Join(homeDir, ".claude", usageCacheFile)
	assert.Equal(t, expected, got)
}

// ---------------------------------------------------------------------------
// readUsageCache
// ---------------------------------------------------------------------------

func TestReadUsageCache_FileNotExists(t *testing.T) {
	// Arrange
	setupTempHomeDir(t)

	// Act
	cache := readUsageCache()

	// Assert
	assert.Nil(t, cache)
}

func TestReadUsageCache_CorruptedJSON(t *testing.T) {
	// Arrange
	homeDir := setupTempHomeDir(t)
	claudeDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, usageCacheFile), []byte("not-json{{{"), 0644))

	// Act
	cache := readUsageCache()

	// Assert
	assert.Nil(t, cache)
}

func TestReadUsageCache_ValidFile(t *testing.T) {
	// Arrange
	homeDir := setupTempHomeDir(t)
	now := time.Now().UTC().Truncate(time.Second)
	original := &usageCacheData{
		FiveHour:  55.5,
		SevenDay:  30.0,
		FetchedAt: now,
	}
	writeTestCacheFile(t, homeDir, original)

	// Act
	cache := readUsageCache()

	// Assert
	require.NotNil(t, cache)
	assert.InDelta(t, 55.5, cache.FiveHour, 0.001)
	assert.InDelta(t, 30.0, cache.SevenDay, 0.001)
	assert.Equal(t, now.Unix(), cache.FetchedAt.Unix())
}

// ---------------------------------------------------------------------------
// writeUsageCache + readUsageCache (round-trip)
// ---------------------------------------------------------------------------

func TestWriteAndReadUsageCache(t *testing.T) {
	// Arrange
	setupTempHomeDir(t)
	now := time.Now().UTC().Truncate(time.Second)
	lastGood := &usageCacheData{FiveHour: 80.0, SevenDay: 50.0}
	original := &usageCacheData{
		FiveHour:         60.0,
		SevenDay:         40.0,
		FetchedAt:        now,
		APIError:         "rate-limited",
		RateLimitedCount: 2,
		LastGoodData:     lastGood,
	}

	// Act
	err := writeUsageCache(original)
	require.NoError(t, err)
	got := readUsageCache()

	// Assert
	require.NotNil(t, got)
	assert.InDelta(t, 60.0, got.FiveHour, 0.001)
	assert.InDelta(t, 40.0, got.SevenDay, 0.001)
	assert.Equal(t, "rate-limited", got.APIError)
	assert.Equal(t, 2, got.RateLimitedCount)
	require.NotNil(t, got.LastGoodData)
	assert.InDelta(t, 80.0, got.LastGoodData.FiveHour, 0.001)
}

// ---------------------------------------------------------------------------
// shouldRefreshResult
// ---------------------------------------------------------------------------

func TestShouldRefreshResult_NoCache(t *testing.T) {
	// Arrange: temp home with no cache file
	setupTempHomeDir(t)

	// Act
	shouldRefresh, cache, isBackoff := shouldRefreshResult()

	// Assert
	assert.True(t, shouldRefresh)
	assert.Nil(t, cache)
	assert.False(t, isBackoff)
}

func TestShouldRefreshResult_FreshSuccessCache(t *testing.T) {
	// Arrange: cache written 30s ago (within 60s TTL)
	homeDir := setupTempHomeDir(t)
	c := &usageCacheData{
		FiveHour:  20.0,
		FetchedAt: time.Now().Add(-30 * time.Second),
	}
	writeTestCacheFile(t, homeDir, c)

	// Act
	shouldRefresh, cache, isBackoff := shouldRefreshResult()

	// Assert
	assert.False(t, shouldRefresh)
	require.NotNil(t, cache)
	assert.False(t, isBackoff)
}

func TestShouldRefreshResult_FreshFailureCache(t *testing.T) {
	// Arrange: failure cache written 10s ago (within 15s failure TTL)
	homeDir := setupTempHomeDir(t)
	c := &usageCacheData{
		FetchedAt: time.Now().Add(-10 * time.Second),
		APIError:  "network",
	}
	writeTestCacheFile(t, homeDir, c)

	// Act
	shouldRefresh, cache, isBackoff := shouldRefreshResult()

	// Assert
	assert.False(t, shouldRefresh)
	require.NotNil(t, cache)
	assert.False(t, isBackoff)
}

func TestShouldRefreshResult_ExpiredCache(t *testing.T) {
	// Arrange: cache written 2 minutes ago (past 60s TTL)
	homeDir := setupTempHomeDir(t)
	c := &usageCacheData{
		FiveHour:  10.0,
		FetchedAt: time.Now().Add(-2 * time.Minute),
	}
	writeTestCacheFile(t, homeDir, c)

	// Act
	shouldRefresh, _, isBackoff := shouldRefreshResult()

	// Assert: should trigger refresh
	assert.True(t, shouldRefresh)
	assert.False(t, isBackoff)
}

func TestShouldRefreshResult_RateLimitBackoff_WithLastGoodData(t *testing.T) {
	// Arrange: rate-limited, RetryAfterUntil in the future, has LastGoodData
	homeDir := setupTempHomeDir(t)
	lastGood := &usageCacheData{FiveHour: 50.0, SevenDay: 20.0}
	c := &usageCacheData{
		FetchedAt:        time.Now().Add(-5 * time.Second),
		APIError:         "rate-limited",
		RateLimitedCount: 1,
		RetryAfterUntil:  time.Now().Add(55 * time.Second), // still in backoff
		LastGoodData:     lastGood,
	}
	writeTestCacheFile(t, homeDir, c)

	// Act
	shouldRefresh, cache, isBackoff := shouldRefreshResult()

	// Assert: serve last good data during backoff
	assert.False(t, shouldRefresh)
	assert.True(t, isBackoff)
	require.NotNil(t, cache)
	assert.InDelta(t, 50.0, cache.FiveHour, 0.001)
}

func TestShouldRefreshResult_RateLimitBackoff_NoLastGoodData(t *testing.T) {
	// Arrange: rate-limited, RetryAfterUntil in the future, no LastGoodData
	homeDir := setupTempHomeDir(t)
	c := &usageCacheData{
		FetchedAt:        time.Now().Add(-5 * time.Second),
		APIError:         "rate-limited",
		RateLimitedCount: 1,
		RetryAfterUntil:  time.Now().Add(55 * time.Second),
	}
	writeTestCacheFile(t, homeDir, c)

	// Act
	shouldRefresh, cache, isBackoff := shouldRefreshResult()

	// Assert: serve cache itself (no last good data available)
	assert.False(t, shouldRefresh)
	assert.True(t, isBackoff)
	require.NotNil(t, cache)
}

func TestShouldRefreshResult_RateLimitExpired(t *testing.T) {
	// Arrange: rate-limited, RetryAfterUntil already passed
	homeDir := setupTempHomeDir(t)
	c := &usageCacheData{
		FetchedAt:        time.Now().Add(-2 * time.Minute),
		APIError:         "rate-limited",
		RateLimitedCount: 1,
		RetryAfterUntil:  time.Now().Add(-30 * time.Second), // backoff expired
	}
	writeTestCacheFile(t, homeDir, c)

	// Act
	shouldRefresh, _, isBackoff := shouldRefreshResult()

	// Assert: backoff expired, trigger refresh
	assert.True(t, shouldRefresh)
	assert.False(t, isBackoff)
}

// ---------------------------------------------------------------------------
// writeRefreshedCache
// ---------------------------------------------------------------------------

func TestWriteRefreshedCache_ValidData(t *testing.T) {
	// Arrange
	setupTempHomeDir(t)
	usage := &UsageData{FiveHour: 65.0, SevenDay: 30.0}
	oldCache := &usageCacheData{
		RateLimitedCount: 3,
		LastGoodData:     &usageCacheData{FiveHour: 10.0},
	}

	// Act
	err := writeRefreshedCache(usage, oldCache)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	assert.Equal(t, 0, got.RateLimitedCount, "rate limit count should reset to 0 on success")
	assert.Empty(t, got.APIError)
	assert.False(t, got.APIUnavailable)
	require.NotNil(t, got.LastGoodData, "LastGoodData should be updated with new data")
	assert.InDelta(t, 65.0, got.LastGoodData.FiveHour, 0.001)
}

func TestWriteRefreshedCache_ZeroData_PreservesOldLastGoodData(t *testing.T) {
	// Arrange: API returned zero usage — old LastGoodData should be preserved
	setupTempHomeDir(t)
	usage := &UsageData{FiveHour: 0, SevenDay: 0}
	old := &usageCacheData{
		LastGoodData: &usageCacheData{FiveHour: 88.0, SevenDay: 44.0},
	}

	// Act
	err := writeRefreshedCache(usage, old)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	require.NotNil(t, got.LastGoodData, "LastGoodData from old cache should be preserved")
	assert.InDelta(t, 88.0, got.LastGoodData.FiveHour, 0.001)
}

func TestWriteRefreshedCache_NilOldCache(t *testing.T) {
	// Arrange: no previous cache (nil)
	setupTempHomeDir(t)
	usage := &UsageData{FiveHour: 12.0}

	// Act — should not panic
	err := writeRefreshedCache(usage, nil)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	assert.InDelta(t, 12.0, got.FiveHour, 0.001)
}

// ---------------------------------------------------------------------------
// writeRefreshFailedCache
// ---------------------------------------------------------------------------

func TestWriteRefreshFailedCache_RateLimit_ExplicitRetryAfter(t *testing.T) {
	// Arrange: 429 with explicit Retry-After of 90 seconds
	setupTempHomeDir(t)
	oldCache := &usageCacheData{FiveHour: 30.0, RateLimitedCount: 0}
	before := time.Now()

	// Act
	err := writeRefreshFailedCache(oldCache, true, 90)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	assert.Equal(t, "rate-limited", got.APIError)
	assert.Equal(t, 1, got.RateLimitedCount)
	// RetryAfterUntil should be ~90s from now
	expectedRetry := before.Add(89 * time.Second)
	assert.True(t, got.RetryAfterUntil.After(expectedRetry),
		"RetryAfterUntil=%v should be after %v", got.RetryAfterUntil, expectedRetry)
}

func TestWriteRefreshFailedCache_RateLimit_Backoff(t *testing.T) {
	// Arrange: 429 without Retry-After — use exponential backoff
	setupTempHomeDir(t)
	oldCache := &usageCacheData{FiveHour: 30.0, RateLimitedCount: 1}
	before := time.Now()

	// Act
	err := writeRefreshFailedCache(oldCache, true, 0)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	assert.Equal(t, "rate-limited", got.APIError)
	assert.Equal(t, 2, got.RateLimitedCount)
	// count=2 → TTL = 120s; RetryAfterUntil should be > now+100s
	assert.True(t, got.RetryAfterUntil.After(before.Add(100*time.Second)),
		"Exponential backoff: RetryAfterUntil should be > now+100s")
}

func TestWriteRefreshFailedCache_NetworkError(t *testing.T) {
	// Arrange: network error (not rate-limited)
	setupTempHomeDir(t)
	oldCache := &usageCacheData{FiveHour: 50.0, RateLimitedCount: 3}

	// Act
	err := writeRefreshFailedCache(oldCache, false, 0)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	assert.True(t, got.APIUnavailable)
	assert.Equal(t, "network", got.APIError)
	assert.Equal(t, 0, got.RateLimitedCount, "RateLimitedCount should reset on non-rate-limit error")
}

func TestWriteRefreshFailedCache_NilOldCache_RateLimit(t *testing.T) {
	// Arrange: nil old cache — first-ever failure, rate-limited
	setupTempHomeDir(t)

	// Act — should not panic
	err := writeRefreshFailedCache(nil, true, 0)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	assert.Equal(t, "rate-limited", got.APIError)
	assert.Equal(t, 1, got.RateLimitedCount)
	assert.False(t, got.APIUnavailable)
}

func TestWriteRefreshFailedCache_NilOldCache_Network(t *testing.T) {
	// Arrange: nil old cache — first-ever failure, network error
	setupTempHomeDir(t)

	// Act — should not panic
	err := writeRefreshFailedCache(nil, false, 0)

	// Assert
	require.NoError(t, err)
	got := readUsageCache()
	require.NotNil(t, got)
	assert.True(t, got.APIUnavailable)
	assert.Equal(t, "network", got.APIError)
}

// ---------------------------------------------------------------------------
// fetchUsageAPI (httptest.Server)
// ---------------------------------------------------------------------------

func TestFetchUsageAPI_Success(t *testing.T) {
	// Arrange
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"five_hour": {"utilization": 45.5, "resets_at": "2026-03-17T12:00:00Z"},
			"seven_day": {"utilization": 20.0, "resets_at": "2026-03-24T00:00:00Z"}
		}`))
	})

	// Act
	usage, isRateLimited, retryAfterSec, err := fetchUsageAPI("test-token")

	// Assert
	require.NoError(t, err)
	assert.False(t, isRateLimited)
	assert.Equal(t, 0, retryAfterSec)
	require.NotNil(t, usage)
	assert.InDelta(t, 45.5, usage.FiveHour, 0.001)
	assert.InDelta(t, 20.0, usage.SevenDay, 0.001)
}

func TestFetchUsageAPI_Success_ParsesResetAt(t *testing.T) {
	// Arrange
	resetAt := "2026-03-17T15:30:00Z"
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"five_hour": {"utilization": 10.0, "resets_at": "` + resetAt + `"}}`))
	})

	// Act
	usage, _, _, err := fetchUsageAPI("tok")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, usage)
	expected, _ := time.Parse(time.RFC3339, resetAt)
	assert.Equal(t, expected.Unix(), usage.FiveHourResetAt.Unix())
}

func TestFetchUsageAPI_RateLimit_WithRetryAfter(t *testing.T) {
	// Arrange: server returns 429 with Retry-After header
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "90")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	// Act
	usage, isRateLimited, retryAfterSec, err := fetchUsageAPI("tok")

	// Assert
	assert.Error(t, err)
	assert.True(t, isRateLimited)
	assert.Equal(t, 90, retryAfterSec)
	assert.Nil(t, usage)
}

func TestFetchUsageAPI_RateLimit_NoRetryAfter(t *testing.T) {
	// Arrange: server returns 429 without Retry-After
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	// Act
	usage, isRateLimited, retryAfterSec, err := fetchUsageAPI("tok")

	// Assert
	assert.Error(t, err)
	assert.True(t, isRateLimited)
	assert.Equal(t, 0, retryAfterSec)
	assert.Nil(t, usage)
}

func TestFetchUsageAPI_ServerError(t *testing.T) {
	// Arrange: server returns 500
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	// Act
	usage, isRateLimited, retryAfterSec, err := fetchUsageAPI("tok")

	// Assert
	assert.Error(t, err)
	assert.False(t, isRateLimited)
	assert.Equal(t, 0, retryAfterSec)
	assert.Nil(t, usage)
}

func TestFetchUsageAPI_InvalidJSON(t *testing.T) {
	// Arrange: server returns 200 with non-JSON body
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json-at-all"))
	})

	// Act
	usage, isRateLimited, retryAfterSec, err := fetchUsageAPI("tok")

	// Assert
	assert.Error(t, err)
	assert.False(t, isRateLimited)
	assert.Equal(t, 0, retryAfterSec)
	assert.Nil(t, usage)
}

// ---------------------------------------------------------------------------
// Collector interface
// ---------------------------------------------------------------------------

func TestCurrentTimeCollector_Collect(t *testing.T) {
	// Arrange
	c := NewCurrentTimeCollector()

	// Act
	result, err := c.Collect(nil, nil)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result, "🕐")
	assert.Contains(t, result, ":")
}

func TestQuotaCollector_Collect_InvalidInput(t *testing.T) {
	// Arrange
	c := NewQuotaCollector()

	// Act
	_, err := c.Collect("not a *StatusLineInput", nil)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input type")
}

// ---------------------------------------------------------------------------
// helpers for credential-based tests
// ---------------------------------------------------------------------------

// writeTestCredentials writes a fake .credentials.json to <homeDir>/.claude/
func writeTestCredentials(t *testing.T, homeDir string, token, subscriptionType string, expiresAt int64) {
	t.Helper()
	claudeDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	type oauthBlock struct {
		AccessToken      string `json:"accessToken"`
		SubscriptionType string `json:"subscriptionType"`
		ExpiresAt        int64  `json:"expiresAt"`
	}
	type credsFile struct {
		ClaudeAiOauth *oauthBlock `json:"claudeAiOauth"`
	}
	creds := credsFile{ClaudeAiOauth: &oauthBlock{
		AccessToken:      token,
		SubscriptionType: subscriptionType,
		ExpiresAt:        expiresAt,
	}}
	data, err := json.Marshal(creds)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), data, 0644))
}

// mockSubscriptionUsage replaces getSubscriptionUsageFn for the duration of the test.
func mockSubscriptionUsage(t *testing.T, fn func() *UsageData) {
	t.Helper()
	old := getSubscriptionUsageFn
	getSubscriptionUsageFn = fn
	t.Cleanup(func() { getSubscriptionUsageFn = old })
}

// ---------------------------------------------------------------------------
// getSubscriptionUsage – testing the full credential + API flow
// ---------------------------------------------------------------------------

func TestGetSubscriptionUsage_CustomApiEndpoint(t *testing.T) {
	// Arrange: custom base URL → skip usage API
	t.Setenv("ANTHROPIC_BASE_URL", "https://custom.llm.example.com")
	setupTempHomeDir(t)

	// Act
	result := getSubscriptionUsage()

	// Assert
	assert.Nil(t, result)
}

func TestGetSubscriptionUsage_FreshCache_NoCreds(t *testing.T) {
	// Arrange: fresh cache present → return cached data without touching creds
	homeDir := setupTempHomeDir(t)
	c := &usageCacheData{
		FiveHour:  33.3,
		SevenDay:  11.1,
		FetchedAt: time.Now().Add(-10 * time.Second), // within 60s TTL
	}
	writeTestCacheFile(t, homeDir, c)
	// No credentials file written intentionally – cache hit must not read creds

	// Act
	result := getSubscriptionUsage()

	// Assert: served from cache
	require.NotNil(t, result)
	assert.InDelta(t, 33.3, result.FiveHour, 0.001)
}

func TestGetSubscriptionUsage_NoCredentialsFile(t *testing.T) {
	// Arrange: no cache, no .credentials.json → fallback nil
	setupTempHomeDir(t)

	// Act
	result := getSubscriptionUsage()

	// Assert
	assert.Nil(t, result)
}

func TestGetSubscriptionUsage_InvalidCredentialsJSON(t *testing.T) {
	// Arrange: corrupted credentials file
	homeDir := setupTempHomeDir(t)
	claudeDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte("not-json{{"), 0644))

	// Act
	result := getSubscriptionUsage()

	// Assert
	assert.Nil(t, result)
}

func TestGetSubscriptionUsage_NoAccessToken(t *testing.T) {
	// Arrange: credentials file present but accessToken is empty
	homeDir := setupTempHomeDir(t)
	writeTestCredentials(t, homeDir, "", "claude-pro", 0)

	// Act
	result := getSubscriptionUsage()

	// Assert
	assert.Nil(t, result)
}

func TestGetSubscriptionUsage_ExpiredToken(t *testing.T) {
	// Arrange: token expiry in the past (Unix ms)
	homeDir := setupTempHomeDir(t)
	expiredAt := time.Now().Add(-1*time.Hour).UnixMilli()
	writeTestCredentials(t, homeDir, "stale-token", "claude-pro", expiredAt)

	// Act
	result := getSubscriptionUsage()

	// Assert
	assert.Nil(t, result)
}

func TestGetSubscriptionUsage_APIUser_NoSubscription(t *testing.T) {
	// Arrange: subscriptionType="" → API user, skip usage fetch
	homeDir := setupTempHomeDir(t)
	farFuture := time.Now().Add(24 * time.Hour).UnixMilli()
	writeTestCredentials(t, homeDir, "api-token", "", farFuture)

	// Act
	result := getSubscriptionUsage()

	// Assert: API user → nil (no quota display)
	assert.Nil(t, result)
}

func TestGetSubscriptionUsage_Success(t *testing.T) {
	// Arrange: valid credentials + test server → full success flow
	homeDir := setupTempHomeDir(t)
	farFuture := time.Now().Add(24 * time.Hour).UnixMilli()
	writeTestCredentials(t, homeDir, "valid-token", "claude-pro", farFuture)

	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer valid-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"five_hour": {"utilization": 72.0, "resets_at": "2026-03-17T18:00:00Z"},
			"seven_day": {"utilization": 45.0, "resets_at": "2026-03-24T00:00:00Z"}
		}`))
	})

	// Act
	result := getSubscriptionUsage()

	// Assert
	require.NotNil(t, result)
	assert.InDelta(t, 72.0, result.FiveHour, 0.001)
	assert.InDelta(t, 45.0, result.SevenDay, 0.001)

	// Cache should have been written
	cached := readUsageCache()
	require.NotNil(t, cached)
	assert.InDelta(t, 72.0, cached.FiveHour, 0.001)
}

func TestGetSubscriptionUsage_APIRateLimit_WritesFailureCache(t *testing.T) {
	// Arrange: server returns 429 → failure cache written, nil returned (no old data)
	homeDir := setupTempHomeDir(t)
	farFuture := time.Now().Add(24 * time.Hour).UnixMilli()
	writeTestCredentials(t, homeDir, "token", "claude-pro", farFuture)

	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	// Act
	result := getSubscriptionUsage()

	// Assert: no previous data → nil
	assert.Nil(t, result)

	// Failure cache must have been written
	cached := readUsageCache()
	require.NotNil(t, cached)
	assert.Equal(t, "rate-limited", cached.APIError)
	assert.Equal(t, 1, cached.RateLimitedCount)
}

// ---------------------------------------------------------------------------
// getSubscriptionQuota – mock the usage provider
// ---------------------------------------------------------------------------

func TestGetSubscriptionQuota_NilUsage(t *testing.T) {
	// Arrange
	mockSubscriptionUsage(t, func() *UsageData { return nil })
	input := &StatusLineInput{}

	// Act
	result := getSubscriptionQuota(input)

	// Assert
	assert.Empty(t, result)
}

func TestGetSubscriptionQuota_BothZero(t *testing.T) {
	// Arrange: usage with both fields zero
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{FiveHour: 0, SevenDay: 0}
	})

	// Act
	result := getSubscriptionQuota(&StatusLineInput{})

	// Assert
	assert.Empty(t, result)
}

func TestGetSubscriptionQuota_FiveHourWithResetTime(t *testing.T) {
	// Arrange: five-hour usage with a valid reset time
	resetAt := time.Date(2026, 3, 17, 15, 30, 0, 0, time.UTC)
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{FiveHour: 65.0, FiveHourResetAt: resetAt}
	})

	// Act
	result := getSubscriptionQuota(&StatusLineInput{})

	// Assert
	assert.Contains(t, result, "65%")
	assert.Contains(t, result, "Reset")
	assert.Contains(t, result, ":")
}

func TestGetSubscriptionQuota_FiveHourNoResetTime(t *testing.T) {
	// Arrange: five-hour usage without reset time
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{FiveHour: 80.0} // zero FiveHourResetAt
	})

	// Act
	result := getSubscriptionQuota(&StatusLineInput{})

	// Assert: shows percentage but no "Reset"
	assert.Contains(t, result, "80%")
	assert.NotContains(t, result, "Reset")
}

func TestGetSubscriptionQuota_SevenDayFallback(t *testing.T) {
	// Arrange: five_hour=0, only seven_day has data → label "7d" appended
	resetAt := time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC)
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{FiveHour: 0, SevenDay: 42.0, SevenDayResetAt: resetAt}
	})

	// Act
	result := getSubscriptionQuota(&StatusLineInput{})

	// Assert
	assert.Contains(t, result, "42%")
	assert.Contains(t, result, "7d")
	assert.Contains(t, result, "Reset")
}

func TestGetSubscriptionQuota_BothLimits_WithResetTime(t *testing.T) {
	// Arrange: both five_hour and seven_day present
	resetAt := time.Date(2026, 3, 17, 15, 0, 0, 0, time.UTC)
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{
			FiveHour:        67.0,
			SevenDay:        45.0,
			FiveHourResetAt: resetAt,
		}
	})

	// Act
	result := getSubscriptionQuota(&StatusLineInput{})

	// Assert: both percentages visible with labels
	assert.Contains(t, result, "67%")
	assert.Contains(t, result, "5h")
	assert.Contains(t, result, "45%")
	assert.Contains(t, result, "7d")
	assert.Contains(t, result, "Reset")
}

func TestGetSubscriptionQuota_BothLimits_NoResetTime(t *testing.T) {
	// Arrange: both limits present but no reset time
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{FiveHour: 50.0, SevenDay: 30.0}
	})

	// Act
	result := getSubscriptionQuota(&StatusLineInput{})

	// Assert
	assert.Contains(t, result, "50%")
	assert.Contains(t, result, "5h")
	assert.Contains(t, result, "30%")
	assert.Contains(t, result, "7d")
	assert.NotContains(t, result, "Reset")
}

func TestGetSubscriptionQuota_FormatsPercentageWithoutDecimal(t *testing.T) {
	// Arrange: verify %.0f formatting (no decimal places)
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{FiveHour: 33.7}
	})

	// Act
	result := getSubscriptionQuota(&StatusLineInput{})

	// Assert: rounded to integer
	assert.Contains(t, result, "34%")
	assert.NotContains(t, result, "33.7")
}

func TestGetSubscriptionQuota_ValidInputType(t *testing.T) {
	// Arrange: QuotaCollector.Collect with valid *StatusLineInput
	mockSubscriptionUsage(t, func() *UsageData {
		return &UsageData{FiveHour: 50.0}
	})
	c := NewQuotaCollector()

	// Act
	result, err := c.Collect(&StatusLineInput{}, nil)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result, fmt.Sprintf("%d%%", 50))
}
