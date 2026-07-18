package content

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// usageAPIURL is the Anthropic OAuth-usage endpoint. Var (not const) so the
// httptest server in unit tests can redirect fetches at a fake server.
var usageAPIURL = "https://api.anthropic.com/api/oauth/usage"

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

// getAnthropicUsage fetches subscription usage. claudeDir is the active
// Claude config dir resolved once by BuildEnv (honoring $CLAUDE_CONFIG_DIR);
// threaded through so this layer never calls claudedir.Resolve itself.
//
// Fast path (CC 2.1.x+): when input.RateLimits is non-nil, the host has
// already given us the five_hour / seven_day percentages and reset times in
// the stdin payload. We skip the OAuth /api/oauth/usage HTTP call and its
// 429 backoff state machine entirely, only reading .credentials.json for
// the plan label (Max/Pro/Team). This is preferred because the upstream API
// is rate-limited and re-using CC's own values is both cheaper and fresher.
//
// Slow path (older CC, or stdin payload missing rate_limits): falls back to
// the original OAuth API flow with its shared file-based cache for
// cross-process coordination. The Anthropic flow passes an empty
// accountKey because $CLAUDE_CONFIG_DIR already discriminates accounts
// (each account has its own ~/.claude-account-XX with its own credentials).
func getAnthropicUsage(input *StatusLineInput, claudeDir string) *UsageData {
	if input != nil && input.RateLimits != nil {
		return buildAnthropicUsageFromStdin(input.RateLimits, claudeDir)
	}
	return getAnthropicUsageFromAPI(claudeDir)
}

// buildAnthropicUsageFromStdin assembles a UsageData from the rate_limits
// payload Claude Code now supplies on stdin. The plan label is still read
// from .credentials.json (CC does not pass subscriptionType on stdin), but
// failure to read it is non-fatal — we still surface the percentages, just
// without the "[Max]" / "[Pro]" / "[Team]" prefix.
func buildAnthropicUsageFromStdin(rl *StdinRateLimits, claudeDir string) *UsageData {
	usage := &UsageData{Provider: "anthropic"}
	if rl.FiveHour != nil {
		usage.FiveHour = rl.FiveHour.UsedPercentage
		if rl.FiveHour.ResetsAt > 0 {
			usage.FiveHourResetAt = time.Unix(rl.FiveHour.ResetsAt, 0)
		}
	}
	if rl.SevenDay != nil {
		usage.SevenDay = rl.SevenDay.UsedPercentage
		if rl.SevenDay.ResetsAt > 0 {
			usage.SevenDayResetAt = time.Unix(rl.SevenDay.ResetsAt, 0)
		}
	}
	if plan := readAnthropicPlanName(claudeDir); plan != "" {
		usage.PlanLevel = plan
	}
	return usage
}

// readAnthropicPlanName returns the subscription tier (Max/Pro/Team/Lite)
// derived from .credentials.json under claudeDir. Returns "" for API-only
// accounts or when the credentials file is unreadable — callers should
// treat that as "render percentages without a plan label".
func readAnthropicPlanName(claudeDir string) string {
	if claudeDir == "" {
		return ""
	}
	credData, err := os.ReadFile(filepath.Join(claudeDir, ".credentials.json"))
	if err != nil {
		return ""
	}
	var creds CredentialsFile
	if err := json.Unmarshal(credData, &creds); err != nil {
		return ""
	}
	if creds.ClaudeAiOauth == nil {
		return ""
	}
	return getPlanName(creds.ClaudeAiOauth.SubscriptionType)
}

// getAnthropicUsageFromAPI is the original OAuth-based fetcher kept as a
// fallback for hosts that don't supply rate_limits on stdin. claudeDir is
// the active Claude config dir threaded down from the collector so
// multi-account setups ($CLAUDE_CONFIG_DIR pointing at e.g.
// ~/.claude-account-ME) report the right account.
func getAnthropicUsageFromAPI(claudeDir string) *UsageData {
	const provider, accountKey = "anthropic", ""
	shouldRefresh, cache, isBackoff := shouldRefreshResult(claudeDir, provider, accountKey)

	// Symmetric provider-switch invalidation. If the cache was last written
	// by a GLM session and the user has now flipped back to Anthropic, force
	// a fresh fetch and drop the stale GLM values.
	if cache != nil && !providerCacheMatches(cache, "anthropic") {
		shouldRefresh = true
		isBackoff = false
		cache = nil
	}

	// During rate-limit backoff, serve last good data
	if isBackoff && cache != nil {
		return fallbackOrNil(cache)
	}

	// No refresh needed, return cache directly
	if !shouldRefresh {
		return fallbackOrNil(cache)
	}

	// Need refresh - read credentials and call API.
	if claudeDir == "" {
		return fallbackOrNil(cache)
	}

	credPath := filepath.Join(claudeDir, ".credentials.json")
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

	// nowFn (not time.Now) so tests can pin the clock for credential-expiry
	// assertions; env.Now itself is derived from nowFn at BuildEnv, so the two
	// stay consistent in production. Threading env.Now here would mean changing
	// getAnthropicUsage → getAnthropicUsageFromAPI plus every cache helper it
	// calls (shouldRefreshResult / writeRefreshedCache / writeRefreshFailedCache)
	// — see quota_cache.go for the same trade-off Task 3 made for goosFn.
	now := nowFn()
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
		// API failed, record failure state tagged for the Anthropic path so
		// a subsequent GLM call sees a mismatch and refreshes.
		writeRefreshFailedCache(claudeDir, cache, isRateLimited, retryAfterSec, provider, accountKey)
		return fallbackOrNil(cache)
	}

	// Tag the source so cache invalidation and the renderer can distinguish
	// Anthropic-OAuth output from future providers (GLM, ...). AccountKey
	// stays empty for the Anthropic flow on purpose — see getAnthropicUsage.
	usage.Provider = provider
	usage.AccountKey = accountKey
	usage.PlanLevel = planName

	// Success, write cache (also resets rate-limited count)
	writeRefreshedCache(claudeDir, usage, cache)
	return usage
}

// fetchUsageAPI calls the Claude OAuth usage API
// Returns: usage data, isRateLimited, retryAfterSec, error
func fetchUsageAPI(accessToken string) (*UsageData, bool, int, error) {
	client := newClaudeHTTPClient(time.Duration(httpTimeoutSeconds) * time.Second)

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
