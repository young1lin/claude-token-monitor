package content

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// testOverrides holds values that can be overridden during testing.
// Empty string means "use the real value".
var (
	overrideHomeDir        string // Override os.UserHomeDir() in tests
	usageAPIURL            = "https://api.anthropic.com/api/oauth/usage"
	getSubscriptionUsageFn func() *UsageData                                   // Override getSubscriptionUsage in tests (nil = real impl)
	syncFileFn             = syncFile                                          // Override file sync in tests (nil = no-op)
	currentOS              = runtime.GOOS                                      // Override runtime.GOOS in tests for cross-platform coverage
	readlinkFn             = os.Readlink                                       // Override os.Readlink in tests
	timeZoneFn             = func() (string, int) { return time.Now().Zone() } // Override in tests
	getHomeDirFn           = getEffectiveHomeDir                               // Override in tests for error injection
)

// syncFile opens a file, calls Sync(), and closes it. Returns sync error if any.
func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// getEffectiveHomeDir returns the home directory, allowing test override.
func getEffectiveHomeDir() (string, error) {
	if overrideHomeDir != "" {
		return overrideHomeDir, nil
	}
	return os.UserHomeDir()
}

// Cross-process cache constants (aligned with claude-hud)
const (
	usageCacheFile         = ".usage-cache.json"
	refreshTimeout         = 10 * time.Second // Refresh timeout, prevents stale locks from crashes
	refreshCoordDelay      = 50 * time.Millisecond
	cacheTTLSeconds        = 60  // Success cache TTL (claude-hud default)
	failureCacheTTLSeconds = 15  // Failure cache TTL
	rateLimitBaseSeconds   = 60  // 429 base backoff
	rateLimitMaxSeconds    = 300 // 429 max backoff (5 min)
	httpTimeoutSeconds     = 15  // HTTP client timeout for API calls
)

// isUsingCustomApiEndpoint checks if user is using a custom API endpoint
// When using custom providers, the OAuth usage API is not applicable.
func isUsingCustomApiEndpoint() bool {
	baseURL := strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("ANTHROPIC_API_BASE_URL"))
	}
	if baseURL == "" {
		return false
	}
	// Check if it's not the default Anthropic API
	return !strings.HasPrefix(baseURL, "https://api.anthropic.com")
}

// getPlanName determines the plan name from subscriptionType
// Returns empty string for API users (no subscription)
func getPlanName(subscriptionType string) string {
	lower := strings.ToLower(strings.TrimSpace(subscriptionType))
	if strings.Contains(lower, "max") {
		return "Max"
	}
	if strings.Contains(lower, "pro") {
		return "Pro"
	}
	if strings.Contains(lower, "team") {
		return "Team"
	}
	// API users don't have subscriptionType or have 'api'
	if subscriptionType == "" || strings.Contains(lower, "api") {
		return ""
	}
	// Unknown subscription type - capitalize first letter
	return strings.ToUpper(subscriptionType[:1]) + subscriptionType[1:]
}

// usageCacheData represents the file-based cache structure
type usageCacheData struct {
	FiveHour        float64   `json:"five_hour"`
	SevenDay        float64   `json:"seven_day"`
	FiveHourResetAt time.Time `json:"five_hour_reset_at"`
	SevenDayResetAt time.Time `json:"seven_day_reset_at"`
	FetchedAt       time.Time `json:"fetched_at"`
	RefreshingSince time.Time `json:"refreshing_since,omitempty"` // Refresh start time (crash recovery)
	APIUnavailable  bool      `json:"api_unavailable,omitempty"`
	APIError        string    `json:"api_error,omitempty"` // "rate-limited", "network", "http-429", etc.

	// 429 rate limit backoff (aligned with claude-hud)
	RateLimitedCount int       `json:"rate_limited_count,omitempty"` // Consecutive 429 count
	RetryAfterUntil  time.Time `json:"retry_after_until,omitempty"`  // Absolute time when retry is allowed

	// Last successful data (preserved during rate-limited periods)
	LastGoodData *usageCacheData `json:"last_good_data,omitempty"`
}

// CredentialsFile represents the Claude credentials file
type CredentialsFile struct {
	ClaudeAiOauth *struct {
		AccessToken      string `json:"accessToken"`
		SubscriptionType string `json:"subscriptionType"`
		ExpiresAt        int64  `json:"expiresAt"`
	} `json:"claudeAiOauth"`
}

// UsageApiResponse represents the OAuth usage API response
type UsageApiResponse struct {
	FiveHour *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
}

// UsageData holds parsed usage information
type UsageData struct {
	FiveHour        float64
	SevenDay        float64
	FiveHourResetAt time.Time
	SevenDayResetAt time.Time
	APIUnavailable  bool
	APIError        string
}

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

// QuotaCollector collects subscription quota usage
type QuotaCollector struct {
	*BaseCollector
}

