package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// glmAccountFingerprint returns a stable, non-reversible 12-hex-char tag for
// the given auth token. The result is suitable for use in a filename and
// safe to log: SHA-256 + truncation to 48 bits is not brute-forceable back
// to the original token, while still giving enough entropy that the dozen-
// or-so accounts a single user might rotate through never collide.
//
// Empty token → empty string; callers decide what to do (getGLMUsage
// short-circuits, treating it as a config issue).
func glmAccountFingerprint(token string) string {
	t := strings.TrimSpace(token)
	if t == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])[:12]
}

// providerKind tags which backend Claude Code is pointed at. Knowing this
// lets the renderer choose between the legacy Anthropic two-window layout
// and the GLM multi-window layout, and lets the cache invalidate itself on
// account switches.
type providerKind int

const (
	providerUnknown   providerKind = iota
	providerAnthropic              // https://api.anthropic.com (or unset)
	providerGLMZai                 // https://api.z.ai
	providerGLMZhipu               // https://open.bigmodel.cn / https://dev.bigmodel.cn
	providerCustom                 // any other third-party proxy
)

// String returns the stable tag stored in the cache file. Tests rely on these
// exact values, do not change without updating the cache compatibility note.
func (p providerKind) String() string {
	switch p {
	case providerAnthropic:
		return "anthropic"
	case providerGLMZai:
		return "glm-zai"
	case providerGLMZhipu:
		return "glm-zhipu"
	case providerCustom:
		return "custom"
	}
	return "unknown"
}

// isGLM is true for any GLM-Coding-Plan-compatible backend.
func (p providerKind) isGLM() bool {
	return p == providerGLMZai || p == providerGLMZhipu
}

// glmPlanWindows reports which token-percent windows a given GLM plan is
// known to have. This is plan-structural metadata, NOT live data — so we
// can keep rendering "0% 5h" right after a window resets (when the API
// briefly returns percentage=0 with nextResetTime=0 because no token has
// been spent in the new window yet) instead of collapsing the segment to
// nothing. Without this, GLM Max users see the 5h line disappear and
// reappear every five hours, which looks like a broken display.
//
// Plan structure as of 2026-Q2:
//
//	max         → 5h only (no weekly window)
//	pro / lite  → 5h + 7d
//	others      → unknown; caller should fall back to the "render only if
//	              we have data" rule
//
// PlanLevel is lower-cased and trimmed before matching since the API has
// historically returned mixed-case values ("Max" vs "max" vs " MAX ").
func glmPlanWindows(planLevel string) (hasFiveHour, hasSevenDay bool) {
	switch strings.ToLower(strings.TrimSpace(planLevel)) {
	case "max":
		return true, false
	case "pro", "lite":
		return true, true
	}
	return false, false
}

// providerCacheMatches reports whether the cache entry was produced by the
// same provider the caller is currently using. An empty Provider in the
// cache is treated as "anthropic", which is what pre-multiprovider binaries
// wrote — that keeps the Anthropic path from forcing a needless refresh on
// the first run after upgrade, while still detecting an account switch when
// the current request is GLM.
func providerCacheMatches(cache *usageCacheData, want string) bool {
	if cache == nil {
		return true
	}
	have := cache.Provider
	if have == "" {
		have = "anthropic"
	}
	return have == want
}

// glmBaseURLOverride lets tests redirect GLM fetches at an httptest server.
// Empty (default) → resolve from provider per glmBaseURL.
var glmBaseURLOverride string

// glmHTTPTimeout caps a single GLM monitor request. Kept short so a slow
// upstream cannot stall the statusline render.
var glmHTTPTimeout = 4 * time.Second

// detectProvider classifies $ANTHROPIC_BASE_URL (with fallback to
// $ANTHROPIC_API_BASE_URL — both forms occur in user configs).
func detectProvider() providerKind {
	baseURL := strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("ANTHROPIC_API_BASE_URL"))
	}
	if baseURL == "" || strings.HasPrefix(baseURL, "https://api.anthropic.com") {
		return providerAnthropic
	}
	lower := strings.ToLower(baseURL)
	switch {
	case strings.Contains(lower, "api.z.ai"):
		return providerGLMZai
	case strings.Contains(lower, "bigmodel.cn"):
		return providerGLMZhipu
	default:
		return providerCustom
	}
}

