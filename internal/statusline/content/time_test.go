package content

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// setupTestCache creates a temp dir and returns cleanup function
func setupTestCache(t *testing.T) (cacheDir string, cleanup func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "usage-cache-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Override home directory for tests (platform-specific)
	var originalHome string
	if runtime.GOOS == "windows" {
		originalHome = os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tempDir)
	} else {
		originalHome = os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
	}

	// Create .claude directory
	claudeDir := filepath.Join(tempDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	return claudeDir, func() {
		if runtime.GOOS == "windows" {
			os.Setenv("USERPROFILE", originalHome)
		} else {
			os.Setenv("HOME", originalHome)
		}
		os.RemoveAll(tempDir)
	}
}

func TestReadUsageCache_FileNotExists(t *testing.T) {
	_, cleanup := setupTestCache(t)
	defer cleanup()

	cache := readUsageCache()
	if cache != nil {
		t.Errorf("Expected nil for non-existent cache, got %+v", cache)
	}
}

func TestReadUsageCache_ValidCache(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	// Write valid cache
	now := time.Now()
	testCache := &usageCacheData{
		FiveHour:        45.5,
		SevenDay:        12.3,
		FiveHourResetAt: now.Add(1 * time.Hour),
		SevenDayResetAt: now.Add(24 * time.Hour),
		FetchedAt:       now,
	}

	data, err := json.Marshal(testCache)
	if err != nil {
		t.Fatalf("Failed to marshal test cache: %v", err)
	}

	cachePath := filepath.Join(cacheDir, usageCacheFile)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatalf("Failed to write test cache: %v", err)
	}

	cache := readUsageCache()
	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}
	if cache.FiveHour != 45.5 {
		t.Errorf("FiveHour = %f, want 45.5", cache.FiveHour)
	}
	if cache.SevenDay != 12.3 {
		t.Errorf("SevenDay = %f, want 12.3", cache.SevenDay)
	}
}

func TestReadUsageCache_CorruptedCache(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	cachePath := filepath.Join(cacheDir, usageCacheFile)
	if err := os.WriteFile(cachePath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted cache: %v", err)
	}

	cache := readUsageCache()
	if cache != nil {
		t.Errorf("Expected nil for corrupted cache, got %+v", cache)
	}
}

func TestWriteUsageCache_AtomicWrite(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	testCache := &usageCacheData{
		FiveHour:        50.0,
		SevenDay:        15.0,
		FetchedAt:       time.Now(),
		RefreshingSince: time.Time{},
	}

	if err := writeUsageCache(testCache); err != nil {
		t.Fatalf("writeUsageCache failed: %v", err)
	}

	// Verify file exists and content is correct
	cachePath := filepath.Join(cacheDir, usageCacheFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	var loaded usageCacheData
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal cache: %v", err)
	}

	if loaded.FiveHour != 50.0 {
		t.Errorf("FiveHour = %f, want 50.0", loaded.FiveHour)
	}

	// Verify no temp files left
	files, _ := os.ReadDir(cacheDir)
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".tmp" {
			t.Errorf("Temp file left behind: %s", f.Name())
		}
	}
}

func TestShouldRefreshResult_NoCache(t *testing.T) {
	_, cleanup := setupTestCache(t)
	defer cleanup()

	refresh, cache := shouldRefreshResult(30 * time.Second)
	if !refresh {
		t.Error("Expected refresh=true for no cache")
	}
	if cache != nil {
		t.Errorf("Expected cache=nil, got %+v", cache)
	}
}

func TestShouldRefreshResult_ValidCache(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	// Write valid cache (within TTL)
	now := time.Now()
	testCache := &usageCacheData{
		FiveHour:  45.0,
		FetchedAt: now.Add(-10 * time.Second), // 10s ago, within 30s TTL
	}

	data, _ := json.Marshal(testCache)
	cachePath := filepath.Join(cacheDir, usageCacheFile)
	os.WriteFile(cachePath, data, 0644)

	refresh, cache := shouldRefreshResult(30 * time.Second)
	if refresh {
		t.Error("Expected refresh=false for valid cache")
	}
	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}
	if cache.FiveHour != 45.0 {
		t.Errorf("FiveHour = %f, want 45.0", cache.FiveHour)
	}
}

