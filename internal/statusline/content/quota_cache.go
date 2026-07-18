package content

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// errNoClaudeDir is returned by cache helpers when the caller passed an empty
// Claude config dir. Production code receives env.ClaudeDir from BuildEnv
// (which resolves it once via claudedir.Resolve); tests pass it explicitly.
// Callers treat any error as "skip cache I/O", matching the pre-refactor
// behavior when claudedir.Resolve failed.
var errNoClaudeDir = errors.New("content: empty Claude config dir")

// Test injection points for the quota cache / filesystem layer.
var (
	syncFileFn = syncFile // Override file sync in tests (nil = no-op)
)

// Cross-process cache constants (aligned with claude-hud).
// The success-path TTL is variable-not-const because YAML/CLI config can
// override it at startup — see usageCacheTTL below.
const (
	usageCacheFile           = ".usage-cache.json"
	refreshTimeout           = 10 * time.Second // Refresh timeout, prevents stale locks from crashes
	defaultUsageCacheTTLSecs = 90               // Default success cache TTL when nothing is configured
	failureCacheTTLSeconds   = 15               // Failure cache TTL
	rateLimitBaseSeconds     = 60               // 429 base backoff
	rateLimitMaxSeconds      = 300              // 429 max backoff (5 min)
)

// refreshCoordDelay is the post-mark settle window used by shouldRefreshResult
// to let a concurrent process win the refresh race. A var (not const) so the
// package-level TestMain can zero it for the rest of the suite — six tests
// otherwise stall 50ms each waiting for cross-process coordination that never
// happens in a single-process unit test. The one test that does exercise the
// real coordination semantics restores 50ms locally via t.Cleanup.
var refreshCoordDelay = 50 * time.Millisecond

// usageCacheTTL is the effective success-path TTL for the usage/quota cache.
// Defaults to defaultUsageCacheTTLSecs and is replaced via SetUsageCacheTTL
// from main once the YAML config is loaded.
// Failure / 429 backoff timings are intentionally NOT configurable — they
// protect us from runaway requests when the upstream is unhealthy.
var (
	usageCacheTTL   = time.Duration(defaultUsageCacheTTLSecs) * time.Second
	usageCacheTTLMu sync.RWMutex
)

// SetUsageCacheTTL configures the success-path cache TTL for the usage/quota
// API. Non-positive values are ignored — they fall back to the built-in
// default (90s). Thread-safe.
func SetUsageCacheTTL(d time.Duration) {
	if d <= 0 {
		return
	}
	usageCacheTTLMu.Lock()
	defer usageCacheTTLMu.Unlock()
	usageCacheTTL = d
}

// getUsageCacheTTL returns the currently configured success cache TTL.
func getUsageCacheTTL() time.Duration {
	usageCacheTTLMu.RLock()
	defer usageCacheTTLMu.RUnlock()
	return usageCacheTTL
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

	// Multi-provider fields. Empty Provider means the cache was written by an
	// older binary; treat it as "anthropic" so we don't force a needless
	// refresh on first run after upgrade.
	Provider string `json:"provider,omitempty"`
	// AccountKey is a short, non-reversible fingerprint of the credential that
	// produced this cache entry. Empty for the Anthropic flow (where
	// $CLAUDE_CONFIG_DIR is the account discriminator); for GLM, it's
	// sha256(ANTHROPIC_AUTH_TOKEN)[:12]. This lets one provider host multiple
	// accounts (e.g. GLM Pro + GLM Lite on the same machine) without their
	// caches clobbering each other.
	AccountKey   string        `json:"account_key,omitempty"`
	PlanLevel    string        `json:"plan_level,omitempty"`
	MCP          *MCPWindow    `json:"mcp,omitempty"`
	ExtraWindows []UsageWindow `json:"extra_windows,omitempty"`
}