// glmBaseURL returns the scheme+host (no trailing slash) of the quota monitor
// API. We parse $ANTHROPIC_BASE_URL directly so user configs like
// "https://open.bigmodel.cn/api/anthropic" (Anthropic-compat subpath) still
// resolve to the right host — appending /api/monitor/... to the raw value
// would otherwise double up the /api segment.
//
// glmBaseURLOverride wins over everything so httptest servers work in tests.
func glmBaseURL(p providerKind) string {
	if glmBaseURLOverride != "" {
		return glmBaseURLOverride
	}
	raw := strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("ANTHROPIC_API_BASE_URL"))
	}
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	// Hardcoded fallbacks for the unlikely case ANTHROPIC_BASE_URL is unset
	// while detection still landed on a GLM provider (shouldn't happen, but
	// don't crash).
	switch p {
	case providerGLMZai:
		return "https://api.z.ai"
	case providerGLMZhipu:
		return "https://open.bigmodel.cn"
	}
	return ""
}

// getGLMAuthToken reads $ANTHROPIC_AUTH_TOKEN from the process env. We
// intentionally do NOT fall back to reading settings.json from disk:
//
//   - Claude Code already merges settings.json's `env` block into the
//     subprocess environment before spawning statusline, so by the time we
//     run here, the env var is authoritative.
//   - Reading settings.json ourselves would defeat ad-hoc overrides like
//     PowerShell's `$env:ANTHROPIC_AUTH_TOKEN="..."` followed by `claude`,
//     where the user expects the env value to win over whatever is on disk.
//   - It also avoids guessing which of three settings.json paths
//     (settings.local.json, settings.json, $CLAUDE_CONFIG_DIR/settings.json)
//     is "the right one" — Claude Code's own precedence logic is what we
//     want, and it's already been applied.
func getGLMAuthToken() string {
	return strings.TrimSpace(os.Getenv("ANTHROPIC_AUTH_TOKEN"))
}

// glmQuotaResponse mirrors the JSON shape of
// GET /api/monitor/usage/quota/limit. Only fields we actually read are
// declared; extras are silently ignored.
type glmQuotaResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
	Data    struct {
		Level  string          `json:"level"`
		Limits []glmQuotaLimit `json:"limits"`
	} `json:"data"`
}

type glmQuotaLimit struct {
	Type          string                `json:"type"`
	Unit          int                   `json:"unit"`
	Number        int                   `json:"number"`
	Usage         int64                 `json:"usage"`
	CurrentValue  int64                 `json:"currentValue"`
	Remaining     int64                 `json:"remaining"`
	Percentage    float64               `json:"percentage"`
	NextResetTime int64                 `json:"nextResetTime"`
	UsageDetails  []glmQuotaUsageDetail `json:"usageDetails"`
}

type glmQuotaUsageDetail struct {
	ModelCode string `json:"modelCode"`
	Usage     int64  `json:"usage"`
}

type rateLimitError struct {
	retryAfterSec int
}

func (e *rateLimitError) Error() string {
	if e.retryAfterSec > 0 {
		return fmt.Sprintf("rate limited; retry after %ds", e.retryAfterSec)
	}
	return "rate limited"
}

// fetchGLMQuota issues GET /api/monitor/usage/quota/limit and decodes the
// response. Returns (nil, err) on any transport or parsing failure; the
// caller decides whether to fall back to cached data or hide the line.
//
// IMPORTANT: GLM accepts the token raw, without a "Bearer " prefix. Both
// open.bigmodel.cn and api.z.ai reject "Bearer <token>" with 401. The other
// two headers are also required — without Accept-Language and
// Content-Type the upstream rate-limits aggressively.
func fetchGLMQuota(baseURL, token string) (*glmQuotaResponse, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("glm: empty base url")
	}
	if token == "" {
		return nil, fmt.Errorf("glm: missing auth token")
	}
	req, err := http.NewRequest("GET", baseURL+"/api/monitor/usage/quota/limit", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Accept-Language", "en-US,en")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: glmHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &rateLimitError{retryAfterSec: parseRetryAfterHeader(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("glm: http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var out glmQuotaResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("glm: api success=false (code=%d msg=%q)", out.Code, out.Msg)
	}
	return &out, nil
}