func TestShouldRefreshResult_ExpiredCache(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	// Write expired cache
	now := time.Now()
	testCache := &usageCacheData{
		FiveHour:  45.0,
		FetchedAt: now.Add(-60 * time.Second), // 60s ago, exceeds 30s TTL
	}

	data, _ := json.Marshal(testCache)
	cachePath := filepath.Join(cacheDir, usageCacheFile)
	os.WriteFile(cachePath, data, 0644)

	refresh, cache := shouldRefreshResult(30 * time.Second)
	if !refresh {
		t.Error("Expected refresh=true for expired cache")
	}
	if cache == nil {
		t.Fatal("Expected cache as fallback, got nil")
	}
	if cache.FiveHour != 45.0 {
		t.Errorf("FiveHour = %f, want 45.0", cache.FiveHour)
	}
}

func TestShouldRefreshResult_AnotherProcessRefreshing(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	// Write cache with active refresh marker
	now := time.Now()
	testCache := &usageCacheData{
		FiveHour:        45.0,
		FetchedAt:       now.Add(-60 * time.Second), // Expired
		RefreshingSince: now.Add(-2 * time.Second),  // Refresh started 2s ago
	}

	data, _ := json.Marshal(testCache)
	cachePath := filepath.Join(cacheDir, usageCacheFile)
	os.WriteFile(cachePath, data, 0644)

	refresh, cache := shouldRefreshResult(30 * time.Second)
	if refresh {
		t.Error("Expected refresh=false when another process is refreshing")
	}
	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}
}

func TestShouldRefreshResult_StaleRefreshMarker(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	// Write cache with stale refresh marker (> 10s)
	now := time.Now()
	testCache := &usageCacheData{
		FiveHour:        45.0,
		FetchedAt:       now.Add(-60 * time.Second), // Expired
		RefreshingSince: now.Add(-15 * time.Second), // Stale (15s > 10s timeout)
	}

	data, _ := json.Marshal(testCache)
	cachePath := filepath.Join(cacheDir, usageCacheFile)
	os.WriteFile(cachePath, data, 0644)

	refresh, cache := shouldRefreshResult(30 * time.Second)
	if !refresh {
		t.Error("Expected refresh=true for stale refresh marker (crash recovery)")
	}
	if cache == nil {
		t.Fatal("Expected cache as fallback, got nil")
	}
}

func TestShouldRefreshResult_ConcurrentCoordination(t *testing.T) {
	cacheDir, cleanup := setupTestCache(t)
	defer cleanup()

	// Simulate scenario where another process wrote first
	// We write our marker, then someone else's earlier marker appears
	now := time.Now()
	testCache := &usageCacheData{
		FiveHour:        45.0,
		FetchedAt:       now.Add(-60 * time.Second),
		RefreshingSince: now.Add(-200 * time.Millisecond), // Another process wrote 200ms before us
	}

	data, _ := json.Marshal(testCache)
	cachePath := filepath.Join(cacheDir, usageCacheFile)
	os.WriteFile(cachePath, data, 0644)

	// After our write, re-read will find the earlier timestamp
	// This simulates losing the race to another process
	refresh, cache := shouldRefreshResult(30 * time.Second)

	// Since another process's timestamp (200ms before now) is earlier than our "now",
	// we should NOT refresh and use their cache
	if refresh {
		t.Error("Expected refresh=false when another process wrote earlier timestamp")
	}
	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}
}

func TestWriteRefreshedCache(t *testing.T) {
	_, cleanup := setupTestCache(t)
	defer cleanup()

	now := time.Now()
	usage := &UsageData{
		FiveHour:        55.5,
		SevenDay:        20.0,
		FiveHourResetAt: now.Add(1 * time.Hour),
		SevenDayResetAt: now.Add(24 * time.Hour),
	}

	if err := writeRefreshedCache(usage); err != nil {
		t.Fatalf("writeRefreshedCache failed: %v", err)
	}

	// Verify cache file
	cache := readUsageCache()
	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}
	if cache.FiveHour != 55.5 {
		t.Errorf("FiveHour = %f, want 55.5", cache.FiveHour)
	}
	if !cache.RefreshingSince.IsZero() {
		t.Error("RefreshingSince should be zero after successful refresh")
	}
	if cache.APIUnavailable {
		t.Error("APIUnavailable should be false after successful refresh")
	}
}