// syncFile opens a file, calls Sync(), and closes it. Returns sync error if any.
func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// getCachePath returns the cache file path for a (provider, accountKey)
// pair inside the resolved Claude config dir.
//
//   - Anthropic keeps the historical ".usage-cache.json" filename so existing
//     caches survive an in-place binary upgrade with zero migration work.
//     accountKey is intentionally ignored here — for the Anthropic flow,
//     $CLAUDE_CONFIG_DIR already discriminates accounts (each account has its
//     own ~/.claude-account-XX with its own .credentials.json).
//   - All other providers get ".usage-cache.<provider>.<accountKey>.json"
//     when an accountKey is supplied, or ".usage-cache.<provider>.json" when
//     it isn't (defensive fallback for callers that don't know the key yet —
//     should not happen in normal flow).
//
// This per-account layout matters for GLM: one user can hold a Pro account
// and a Lite account on the same provider, and the only thing distinguishing
// them at the env-var level is $ANTHROPIC_AUTH_TOKEN. Without per-account
// files, switching $ANTHROPIC_AUTH_TOKEN would let one account's stale cache
// shadow the other.
func getCachePath(claudeDir, provider, accountKey string) string {
	if provider == "" || provider == "anthropic" {
		return filepath.Join(claudeDir, usageCacheFile)
	}
	if accountKey == "" {
		return filepath.Join(claudeDir, ".usage-cache."+provider+".json")
	}
	return filepath.Join(claudeDir, ".usage-cache."+provider+"."+accountKey+".json")
}

// readUsageCache reads the on-disk cache for a (provider, accountKey) pair.
// claudeDir is the active Claude config dir resolved once by BuildEnv (honoring
// $CLAUDE_CONFIG_DIR); threaded through the cache chain so this layer never
// calls claudedir.Resolve itself.
//
// Returns nil when:
//   - the file doesn't exist (first run);
//   - the file is corrupt;
//   - the embedded Provider tag doesn't match what the caller asked for
//     (defensive guard against a pre-multiprovider shared .usage-cache.json
//     that happens to carry a GLM Provider tag);
//   - accountKey is supplied (non-empty) but the embedded AccountKey doesn't
//     match it (defensive guard against manual file copy / dev typo — under
//     normal operation the filename alone would have already isolated us).
func readUsageCache(claudeDir, provider, accountKey string) *usageCacheData {
	if claudeDir == "" {
		return nil
	}
	cachePath := getCachePath(claudeDir, provider, accountKey)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil // File not exists (first run)
	}
	var cache usageCacheData
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil // File corrupted
	}
	if !providerCacheMatches(&cache, providerTagOrAnthropic(provider)) {
		return nil
	}
	if accountKey != "" && cache.AccountKey != accountKey {
		return nil
	}
	return &cache
}

// providerTagOrAnthropic normalises empty string to "anthropic" so callers
// that don't bother distinguishing both can pass "" without surprises.
func providerTagOrAnthropic(provider string) string {
	if provider == "" {
		return "anthropic"
	}
	return provider
}