// NewQuotaCollector creates a new quota collector
func NewQuotaCollector() *QuotaCollector {
	return &QuotaCollector{
		BaseCollector: NewBaseCollectorWithTimeout(ContentQuota, 5*time.Minute, 4*time.Second, true),
	}
}

// Collect returns subscription quota usage
func (c *QuotaCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	return getSubscriptionQuota(statusInput), nil
}

// getSubscriptionQuota returns the subscription quota usage percentage with reset time
func getSubscriptionQuota(input *StatusLineInput) string {
	var usage *UsageData
	if getSubscriptionUsageFn != nil {
		usage = getSubscriptionUsageFn()
	} else {
		usage = getSubscriptionUsage()
	}

	if usage == nil {
		return ""
	}

	hasFive := usage.FiveHour > 0
	hasSeven := usage.SevenDay > 0

	if !hasFive && !hasSeven {
		return ""
	}

	// Both limits available: show 5h (primary) + 7d (secondary), reset refers to 5h
	if hasFive && hasSeven {
		resetTime := formatResetTime(usage.FiveHourResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% 5h · %.0f%% 7d · Reset %s", usage.FiveHour, usage.SevenDay, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%% 5h · %.0f%% 7d", usage.FiveHour, usage.SevenDay)
	}

	// Only 5-hour limit
	if hasFive {
		resetTime := formatResetTime(usage.FiveHourResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% · Reset %s", usage.FiveHour, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%%", usage.FiveHour)
	}

	// Only 7-day limit
	resetTime := formatResetTime(usage.SevenDayResetAt)
	if resetTime != "" {
		return fmt.Sprintf("📊 %.0f%% 7d · Reset %s", usage.SevenDay, resetTime)
	}
	return fmt.Sprintf("📊 %.0f%% 7d", usage.SevenDay)
}

// formatResetTime formats the reset time in local timezone with timezone name
func formatResetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	local := t.Local()
	timeStr := local.Format("15:04")
	zoneName := getLocalTimeZoneName()
	return fmt.Sprintf("%s (%s)", timeStr, zoneName)
}

// getCachePath returns the cache file path
func getCachePath(homeDir string) string {
	return filepath.Join(homeDir, ".claude", usageCacheFile)
}

// readUsageCache reads the cache file (no lock, direct read)
func readUsageCache() *usageCacheData {
	homeDir, err := getHomeDirFn()
	if err != nil {
		return nil
	}
	cachePath := getCachePath(homeDir)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil // File not exists (first run)
	}
	var cache usageCacheData
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil // File corrupted
	}
	return &cache
}

// writeUsageCache writes cache atomically (temp file + rename)
func writeUsageCache(cache *usageCacheData) error {
	homeDir, err := getHomeDirFn()
	if err != nil {
		return err
	}

	cachePath := getCachePath(homeDir)
	cacheDir := filepath.Dir(cachePath)

	// Ensure directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return err
		}
	}

	// Use nanosecond timestamp to ensure unique temp file name
	tmpPath := cachePath + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 10)

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	// 1. Write to temp file
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	// 2. Sync to ensure data is persisted (optional, increases safety)
	if syncFileFn != nil {
		syncFileFn(tmpPath) // best effort, ignore error
	}

	// 3. Atomic replace
	// Windows: os.Rename fails if target exists, need to remove first
	if currentOS == "windows" {
		os.Remove(cachePath)
	}
	err = os.Rename(tmpPath, cachePath)
	if err != nil {
		os.Remove(tmpPath) // Clean up temp file
	}
	return err
}

// getRateLimitedTTL returns exponential backoff TTL based on consecutive 429 count
// Pattern: 60s, 120s, 240s, capped at 5 min
func getRateLimitedTTL(count int) time.Duration {
	ttl := rateLimitBaseSeconds * time.Second
	for i := 1; i < count; i++ {
		ttl *= 2
	}
	if ttl > rateLimitMaxSeconds*time.Second {
		ttl = rateLimitMaxSeconds * time.Second
	}
	return ttl
}