func TestWriteRefreshFailedCache(t *testing.T) {
	_, cleanup := setupTestCache(t)
	defer cleanup()

	// Test with no old cache - should mark APIUnavailable
	if err := writeRefreshFailedCache(nil); err != nil {
		t.Fatalf("writeRefreshFailedCache failed: %v", err)
	}

	// Verify cache file
	cache := readUsageCache()
	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}
	if !cache.APIUnavailable {
		t.Error("APIUnavailable should be true after failed refresh with no old data")
	}
	if !cache.RefreshingSince.IsZero() {
		t.Error("RefreshingSince should be zero after failed refresh")
	}
}

func TestWriteRefreshFailedCache_PreservesOldData(t *testing.T) {
	_, cleanup := setupTestCache(t)
	defer cleanup()

	// First, write some valid data
	now := time.Now()
	oldCache := &usageCacheData{
		FiveHour:        45.5,
		SevenDay:        12.3,
		FiveHourResetAt: now.Add(1 * time.Hour),
		SevenDayResetAt: now.Add(24 * time.Hour),
		FetchedAt:       now.Add(-1 * time.Minute),
	}
	if err := writeUsageCache(oldCache); err != nil {
		t.Fatalf("writeUsageCache failed: %v", err)
	}

	// Simulate API failure - should preserve old data
	readCache := readUsageCache()
	if err := writeRefreshFailedCache(readCache); err != nil {
		t.Fatalf("writeRefreshFailedCache failed: %v", err)
	}

	// Verify old data is preserved
	cache := readUsageCache()
	if cache == nil {
		t.Fatal("Expected cache, got nil")
	}
	if cache.FiveHour != 45.5 {
		t.Errorf("FiveHour = %f, want 45.5 (should be preserved)", cache.FiveHour)
	}
	if cache.SevenDay != 12.3 {
		t.Errorf("SevenDay = %f, want 12.3 (should be preserved)", cache.SevenDay)
	}
	if cache.APIUnavailable {
		t.Error("APIUnavailable should be false when old data is preserved")
	}
}

func TestFallbackOrNil_NilCache(t *testing.T) {
	result := fallbackOrNil(nil)
	if result != nil {
		t.Errorf("Expected nil for nil cache, got %+v", result)
	}
}

func TestFallbackOrNil_APIUnavailableButHasData(t *testing.T) {
	// Even if APIUnavailable is true, we should return old data (better than nothing)
	cache := &usageCacheData{
		FiveHour:        45.0,
		SevenDay:        10.0,
		APIUnavailable:  true,
	}

	result := fallbackOrNil(cache)
	if result == nil {
		t.Fatal("Expected result (old data), got nil")
	}
	if result.FiveHour != 45.0 {
		t.Errorf("FiveHour = %f, want 45.0", result.FiveHour)
	}
}

func TestFallbackOrNil_APIUnavailableNoData(t *testing.T) {
	// If APIUnavailable is true and no data, return nil
	cache := &usageCacheData{
		APIUnavailable: true,
	}

	result := fallbackOrNil(cache)
	if result != nil {
		t.Errorf("Expected nil for API unavailable with no data, got %+v", result)
	}
}

func TestFallbackOrNil_ValidCache(t *testing.T) {
	now := time.Now()
	cache := &usageCacheData{
		FiveHour:        45.0,
		SevenDay:        10.0,
		FiveHourResetAt: now.Add(1 * time.Hour),
		SevenDayResetAt: now.Add(24 * time.Hour),
	}

	result := fallbackOrNil(cache)
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.FiveHour != 45.0 {
		t.Errorf("FiveHour = %f, want 45.0", result.FiveHour)
	}
	if result.SevenDay != 10.0 {
		t.Errorf("SevenDay = %f, want 10.0", result.SevenDay)
	}
}
