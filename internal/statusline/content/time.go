package content

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/young1lin/claude-token-monitor/internal/claudedir"
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
	nowFn                  = time.Now                                          // Override in tests for deterministic countdowns
)

// claudeAPIProxy holds the proxy URL applied only to api.anthropic.com requests.
// Empty (default) → no proxy. Precedence resolution (CLI > env > YAML) happens
// in (*config.Config).ResolveClaudeAPIProxy and is passed in via SetClaudeAPIProxy.
var (
	claudeAPIProxy   string
	claudeAPIProxyMu sync.RWMutex
)

// SetClaudeAPIProxy stores the already-resolved proxy URL used for outbound
// requests to api.anthropic.com. An empty string disables the proxy.
// Thread-safe. Callers should pass the value from
// (*config.Config).ResolveClaudeAPIProxy so CLI / env / YAML precedence stays
// in one place.
func SetClaudeAPIProxy(proxyURL string) {
	claudeAPIProxyMu.Lock()
	defer claudeAPIProxyMu.Unlock()
	claudeAPIProxy = strings.TrimSpace(proxyURL)
}

// getClaudeAPIProxy returns the stored proxy URL for Claude API requests.
// Returns an empty string when no proxy is configured.
func getClaudeAPIProxy() string {
	claudeAPIProxyMu.RLock()
	defer claudeAPIProxyMu.RUnlock()
	return claudeAPIProxy
}

// newClaudeHTTPClient returns an HTTP client for Claude OAuth API requests.
// When a proxy is configured it routes through that proxy; otherwise it uses
// a direct connection. It deliberately does NOT honor HTTP_PROXY/HTTPS_PROXY
// so unrelated environment proxies cannot leak into Claude API traffic.
//
// Supported schemes:
//   - http://  / https:// — standard HTTP CONNECT proxy. Basic-auth credentials
//     embedded in the URL (user:pass@host) are sent automatically by
//     net/http via the Proxy-Authorization header.
//   - socks5:// / socks5h:// — SOCKS5 proxy via golang.org/x/net/proxy.
//     SOCKS5 username/password auth is read from the URL by proxy.FromURL.
//
// Unknown or unparseable schemes silently fall through to a direct connection
// rather than failing — a typo in the YAML must never break the statusline.
func newClaudeHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{Proxy: nil}
	if raw := getClaudeAPIProxy(); raw != "" {
		applyProxyToTransport(transport, raw)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// applyProxyToTransport mutates transport so requests route through the proxy
// described by rawURL. Returns silently on any parse / scheme error so that a
// malformed YAML value never escalates into a startup failure.
func applyProxyToTransport(transport *http.Transport, rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		// http.ProxyURL takes care of Basic-auth credentials in user info.
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5", "socks5h":
		// proxy.FromURL reads SOCKS5 user/password from the URL's user info.
		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return
		}
		// Only ContextDialer integrates cleanly with http.Transport — every
		// dialer in x/net/proxy implements it, but guard anyway for forward
		// compatibility.
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = cd.DialContext
		}
	}
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

// getEffectiveHomeDir returns the home directory, allowing test override.
func getEffectiveHomeDir() (string, error) {
	if overrideHomeDir != "" {
		return overrideHomeDir, nil
	}
	return os.UserHomeDir()
}

// getClaudeConfigDir is a package-local wrapper around claudedir.Resolve that
// plugs in the time.go-specific home-dir injection point (getHomeDirFn). All
// statusline data sources that read per-account state should go through the
// shared claudedir package — see internal/claudedir/claudedir.go for the
// canonical resolution order.
func getClaudeConfigDir() (string, error) {
	return claudedir.Resolve(getHomeDirFn)
}