// writeUsageCache writes cache atomically (temp file + rename). claudeDir is
// the active Claude config dir resolved once by BuildEnv (honoring
// $CLAUDE_CONFIG_DIR); threaded through the cache chain so this layer never
// calls claudedir.Resolve itself. The destination file is derived from
// cache.Provider so each provider's data lands in its own file; callers must
// therefore make sure Provider is set before invoking this.
func writeUsageCache(claudeDir string, cache *usageCacheData) error {
	if claudeDir == "" {
		return errNoClaudeDir
	}

	cachePath := getCachePath(claudeDir, cache.Provider, cache.AccountKey)
	cacheDir := filepath.Dir(cachePath)

	// Ensure directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return err
		}
	}

	// Use nanosecond timestamp to ensure unique temp file name. nowFn (not
	// time.Now) keeps this consistent with the rest of the quota chain — env.Now
	// is itself nowFn() captured at BuildEnv. This is pure cache mechanics (tmp
	// file uniqueness), not a display cell, so threading env.Now through 5+ layers
	// of cache helpers adds churn without behavioral payoff. See Task 3's goosFn
	// comment below for the same trade-off.
	tmpPath := cachePath + ".tmp." + strconv.FormatInt(nowFn().UnixNano(), 10)

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
	// Windows: os.Rename fails if target exists, need to remove first.
	// goosFn() is the single injectable OS seam that env.OS.IsWindows itself
	// reads from (env.go:56); we consult it here rather than threading
	// isWindows through 6+ layers of cache utilities (Collect →
	// getSubscriptionQuota → getSubscriptionUsage → provider fetcher →
	// shouldRefreshResult/writeRefreshed* → writeUsageCache), because
	// runtime.GOOS is a process-constant and the cache path has no collector
	// semantics that would benefit from per-invocation context.
	if goosFn() == "windows" {
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

// shouldRefreshResult returns refresh decision with TTL handling for the
// given (provider, accountKey) cache file. claudeDir is the active Claude
// config dir resolved once by BuildEnv (honoring $CLAUDE_CONFIG_DIR);
// threaded through the cache chain so this layer never calls
// claudedir.Resolve itself. Callers must pass their own identity so the
// coordination state (RefreshingSince, rate-limit backoff) stays scoped to
// one backend AND one account — an Anthropic 429 must not throttle GLM, and
// a GLM-Pro 429 must not throttle GLM-Lite.
//
// nowFn (not time.Now) is used for the TTL clock so tests can pin the cache
// expiry; env.Now is itself nowFn() captured at BuildEnv, so production sees
// real wall-clock. Threading env.Now here would require changing every cache
// helper signature (writeUsageCache / writeRefreshedCache / writeRefreshFailedCache)
// AND their callers in getAnthropicUsageFromAPI / getGLMUsage — the same
// cost/benefit trade-off Task 3 resolved by reading goosFn() inline.
// Returns: (shouldRefresh, cache, isRateLimitedBackoff).
func shouldRefreshResult(claudeDir, provider, accountKey string) (bool, *usageCacheData, bool) {
	now := nowFn()
	cache := readUsageCache(claudeDir, provider, accountKey)

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

	// Determine TTL based on cache state.
	// Failure TTL stays short (and fixed) so transient errors recover quickly;
	// success TTL honors the configured value (default 90s).
	var ttl time.Duration
	if cache.APIUnavailable || cache.APIError != "" {
		ttl = time.Duration(failureCacheTTLSeconds) * time.Second
	} else {
		ttl = getUsageCacheTTL()
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
	if err := writeUsageCache(claudeDir, cache); err != nil {
		// Write failed (maybe another process writing at same time), use expired cache
		return false, cache, false
	}

	// Re-read to check if another process also marked refresh
	// We wrote at time 'now', so if we read back a timestamp earlier than 'now',
	// it means another process wrote before us and we should use their result
	time.Sleep(refreshCoordDelay)
	latestCache := readUsageCache(claudeDir, provider, accountKey)
	if latestCache != nil && !latestCache.RefreshingSince.IsZero() && latestCache.RefreshingSince.Before(now) {
		// Another process marked refresh first (their timestamp is earlier than ours)
		return false, latestCache, false
	}

	return true, cache, false // We are responsible for refresh, cache as fallback
}

// writeRefreshedCache writes successful refresh result. claudeDir is the
// active Claude config dir threaded down from the collector. FetchedAt uses
// nowFn for the same reason shouldRefreshResult does (see its doc comment) —
// cache mechanics, not display, and threading env.Now would ripple through
// every cache helper signature.
func writeRefreshedCache(claudeDir string, usage *UsageData, oldCache *usageCacheData) error {
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
		FetchedAt:        nowFn(),
		RefreshingSince:  time.Time{}, // Clear refresh flag
		APIUnavailable:   false,
		APIError:         "",
		RateLimitedCount: 0, // Reset rate limit count on success
		// Preserve last good data from old cache
		LastGoodData: lastGoodData,
		// Multi-provider + multi-account fields
		Provider:     usage.Provider,
		AccountKey:   usage.AccountKey,
		PlanLevel:    usage.PlanLevel,
		MCP:          usage.MCP,
		ExtraWindows: usage.ExtraWindows,
	}

	// If this is valid data, save it as lastGoodData. GLM MCP-only accounts
	// can legitimately have FiveHour == 0 && SevenDay == 0 with an MCP budget,
	// so accept any of the three signals.
	if usage.FiveHour > 0 || usage.SevenDay > 0 || usage.MCP != nil {
		cache.LastGoodData = &usageCacheData{
			FiveHour:        usage.FiveHour,
			SevenDay:        usage.SevenDay,
			FiveHourResetAt: usage.FiveHourResetAt,
			SevenDayResetAt: usage.SevenDayResetAt,
			Provider:        usage.Provider,
			AccountKey:      usage.AccountKey,
			PlanLevel:       usage.PlanLevel,
			MCP:             usage.MCP,
			ExtraWindows:    usage.ExtraWindows,
		}
	}

	return writeUsageCache(claudeDir, cache)
}

// writeRefreshFailedCache writes failed refresh result. claudeDir is the
// active Claude config dir threaded down from the collector.
//
// provider + accountKey together tag the cache entry with whichever backend
// AND account was attempted (e.g. "glm-zhipu" + "a1b2c3d4e5f6"). The pair is
// needed because:
//
//   - On the read path, providerCacheMatches uses Provider to invalidate
//     cross-provider failures, and the per-account filename keeps GLM-Pro and
//     GLM-Lite from sharing the same backoff slot.
//   - The Anthropic call site doesn't have a useful accountKey (the dir IS
//     the discriminator) and passes "" — see getCachePath for why that maps
//     to the legacy ".usage-cache.json" path.
//
// Pass provider == "" to leave the existing Provider tag on oldCache in
// place — historically useful for symmetry with old call sites; in practice
// both current callers pass an explicit value.
//
// nowFn (not time.Now) is used for the TTL clock — same trade-off as
// shouldRefreshResult above: cache mechanics, not display, and env.Now is
// already nowFn() captured at BuildEnv.
func writeRefreshFailedCache(claudeDir string, oldCache *usageCacheData, isRateLimited bool, retryAfterSec int, provider, accountKey string) error {
	now := nowFn()

	// Preserve old data if available
	if oldCache != nil {
		oldCache.FetchedAt = now
		oldCache.RefreshingSince = time.Time{}
		if provider != "" {
			oldCache.Provider = provider
		}
		// AccountKey is informational and used to derive the filename — keep
		// it in sync with whatever the caller asked for. Unlike Provider we
		// don't condition on non-empty: an Anthropic failure legitimately
		// wants an empty AccountKey so the cache lands in .usage-cache.json.
		oldCache.AccountKey = accountKey

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

		return writeUsageCache(claudeDir, oldCache)
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
		Provider:   provider,
		AccountKey: accountKey,
	}
	if isRateLimited {
		cache.RateLimitedCount = 1
		if retryAfterSec > 0 {
			cache.RetryAfterUntil = now.Add(time.Duration(retryAfterSec) * time.Second)
		} else {
			ttl := getRateLimitedTTL(1)
			cache.RetryAfterUntil = now.Add(ttl)
		}
	}
	return writeUsageCache(claudeDir, cache)
}

// fallbackOrNil returns cache data as UsageData or nil
// Returns old data even if API was unavailable (better than nothing)
func fallbackOrNil(cache *usageCacheData) *UsageData {
	if cache == nil {
		return nil
	}
	// A cache with no usage signal AND an APIError came from
	// writeRefreshFailedCache on an initial failure with no prior data —
	// genuinely nothing to show. A 0/0 cache with APIError == "" came from a
	// successful refresh that returned 0% (e.g. just past a reset) and MUST
	// be shown. GLM accounts with MCP-only signal also count as "have data".
	if cache.FiveHour == 0 && cache.SevenDay == 0 && cache.MCP == nil && cache.APIError != "" {
		return nil
	}
	usage := &UsageData{
		FiveHour:        cache.FiveHour,
		SevenDay:        cache.SevenDay,
		FiveHourResetAt: cache.FiveHourResetAt,
		SevenDayResetAt: cache.SevenDayResetAt,
		APIUnavailable:  cache.APIUnavailable,
		APIError:        cache.APIError,
		Provider:        cache.Provider,
		AccountKey:      cache.AccountKey,
		PlanLevel:       cache.PlanLevel,
		MCP:             cache.MCP,
		ExtraWindows:    cache.ExtraWindows,
	}
	return usage
}