// glmResponseToUsageData maps the API response into UsageData using the
// (type, unit, number) dispatch table:
//
//	TOKENS_LIMIT  unit=3  number=5  → 5-hour rolling token window  (FiveHour)
//	TOKENS_LIMIT  unit=6  number=1  → weekly token quota           (SevenDay)
//	TIME_LIMIT    *                  → monthly MCP tool-call cap   (MCP)
//	TOKENS_LIMIT  other              → ExtraWindows with "Tok(uX,nY)"
//	other type                       → ExtraWindows with lowercased type
//
// The 5h/7d mapping is what makes the GLM Pro/Lite plans render identically
// to Anthropic 5h/7d subscribers — no special-casing needed downstream.
func glmResponseToUsageData(resp *glmQuotaResponse, provider providerKind) *UsageData {
	if resp == nil {
		return nil
	}
	usage := &UsageData{
		Provider:  provider.String(),
		PlanLevel: resp.Data.Level,
	}
	for _, l := range resp.Data.Limits {
		switch {
		case l.Type == "TOKENS_LIMIT" && l.Unit == 3 && l.Number == 5:
			usage.FiveHour = l.Percentage
			usage.FiveHourResetAt = msToTime(l.NextResetTime)

		case l.Type == "TOKENS_LIMIT" && l.Unit == 6 && l.Number == 1:
			usage.SevenDay = l.Percentage
			usage.SevenDayResetAt = msToTime(l.NextResetTime)

		case l.Type == "TIME_LIMIT":
			details := make([]MCPDetail, 0, len(l.UsageDetails))
			for _, d := range l.UsageDetails {
				details = append(details, MCPDetail{Tool: d.ModelCode, Usage: d.Usage})
			}
			usage.MCP = &MCPWindow{
				Used:    l.CurrentValue,
				Limit:   l.Usage,
				Percent: l.Percentage,
				ResetAt: msToTime(l.NextResetTime),
				Details: details,
			}

		case l.Type == "TOKENS_LIMIT":
			usage.ExtraWindows = append(usage.ExtraWindows, UsageWindow{
				Label:   fmt.Sprintf("Tok(u%d,n%d)", l.Unit, l.Number),
				Percent: l.Percentage,
				ResetAt: msToTime(l.NextResetTime),
			})

		default:
			usage.ExtraWindows = append(usage.ExtraWindows, UsageWindow{
				Label:   strings.ToLower(l.Type),
				Percent: l.Percentage,
				ResetAt: msToTime(l.NextResetTime),
			})
		}
	}
	return usage
}

// msToTime converts a Unix-milliseconds timestamp to time.Time, treating
// non-positive values as zero so the renderer skips the countdown.
func msToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// getGLMUsage is the GLM counterpart to the Anthropic OAuth-usage flow. It
// shares the cross-process cache and rate-limit machinery, but keys the
// cache on (provider, accountFingerprint) so multiple GLM accounts on the
// same provider (e.g. Pro + Lite) don't clobber each other.
//
// Order of operations is important:
//  1. Read the auth token. Missing token → bail out, no cache I/O.
//  2. Compute the account fingerprint from the token.
//  3. Consult the per-account cache.
//
// We do (1) before (3) on purpose: the cache key depends on the token, so
// asking the cache layer "is the GLM cache fresh?" without first knowing
// which account we're talking about would always hit the wrong file.
//
// input is reserved for future per-session signals; currently unused because
// the token and URL both come from the process environment (see
// getGLMAuthToken for why we don't read settings.json from disk).
func getGLMUsage(_ *StatusLineInput, provider providerKind) *UsageData {
	token := getGLMAuthToken()
	if token == "" {
		// Missing token is a config issue, not an API failure — don't write
		// a failure marker that would suppress retries on the next refresh.
		return nil
	}
	providerTag := provider.String()
	accountKey := glmAccountFingerprint(token)

	shouldRefresh, cache, isBackoff := shouldRefreshResult(providerTag, accountKey)

	// Cache-provider mismatch (account switch / config change): force a fresh
	// fetch AND clear the cache reference so a fetch failure doesn't fall
	// back to the previous provider's data — which would render
	// Anthropic-shaped numbers under a GLM PlanLevel prefix and confuse the
	// user worse than showing nothing.
	//
	// In practice the per-account filename already isolates us, but a leftover
	// file from an older binary (when the layout was provider-only or shared)
	// can still trip this — keep the check.
	if cache != nil && !providerCacheMatches(cache, providerTag) {
		shouldRefresh = true
		isBackoff = false
		cache = nil
	}

	if isBackoff && cache != nil {
		return fallbackOrNil(cache)
	}
	if !shouldRefresh {
		return fallbackOrNil(cache)
	}

	resp, err := fetchGLMQuota(glmBaseURL(provider), token)
	if err != nil || resp == nil {
		isRateLimited := false
		retryAfterSec := 0
		if rateErr, ok := err.(*rateLimitError); ok {
			isRateLimited = true
			retryAfterSec = rateErr.retryAfterSec
		}
		writeRefreshFailedCache(cache, isRateLimited, retryAfterSec, providerTag, accountKey)
		return fallbackOrNil(cache)
	}
	usage := glmResponseToUsageData(resp, provider)
	if usage == nil {
		return fallbackOrNil(cache)
	}
	usage.AccountKey = accountKey
	writeRefreshedCache(usage, cache)
	return usage
}