// Cross-process cache constants (aligned with claude-hud).
// The success-path TTL is variable-not-const because YAML/CLI config can
// override it at startup — see usageCacheTTL below.
const (
	usageCacheFile           = ".usage-cache.json"
	refreshTimeout           = 10 * time.Second // Refresh timeout, prevents stale locks from crashes
	refreshCoordDelay        = 50 * time.Millisecond
	defaultUsageCacheTTLSecs = 90  // Default success cache TTL when nothing is configured
	failureCacheTTLSeconds   = 15  // Failure cache TTL
	rateLimitBaseSeconds     = 60  // 429 base backoff
	rateLimitMaxSeconds      = 300 // 429 max backoff (5 min)
	httpTimeoutSeconds       = 15  // HTTP client timeout for API calls
)

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

// getPlanName determines the plan name from subscriptionType
// Returns empty string for API users (no subscription).
//
// Recognises both Anthropic ("claude-max", "claude-pro", "claude-team") and
// GLM Coding Plan ("max", "pro", "lite") tiers, normalising to title-cased
// names so the renderer can show "[Max]" / "[Pro]" / "[Lite]" / "[Team]"
// without provider-specific branching.
func getPlanName(subscriptionType string) string {
	lower := strings.ToLower(strings.TrimSpace(subscriptionType))
	if strings.Contains(lower, "max") {
		return "Max"
	}
	if strings.Contains(lower, "pro") {
		return "Pro"
	}
	if strings.Contains(lower, "lite") {
		return "Lite"
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

	// Provider tags the source so renderer/cache can branch on it. Values:
	// "anthropic", "glm-zhipu", "glm-zai". Empty is treated as "anthropic" so
	// pre-existing cache files load without migration.
	Provider string
	// AccountKey discriminates accounts that share the same Provider. It is
	// the fingerprint computed at the producer (Anthropic: empty;
	// GLM: sha256(token)[:12]). Used by writeUsageCache to route to the
	// right per-account file.
	AccountKey string
	// PlanLevel is the subscription tier in display form ("Max", "Pro",
	// "Lite", ...) or empty for API-key-only accounts.
	PlanLevel string
	// MCP carries the GLM Coding Plan MCP monthly call budget. Nil for
	// providers that don't expose an MCP limit.
	MCP *MCPWindow
	// ExtraWindows holds any limit windows we don't have a dedicated slot
	// for, keeping the renderer forward-compatible with new (unit, number)
	// tuples that show up on future GLM plans.
	ExtraWindows []UsageWindow
}

// UsageWindow is a generic percentage-based usage window for rendering.
type UsageWindow struct {
	Label   string    `json:"label"`
	Percent float64   `json:"percent"`
	ResetAt time.Time `json:"reset_at,omitempty"`
}

// MCPWindow describes a count-denominated budget (e.g. GLM Coding Plan's
// monthly MCP tool-call cap). Separate from UsageWindow because users want to
// see the absolute "42/4000" rather than a near-zero percentage.
type MCPWindow struct {
	Used    int64       `json:"used"`
	Limit   int64       `json:"limit"`
	Percent float64     `json:"percent"`
	ResetAt time.Time   `json:"reset_at,omitempty"`
	Details []MCPDetail `json:"details,omitempty"`
}

// MCPDetail is one row of the per-tool breakdown returned by GLM's
// quota/limit endpoint (modelCode → usage).
type MCPDetail struct {
	Tool  string `json:"tool"`
	Usage int64  `json:"usage"`
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

// formatResetCountdown renders the duration until a reset as a compact,
// timezone-free countdown. The cascade matches the convention shared by
// every mainstream Claude/Codex statusline (ohugonnot, lee-fuhr, et al.):
//
//	d <= 0          → "now"
//	d < 1m          → "<1m"
//	d < 1h          → "Xm"        (e.g. "45m")
//	d < 24h         → "XhYm"      (e.g. "4h32m")
//	d >= 24h        → "XdYh"      (e.g. "1d22h", "6d4h")
func formatResetCountdown(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	if d < time.Minute {
		return "<1m"
	}
	totalHours := int(d / time.Hour)
	totalMinutes := int(d/time.Minute) % 60
	if totalHours >= 24 {
		return fmt.Sprintf("%dd%dh", totalHours/24, totalHours%24)
	}
	if totalHours >= 1 {
		return fmt.Sprintf("%dh%dm", totalHours, totalMinutes)
	}
	return fmt.Sprintf("%dm", totalMinutes)
}

// getSubscriptionQuota renders subscription quota usage as a "·"-joined list
// of windows, optionally prefixed by a "[Plan]" label. Countdown format is
// timezone-free, so no "(UTC±N)" suffix is needed.
//
// Output shape depends on Provider:
//
//   - Anthropic (legacy): always renders both 5h and 7d, even when zero:
//     📊 [Max] 22% 5h ↻ 4h32m · 2% 7d ↻ 1d22h
//     📊 [Pro] 22% 5h ↻ 4h32m · 2% 7d           // only 5h reset known
//     📊 0% 5h · 0% 7d                          // API user (no PlanLevel)
//
//   - GLM (glm-zhipu / glm-zai): renders only the windows actually present
//     on the plan, plus an MCP segment when applicable. MCP counts are
//     compacted with k/M suffixes:
//     📊 [Max] 1% 5h ↻ 4h7m · MCP 42/4k
//     📊 [Lite] 22% 5h ↻ 4h32m · 50% 7d ↻ 3d4h · MCP 380/4k
//
// PlanLevel is rendered as a bracketed prefix (matching the existing
// "[glm-5.1]" model tag style) and is followed by a single space — NOT a
// "·" — so the label visually groups with the windows rather than becoming
// a separate "column".
func getSubscriptionQuota(input *StatusLineInput) string {
	var usage *UsageData
	if getSubscriptionUsageFn != nil {
		usage = getSubscriptionUsageFn()
	} else {
		usage = getSubscriptionUsage(input)
	}

	if usage == nil {
		return ""
	}

	now := nowFn()
	// Empty provider means "written by pre-multiprovider code" or "Anthropic
	// path didn't bother tagging itself"; either way render the legacy shape.
	isAnthropic := usage.Provider == "" || usage.Provider == "anthropic"
	parts := make([]string, 0, 5)

	// 5h: always rendered in Anthropic mode (legacy invariant), else only
	// when there is actual data or a known reset time.
	if isAnthropic || usage.FiveHour > 0 || !usage.FiveHourResetAt.IsZero() {
		parts = append(parts, formatPercentWindow(usage.FiveHour, "5h", usage.FiveHourResetAt, now))
	}

	// 7d: same rule. GLM Max accounts have no weekly window, so this is
	// skipped; GLM Lite/Pro accounts will have unit=6,number=1 and surface
	// here exactly like the Anthropic 7-day window.
	if isAnthropic || usage.SevenDay > 0 || !usage.SevenDayResetAt.IsZero() {
		parts = append(parts, formatPercentWindow(usage.SevenDay, "7d", usage.SevenDayResetAt, now))
	}

	if usage.MCP != nil {
		parts = append(parts, formatMCPWindow(usage.MCP, now))
	}

	for _, w := range usage.ExtraWindows {
		parts = append(parts, formatPercentWindow(w.Percent, w.Label, w.ResetAt, now))
	}

	if len(parts) == 0 {
		return ""
	}

	body := strings.Join(parts, " · ")
	if label := formatPlanLabel(usage.PlanLevel); label != "" {
		return "📊 " + label + " " + body
	}
	return "📊 " + body
}

// formatPlanLabel turns a raw PlanLevel into the bracketed display form
// ("Max" → "[Max]") used as the leading tag on the quota line. Empty input
// produces an empty string so API-key-only accounts render without a label.
// The plan name is normalized through the same heuristic used for Anthropic
// subscription types, so GLM's "max"/"pro"/"lite" come out title-cased and
// stay visually consistent across providers.
func formatPlanLabel(plan string) string {
	if plan == "" {
		return ""
	}
	name := getPlanName(plan)
	if name == "" {
		return ""
	}
	return "[" + name + "]"
}

// quotaPercentColor returns the ANSI prefix used to colour a quota
// percentage. Thresholds are intentionally inverted relative to the context
// progress bar: for quota, "high percentage" means "less budget remaining",
// so red kicks in at 80% and below that we descend through yellow / cyan /
// green into bright green as the user has more headroom. The bar in
// model.go uses the opposite mapping because there 60% is already near the
// AutoCompact line — do NOT unify the two scales.
//
// Returns the empty string for negative inputs (shouldn't occur in practice,
// but keeps the formatter total).
func quotaPercentColor(pct float64) string {
	switch {
	case pct >= 80:
		return "\x1b[1;31m" // red: out-of-budget warning
	case pct >= 60:
		return "\x1b[1;33m" // yellow: heads-up
	case pct >= 40:
		return "\x1b[1;36m" // cyan: past halfway
	case pct >= 20:
		return "\x1b[1;32m" // green: normal usage
	case pct >= 0:
		return "\x1b[1;92m" // bright green: plenty of headroom
	}
	return ""
}

// colouredPercent renders "%.0f%%" with the quota colour applied. The reset
// code follows so the surrounding text stays uncoloured.
func colouredPercent(pct float64) string {
	if c := quotaPercentColor(pct); c != "" {
		return fmt.Sprintf("%s%.0f%%\x1b[0m", c, pct)
	}
	return fmt.Sprintf("%.0f%%", pct)
}

// formatPercentWindow renders a single "X% label" or "X% label ↻ countdown"
// segment used by both Anthropic and GLM percentage-based windows. The
// percentage is wrapped in a 5-tier ANSI colour (see quotaPercentColor).
func formatPercentWindow(percent float64, label string, resetAt, now time.Time) string {
	if resetAt.IsZero() {
		return fmt.Sprintf("%s %s", colouredPercent(percent), label)
	}
	return fmt.Sprintf("%s %s ↻ %s", colouredPercent(percent), label, formatResetCountdown(resetAt.Sub(now)))
}

// formatMCPWindow renders the GLM Coding Plan MCP segment. MCP is
// count-denominated so "42/4k" is preferred over "1%"; we fall back to
// percentage (with colour) when the absolute limit is unknown. The reset
// countdown is suppressed because the budget rolls monthly — 21 days out is
// not useful statusline info, and showing it just eats horizontal space.
func formatMCPWindow(m *MCPWindow, _ time.Time) string {
	if m.Limit > 0 {
		return fmt.Sprintf("MCP %s/%s", compactCount(m.Used), compactCount(m.Limit))
	}
	return fmt.Sprintf("MCP %s", colouredPercent(m.Percent))
}

// compactCount formats a non-negative integer with a k/M suffix for values
// at or above 1000. Whole thousands drop the decimal ("4000"→"4k") while
// fractional values get one digit ("1234"→"1.2k"). Negative inputs are
// returned unchanged via the default integer path — they shouldn't occur in
// practice but we don't want a surprise crash.
func compactCount(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs < 1000:
		return strconv.FormatInt(n, 10)
	case abs < 1_000_000:
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		if n%1_000_000 == 0 {
			return fmt.Sprintf("%dM", n/1_000_000)
		}
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
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
// Returns nil when:
//   - the file doesn't exist (first run);
//   - the file is corrupt;
//   - the embedded Provider tag doesn't match what the caller asked for
//     (defensive guard against a pre-multiprovider shared .usage-cache.json
//     that happens to carry a GLM Provider tag);
//   - accountKey is supplied (non-empty) but the embedded AccountKey doesn't
//     match it (defensive guard against manual file copy / dev typo — under
//     normal operation the filename alone would have already isolated us).
func readUsageCache(provider, accountKey string) *usageCacheData {
	claudeDir, err := getClaudeConfigDir()
	if err != nil {
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

// writeUsageCache writes cache atomically (temp file + rename). The
// destination file is derived from cache.Provider so each provider's data
// lands in its own file; callers must therefore make sure Provider is set
// before invoking this.
func writeUsageCache(cache *usageCacheData) error {
	claudeDir, err := getClaudeConfigDir()
	if err != nil {
		return err
	}

	cachePath := getCachePath(claudeDir, cache.Provider, cache.AccountKey)
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

// shouldRefreshResult returns refresh decision with TTL handling for the
// given (provider, accountKey) cache file. Callers must pass their own
// identity so the coordination state (RefreshingSince, rate-limit backoff)
// stays scoped to one backend AND one account — an Anthropic 429 must not
// throttle GLM, and a GLM-Pro 429 must not throttle GLM-Lite.
// Returns: (shouldRefresh, cache, isRateLimitedBackoff).
func shouldRefreshResult(provider, accountKey string) (bool, *usageCacheData, bool) {
	now := time.Now()
	cache := readUsageCache(provider, accountKey)

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
	if err := writeUsageCache(cache); err != nil {
		// Write failed (maybe another process writing at same time), use expired cache
		return false, cache, false
	}

	// Re-read to check if another process also marked refresh
	// We wrote at time 'now', so if we read back a timestamp earlier than 'now',
	// it means another process wrote before us and we should use their result
	time.Sleep(refreshCoordDelay)
	latestCache := readUsageCache(provider, accountKey)
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

	return writeUsageCache(cache)
}

// writeRefreshFailedCache writes failed refresh result.
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
func writeRefreshFailedCache(oldCache *usageCacheData, isRateLimited bool, retryAfterSec int, provider, accountKey string) error {
	now := time.Now()

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
	return writeUsageCache(cache)
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

// getSubscriptionUsage dispatches to the right provider's usage fetcher based
// on $ANTHROPIC_BASE_URL. Returns nil for "custom" third-party proxies — we
// have no way to query their quota.
func getSubscriptionUsage(input *StatusLineInput) *UsageData {
	switch p := detectProvider(); {
	case p.isGLM():
		return getGLMUsage(input, p)
	case p == providerCustom:
		// Unknown third-party endpoint (router, proxy, mock). Hide the line
		// rather than show stale Anthropic data or fail noisily.
		return nil
	default:
		return getAnthropicUsage()
	}
}

// getAnthropicUsage fetches subscription usage from the Claude OAuth API.
// Uses the shared file-based cache for cross-process coordination. The
// Anthropic flow passes an empty accountKey because $CLAUDE_CONFIG_DIR
// already discriminates accounts (each account has its own
// ~/.claude-account-XX with its own credentials).
func getAnthropicUsage() *UsageData {
	const provider, accountKey = "anthropic", ""
	shouldRefresh, cache, isBackoff := shouldRefreshResult(provider, accountKey)

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

	// Need refresh - read credentials and call API. Resolve from the active
	// Claude config dir so multi-account setups ($CLAUDE_CONFIG_DIR pointing at
	// e.g. ~/.claude-account-ME) report the right account.
	claudeDir, err := getClaudeConfigDir()
	if err != nil {
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
		// API failed, record failure state tagged for the Anthropic path so
		// a subsequent GLM call sees a mismatch and refreshes.
		writeRefreshFailedCache(cache, isRateLimited, retryAfterSec, provider, accountKey)
		return fallbackOrNil(cache)
	}

	// Tag the source so cache invalidation and the renderer can distinguish
	// Anthropic-OAuth output from future providers (GLM, ...). AccountKey
	// stays empty for the Anthropic flow on purpose — see getAnthropicUsage.
	usage.Provider = provider
	usage.AccountKey = accountKey
	usage.PlanLevel = planName

	// Success, write cache (also resets rate-limited count)
	writeRefreshedCache(usage, cache)
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