// shouldRefreshResult returns refresh decision with TTL handling
// Returns: (shouldRefresh, cache, isRateLimitedBackoff)
func shouldRefreshResult() (bool, *usageCacheData, bool) {
	now := time.Now()
	cache := readUsageCache()

	// Case 1: No cache file (first run)
	if cache == nil {
		return true, nil, false
	}

	// Check if we're in rate-limit backoff period
	if cache.APIError == "rate-limited" && !cache.RetryAfterUntil.IsZero() {
		if now.Before(cache.RetryAfterUntil) {
			// Still in backoff, serve last good data if available
			if cache.LastGoodData != nil {
				return false, cache.LastGoodData, true
			}
			return false, cache, true
		}
	}

	// Determine TTL based on cache state
	var ttl time.Duration
	if cache.APIUnavailable || cache.APIError != "" {
		ttl = time.Duration(failureCacheTTLSeconds) * time.Second
	} else {
		ttl = time.Duration(cacheTTLSeconds) * time.Second
	}

	// Case 2: Cache is still fresh
	if now.Sub(cache.FetchedAt) <= ttl {
		return false, cache, false
	}

	// Case 3: Cache expired, check if another process is refreshing
	if !cache.RefreshingSince.IsZero() {
		refreshingDuration := now.Sub(cache.RefreshingSince)
		if refreshingDuration < refreshTimeout {
			// Another process is refreshing (within 10s), use expired cache
			// But serve last good data if we're rate-limited
			if cache.APIError == "rate-limited" && cache.LastGoodData != nil {
				return false, cache.LastGoodData, false
			}
			return false, cache, false
		}
		// Over 10s, assume refresh process crashed, reset refresh flag and continue
	}

	// Case 4: Cache expired, no one is refreshing
	// Mark "refreshing" and return
	cache.RefreshingSince = now
	if err := writeUsageCache(cache); err != nil {
		// Write failed (maybe another process writing at same time), use expired cache
		return false, cache, false
	}

	// Re-read to check if another process also marked refresh
	// We wrote at time 'now', so if we read back a timestamp earlier than 'now',
	// it means another process wrote before us and we should use their result
	time.Sleep(refreshCoordDelay)
	latestCache := readUsageCache()
	if latestCache != nil && !latestCache.RefreshingSince.IsZero() && latestCache.RefreshingSince.Before(now) {
		// Another process marked refresh first (their timestamp is earlier than ours)
		return false, latestCache, false
	}

	return true, cache, false // We are responsible for refresh, cache as fallback
}

// writeRefreshedCache writes successful refresh result
func writeRefreshedCache(usage *UsageData, oldCache *usageCacheData) error {
	// Safely get last good data from old cache (may be nil)
	var lastGoodData *usageCacheData
	if oldCache != nil {
		lastGoodData = oldCache.LastGoodData
	}

	cache := &usageCacheData{
		FiveHour:         usage.FiveHour,
		SevenDay:         usage.SevenDay,
		FiveHourResetAt:  usage.FiveHourResetAt,
		SevenDayResetAt:  usage.SevenDayResetAt,
		FetchedAt:        time.Now(),
		RefreshingSince:  time.Time{}, // Clear refresh flag
		APIUnavailable:   false,
		APIError:         "",
		RateLimitedCount: 0, // Reset rate limit count on success
		// Preserve last good data from old cache
		LastGoodData: lastGoodData,
	}

	// If this is valid data, save it as lastGoodData
	if usage.FiveHour > 0 || usage.SevenDay > 0 {
		cache.LastGoodData = &usageCacheData{
			FiveHour:        usage.FiveHour,
			SevenDay:        usage.SevenDay,
			FiveHourResetAt: usage.FiveHourResetAt,
			SevenDayResetAt: usage.SevenDayResetAt,
		}
	}

	return writeUsageCache(cache)
}

// writeRefreshFailedCache writes failed refresh result
func writeRefreshFailedCache(oldCache *usageCacheData, isRateLimited bool, retryAfterSec int) error {
	now := time.Now()

	// Preserve old data if available
	if oldCache != nil {
		oldCache.FetchedAt = now
		oldCache.RefreshingSince = time.Time{}

		if isRateLimited {
			oldCache.APIError = "rate-limited"
			oldCache.RateLimitedCount++
			if retryAfterSec > 0 {
				oldCache.RetryAfterUntil = now.Add(time.Duration(retryAfterSec) * time.Second)
			} else {
				// Use exponential backoff
				ttl := getRateLimitedTTL(oldCache.RateLimitedCount)
				oldCache.RetryAfterUntil = now.Add(ttl)
			}
		} else {
			oldCache.APIUnavailable = true
			oldCache.APIError = "network"
			oldCache.RateLimitedCount = 0 // Reset on non-rate-limit error
		}

		return writeUsageCache(oldCache)
	}

	// No old data, record failure
	cache := &usageCacheData{
		FetchedAt:       now,
		RefreshingSince: time.Time{},
		APIUnavailable:  !isRateLimited,
		APIError: func() string {
			if isRateLimited {
				return "rate-limited"
			}
			return "network"
		}(),
	}
	if isRateLimited {
		cache.RateLimitedCount = 1
		ttl := getRateLimitedTTL(1)
		cache.RetryAfterUntil = now.Add(ttl)
	}
	return writeUsageCache(cache)
}

