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

// Cross-process cache constants
const (
	usageCacheFile    = ".usage-cache.json"
	refreshTimeout    = 10 * time.Second // Refresh timeout, prevents stale locks from crashes
	defaultUsageTTL   = 30 * time.Second // Default TTL when config is unavailable
	refreshCoordDelay = 50 * time.Millisecond
)

// usageCacheData represents the file-based cache structure
type usageCacheData struct {
	FiveHour        float64   `json:"five_hour"`
	SevenDay        float64   `json:"seven_day"`
	FiveHourResetAt time.Time `json:"five_hour_reset_at"`
	SevenDayResetAt time.Time `json:"seven_day_reset_at"`
	FetchedAt       time.Time `json:"fetched_at"`
	RefreshingSince time.Time `json:"refreshing_since,omitempty"` // Refresh start time (crash recovery)
	APIUnavailable  bool      `json:"api_unavailable,omitempty"`
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
		BaseCollector: NewBaseCollector(ContentQuota, 5*time.Minute, true),
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
	usage := getSubscriptionUsage()

	if usage == nil {
		return ""
	}

	if usage.FiveHour > 0 {
		resetTime := formatResetTime(usage.FiveHourResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% · Reset %s", usage.FiveHour, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%%", usage.FiveHour)
	}

	if usage.SevenDay > 0 {
		resetTime := formatResetTime(usage.SevenDayResetAt)
		if resetTime != "" {
			return fmt.Sprintf("📊 %.0f%% · Reset %s", usage.SevenDay, resetTime)
		}
		return fmt.Sprintf("📊 %.0f%%", usage.SevenDay)
	}

	return ""
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

// readUsageCache reads the cache file (no lock, direct read)
func readUsageCache() *usageCacheData {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	cachePath := filepath.Join(homeDir, ".claude", usageCacheFile)
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cachePath := filepath.Join(homeDir, ".claude", usageCacheFile)
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
	if f, err := os.Open(tmpPath); err == nil {
		f.Sync()
		f.Close()
	}

	// 3. Atomic replace
	// Windows: os.Rename fails if target exists, need to remove first
	if runtime.GOOS == "windows" {
		os.Remove(cachePath)
	}
	err = os.Rename(tmpPath, cachePath)
	if err != nil {
		os.Remove(tmpPath) // Clean up temp file
	}
	return err
}

// shouldRefreshResult returns refresh decision
// - refresh: true means should refresh (call API)
// - cache: non-nil means usable cache (may be expired)
func shouldRefreshResult(ttl time.Duration) (refresh bool, cache *usageCacheData) {
	now := time.Now()
	cache = readUsageCache()

	// Case 1: No cache file (first run)
	if cache == nil {
		return true, nil // Need refresh
	}

	// Case 2: Cache is valid
	if now.Sub(cache.FetchedAt) <= ttl {
		return false, cache // Use cache
	}

	// Case 3: Cache expired, check if another process is refreshing
	if !cache.RefreshingSince.IsZero() {
		refreshingDuration := now.Sub(cache.RefreshingSince)
		if refreshingDuration < refreshTimeout {
			// Another process is refreshing (within 10s), use expired cache
			return false, cache
		}
		// Over 10s, assume refresh process crashed, reset refresh flag and continue
	}

	// Case 4: Cache expired, no one is refreshing
	// Mark "refreshing" and return
	cache.RefreshingSince = now
	if err := writeUsageCache(cache); err != nil {
		// Write failed (maybe another process writing at same time), use expired cache
		return false, cache
	}

	// Re-read to check if another process also marked refresh
	// We wrote at time 'now', so if we read back a timestamp earlier than 'now',
	// it means another process wrote before us and we should use their result
	time.Sleep(refreshCoordDelay)
	latestCache := readUsageCache()
	if latestCache != nil && !latestCache.RefreshingSince.IsZero() && latestCache.RefreshingSince.Before(now) {
		// Another process marked refresh first (their timestamp is earlier than ours)
		return false, latestCache
	}

	return true, cache // We are responsible for refresh, cache as fallback
}

// writeRefreshedCache writes successful refresh result
func writeRefreshedCache(usage *UsageData) error {
	cache := &usageCacheData{
		FiveHour:        usage.FiveHour,
		SevenDay:        usage.SevenDay,
		FiveHourResetAt: usage.FiveHourResetAt,
		SevenDayResetAt: usage.SevenDayResetAt,
		FetchedAt:       time.Now(),
		RefreshingSince: time.Time{}, // Clear refresh flag
		APIUnavailable:  false,
	}
	return writeUsageCache(cache)
}

// writeRefreshFailedCache writes failed refresh result, preserving old data
func writeRefreshFailedCache(oldCache *usageCacheData) error {
	// Preserve old data if available, just update timestamps
	if oldCache != nil {
		oldCache.FetchedAt = time.Now()
		oldCache.RefreshingSince = time.Time{}
		// Don't set APIUnavailable if we have valid old data
		return writeUsageCache(oldCache)
	}
	// No old data, record failure
	cache := &usageCacheData{
		FetchedAt:       time.Now(),
		RefreshingSince: time.Time{},
		APIUnavailable:  true,
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
	return &UsageData{
		FiveHour:        cache.FiveHour,
		SevenDay:        cache.SevenDay,
		FiveHourResetAt: cache.FiveHourResetAt,
		SevenDayResetAt: cache.SevenDayResetAt,
	}
}

// getLocalTimeZoneName attempts to get the IANA timezone name
func getLocalTimeZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}

	if linkTarget, err := os.Readlink("/etc/localtime"); err == nil {
		if idx := strings.LastIndex(linkTarget, "zoneinfo/"); idx >= 0 {
			return linkTarget[idx+9:]
		}
	}

	_, zoneOffset := time.Now().Zone()
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
	// Read TTL from config (default 30s)
	ttl := defaultUsageTTL

	shouldRefresh, cache := shouldRefreshResult(ttl)

	// No refresh needed, return cache directly
	if !shouldRefresh {
		return fallbackOrNil(cache)
	}

	// Need refresh - read credentials and call API
	homeDir, err := os.UserHomeDir()
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

	subType := strings.ToLower(creds.ClaudeAiOauth.SubscriptionType)
	if subType == "" || strings.Contains(subType, "api") {
		return fallbackOrNil(cache)
	}

	usage, err := fetchUsageAPI(creds.ClaudeAiOauth.AccessToken)
	if err != nil || usage == nil {
		// API failed, record failure state (preserves old data)
		writeRefreshFailedCache(cache)
		return fallbackOrNil(cache)
	}

	// Success, write cache
	writeRefreshedCache(usage)
	return usage
}

// fetchUsageAPI calls the Claude OAuth usage API
func fetchUsageAPI(accessToken string) (*UsageData, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "claude-token-monitor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp UsageApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	usage := &UsageData{}

	if apiResp.FiveHour != nil {
		usage.FiveHour = apiResp.FiveHour.Utilization
		if apiResp.FiveHour.ResetsAt != "" {
			usage.FiveHourResetAt, _ = time.Parse(time.RFC3339, apiResp.FiveHour.ResetsAt)
		}
	}

	if apiResp.SevenDay != nil {
		usage.SevenDay = apiResp.SevenDay.Utilization
		if apiResp.SevenDay.ResetsAt != "" {
			usage.SevenDayResetAt, _ = time.Parse(time.RFC3339, apiResp.SevenDay.ResetsAt)
		}
	}

	return usage, nil
}