// fallbackOrNil returns cache data as UsageData or nil
// Returns old data even if API was unavailable (better than nothing)
func fallbackOrNil(cache *usageCacheData) *UsageData {
	if cache == nil {
		return nil
	}
	// Return old data even if APIUnavailable - it's better than nothing
	// Only return nil if we have no data at all
	if cache.FiveHour == 0 && cache.SevenDay == 0 {
		return nil
	}
	usage := &UsageData{
		FiveHour:        cache.FiveHour,
		SevenDay:        cache.SevenDay,
		FiveHourResetAt: cache.FiveHourResetAt,
		SevenDayResetAt: cache.SevenDayResetAt,
		APIUnavailable:  cache.APIUnavailable,
		APIError:        cache.APIError,
	}
	return usage
}

// getLocalTimeZoneName attempts to get the IANA timezone name
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

// getSubscriptionUsage fetches subscription usage from Claude OAuth API
// Uses file-based cache for cross-process coordination
func getSubscriptionUsage() *UsageData {
	// Skip usage API if user is using a custom provider (API mode)
	if isUsingCustomApiEndpoint() {
		return nil
	}

	shouldRefresh, cache, isBackoff := shouldRefreshResult()

	// During rate-limit backoff, serve last good data
	if isBackoff && cache != nil {
		return fallbackOrNil(cache)
	}

	// No refresh needed, return cache directly
	if !shouldRefresh {
		return fallbackOrNil(cache)
	}

	// Need refresh - read credentials and call API
	homeDir, err := getHomeDirFn()
	if err != nil {
		return fallbackOrNil(cache)
	}

	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	credData, err := os.ReadFile(credPath)
	if err != nil {
		return fallbackOrNil(cache)
	}

	var creds CredentialsFile
	if err := json.Unmarshal(credData, &creds); err != nil {
		return fallbackOrNil(cache)
	}

	if creds.ClaudeAiOauth == nil || creds.ClaudeAiOauth.AccessToken == "" {
		return fallbackOrNil(cache)
	}

	now := time.Now()
	if creds.ClaudeAiOauth.ExpiresAt > 0 && creds.ClaudeAiOauth.ExpiresAt < now.UnixMilli() {
		return fallbackOrNil(cache)
	}

	// Check if user has a valid subscription plan
	// API users don't have subscriptionType or have 'api'
	planName := getPlanName(creds.ClaudeAiOauth.SubscriptionType)
	if planName == "" {
		// Not a subscription account, don't call usage API
		return nil
	}

	usage, isRateLimited, retryAfterSec, err := fetchUsageAPI(creds.ClaudeAiOauth.AccessToken)
	if err != nil || usage == nil {
		// API failed, record failure state
		writeRefreshFailedCache(cache, isRateLimited, retryAfterSec)
		return fallbackOrNil(cache)
	}

	// Success, write cache (also resets rate-limited count)
	writeRefreshedCache(usage, cache)
	return usage
}

// fetchUsageAPI calls the Claude OAuth usage API
// Returns: usage data, isRateLimited, retryAfterSec, error
func fetchUsageAPI(accessToken string) (*UsageData, bool, int, error) {
	client := &http.Client{Timeout: time.Duration(httpTimeoutSeconds) * time.Second}

	req, err := http.NewRequest("GET", usageAPIURL, nil)
	if err != nil {
		return nil, false, 0, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-token-monitor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, 0, err
	}
	defer resp.Body.Close()

	// Handle rate limit (429)
	if resp.StatusCode == 429 {
		retryAfterSec := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
		return nil, true, retryAfterSec, fmt.Errorf("rate limited")
	}

	if resp.StatusCode != 200 {
		return nil, false, 0, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp UsageApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, false, 0, err
	}

	usage := &UsageData{}

	if apiResp.FiveHour != nil {
		usage.FiveHour = apiResp.FiveHour.Utilization
		if apiResp.FiveHour.ResetsAt != "" {
			if t, err := time.Parse(time.RFC3339, apiResp.FiveHour.ResetsAt); err == nil {
				usage.FiveHourResetAt = t
			}
			// If parse fails, FiveHourResetAt remains zero value (acceptable fallback)
		}
	}

	if apiResp.SevenDay != nil {
		usage.SevenDay = apiResp.SevenDay.Utilization
		if apiResp.SevenDay.ResetsAt != "" {
			if t, err := time.Parse(time.RFC3339, apiResp.SevenDay.ResetsAt); err == nil {
				usage.SevenDayResetAt = t
			}
			// If parse fails, SevenDayResetAt remains zero value (acceptable fallback)
		}
	}

	return usage, false, 0, nil
}

// parseRetryAfterHeader parses Retry-After header value
// Can be either seconds or HTTP date format
func parseRetryAfterHeader(value string) int {
	if value == "" {
		return 0
	}

	// Try parsing as seconds
	if sec, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && sec > 0 {
		return sec
	}

	// Try parsing as HTTP date
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		sec := int(time.Until(t).Seconds())
		if sec > 0 {
			return sec
		}
	}

	return 0
}
