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

// All tests in this file use the literal token "test-token" and synthetic
// fixtures. Never put a real API key in test code — even briefly during
// development — because git history is forever and tests run in CI.

// ---------------------------------------------------------------------------
// detectProvider
// ---------------------------------------------------------------------------

func TestDetectProvider(t *testing.T) {
	cases := []struct {
		name       string
		baseURL    string
		apiBaseURL string
		want       providerKind
	}{
		{"both unset → Anthropic", "", "", providerAnthropic},
		{"explicit Anthropic", "https://api.anthropic.com", "", providerAnthropic},
		{"Anthropic with v1 suffix", "https://api.anthropic.com/v1", "", providerAnthropic},
		{"Zhipu open subpath", "https://open.bigmodel.cn/api/anthropic", "", providerGLMZhipu},
		{"Zhipu open bare", "https://open.bigmodel.cn", "", providerGLMZhipu},
		{"Zhipu dev", "https://dev.bigmodel.cn/api/anthropic", "", providerGLMZhipu},
		{"Z.ai", "https://api.z.ai", "", providerGLMZai},
		{"Z.ai with path", "https://api.z.ai/api/anthropic", "", providerGLMZai},
		{"Z.ai uppercase", "https://API.Z.AI", "", providerGLMZai},
		{"third-party proxy", "https://my-router.example.com", "", providerCustom},
		{"falls back to API_BASE_URL", "", "https://open.bigmodel.cn", providerGLMZhipu},
		{"whitespace trimmed", "  https://api.z.ai  ", "", providerGLMZai},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ANTHROPIC_BASE_URL", tc.baseURL)
			t.Setenv("ANTHROPIC_API_BASE_URL", tc.apiBaseURL)

			assert.Equal(t, tc.want, detectProvider())
		})
	}
}

func TestProviderKindString(t *testing.T) {
	// String() values are persisted in the cache file; assert the exact tags
	// so a careless rename doesn't silently invalidate everyone's cache.
	assert.Equal(t, "anthropic", providerAnthropic.String())
	assert.Equal(t, "glm-zai", providerGLMZai.String())
	assert.Equal(t, "glm-zhipu", providerGLMZhipu.String())
	assert.Equal(t, "custom", providerCustom.String())
	assert.Equal(t, "unknown", providerUnknown.String())
}

func TestProviderKindIsGLM(t *testing.T) {
	assert.True(t, providerGLMZai.isGLM())
	assert.True(t, providerGLMZhipu.isGLM())
	assert.False(t, providerAnthropic.isGLM())
	assert.False(t, providerCustom.isGLM())
}

// ---------------------------------------------------------------------------
// glmBaseURL
// ---------------------------------------------------------------------------

func TestGlmBaseURL(t *testing.T) {
	cases := []struct {
		name     string
		envURL   string
		provider providerKind
		want     string
	}{
		{
			name:     "extracts host from Zhipu subpath URL",
			envURL:   "https://open.bigmodel.cn/api/anthropic",
			provider: providerGLMZhipu,
			want:     "https://open.bigmodel.cn",
		},
		{
			name:     "extracts host from Z.ai subpath URL",
			envURL:   "https://api.z.ai/api/anthropic",
			provider: providerGLMZai,
			want:     "https://api.z.ai",
		},
		{
			name:     "dev.bigmodel.cn passthrough",
			envURL:   "https://dev.bigmodel.cn/api/anthropic",
			provider: providerGLMZhipu,
			want:     "https://dev.bigmodel.cn",
		},
		{
			name:     "falls back to default when env empty",
			envURL:   "",
			provider: providerGLMZhipu,
			want:     "https://open.bigmodel.cn",
		},
		{
			name:     "Z.ai fallback when env empty",
			envURL:   "",
			provider: providerGLMZai,
			want:     "https://api.z.ai",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ANTHROPIC_BASE_URL", tc.envURL)
			t.Setenv("ANTHROPIC_API_BASE_URL", "")
			assert.Equal(t, tc.want, glmBaseURL(tc.provider))
		})
	}
}

func TestGlmBaseURL_OverrideWinsForTests(t *testing.T) {
	// Sanity-check the test hook itself: glmBaseURLOverride must short-circuit
	// even when env var would otherwise resolve to a different host.
	t.Setenv("ANTHROPIC_BASE_URL", "https://api.z.ai")
	old := glmBaseURLOverride
	glmBaseURLOverride = "http://127.0.0.1:9999"
	t.Cleanup(func() { glmBaseURLOverride = old })

	assert.Equal(t, "http://127.0.0.1:9999", glmBaseURL(providerGLMZai))
}

// ---------------------------------------------------------------------------
// getGLMAuthToken
// ---------------------------------------------------------------------------

func TestGetGLMAuthToken(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")
		assert.Equal(t, "test-token", getGLMAuthToken())
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "  test-token  ")
		assert.Equal(t, "test-token", getGLMAuthToken())
	})

	t.Run("returns empty when env unset", func(t *testing.T) {
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
		assert.Equal(t, "", getGLMAuthToken())
	})
}

// ---------------------------------------------------------------------------
// fetchGLMQuota: verifies wire-level correctness against an httptest server
// ---------------------------------------------------------------------------

// newGLMTestServer returns a server that asserts the inbound request shape
// and replies with body. The captured headers are exposed via the returned
// map so tests can verify them post-hoc.
func newGLMTestServer(t *testing.T, status int, body string) (*httptest.Server, *map[string]string, *string) {
	t.Helper()
	captured := make(map[string]string)
	path := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured["Authorization"] = r.Header.Get("Authorization")
		captured["Accept-Language"] = r.Header.Get("Accept-Language")
		captured["Content-Type"] = r.Header.Get("Content-Type")
		captured["Method"] = r.Method
		path = r.URL.Path
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured, &path
}

const fixtureQuotaMaxPlan = `{
	"code": 200,
	"msg": "ok",
	"success": true,
	"data": {
		"level": "max",
		"limits": [
			{"type": "TIME_LIMIT", "unit": 5, "number": 1, "usage": 4000, "currentValue": 42, "remaining": 3958, "percentage": 1, "nextResetTime": 1781574177997,
			 "usageDetails": [
				{"modelCode": "search-prime", "usage": 0},
				{"modelCode": "web-reader", "usage": 42},
				{"modelCode": "zread", "usage": 0}
			 ]},
			{"type": "TOKENS_LIMIT", "unit": 3, "number": 5, "percentage": 1, "nextResetTime": 1779707699596}
		]
	}
}`

const fixtureQuotaProPlanWithWeekly = `{
	"code": 200,
	"msg": "ok",
	"success": true,
	"data": {
		"level": "pro",
		"limits": [
			{"type": "TOKENS_LIMIT", "unit": 3, "number": 5, "percentage": 22, "nextResetTime": 1779707699596},
			{"type": "TOKENS_LIMIT", "unit": 6, "number": 1, "percentage": 50, "nextResetTime": 1780000000000},
			{"type": "TIME_LIMIT", "unit": 5, "number": 1, "usage": 4000, "currentValue": 380, "percentage": 9.5, "nextResetTime": 1781574177997}
		]
	}
}`

const fixtureQuotaUnknownTokenWindow = `{
	"code": 200,
	"msg": "ok",
	"success": true,
	"data": {
		"level": "lite",
		"limits": [
			{"type": "TOKENS_LIMIT", "unit": 8, "number": 2, "percentage": 11, "nextResetTime": 1779707699596}
		]
	}
}`

const fixtureQuotaUnknownLimitType = `{
	"code": 200,
	"msg": "ok",
	"success": true,
	"data": {
		"level": "max",
		"limits": [
			{"type": "BANANA_LIMIT", "percentage": 42}
		]
	}
}`

func TestFetchGLMQuota_SendsCorrectRequestShape(t *testing.T) {
	srv, headers, path := newGLMTestServer(t, http.StatusOK, fixtureQuotaMaxPlan)

	resp, err := fetchGLMQuota(srv.URL, "test-token")
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Header contract (see comment in fetchGLMQuota for why these matter).
	assert.Equal(t, "test-token", (*headers)["Authorization"],
		"GLM rejects the request when Authorization carries a Bearer prefix")
	assert.Equal(t, "en-US,en", (*headers)["Accept-Language"])
	assert.Equal(t, "application/json", (*headers)["Content-Type"])
	assert.Equal(t, "GET", (*headers)["Method"])
	assert.Equal(t, "/api/monitor/usage/quota/limit", *path)
}

func TestFetchGLMQuota_RejectsEmptyInputs(t *testing.T) {
	_, err := fetchGLMQuota("", "test-token")
	assert.Error(t, err)
	_, err = fetchGLMQuota("https://example.com", "")
	assert.Error(t, err)
}

func TestFetchGLMQuota_NonOKReturnsError(t *testing.T) {
	srv, _, _ := newGLMTestServer(t, http.StatusUnauthorized, `{"msg":"bad token"}`)
	_, err := fetchGLMQuota(srv.URL, "test-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestFetchGLMQuota_SuccessFalseReturnsError(t *testing.T) {
	srv, _, _ := newGLMTestServer(t, http.StatusOK,
		`{"code": 500, "msg": "internal", "success": false, "data": {"level":"", "limits": []}}`)
	_, err := fetchGLMQuota(srv.URL, "test-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "success=false")
}

func TestFetchGLMQuota_MalformedJSONReturnsError(t *testing.T) {
	srv, _, _ := newGLMTestServer(t, http.StatusOK, `not json`)
	_, err := fetchGLMQuota(srv.URL, "test-token")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// glmResponseToUsageData: (type, unit, number) dispatch
// ---------------------------------------------------------------------------

// parseFixture is a tiny helper so each table case can lean on the same JSON
// fixtures without duplicating the response struct construction.
func parseFixture(t *testing.T, fixture string) *glmQuotaResponse {
	t.Helper()
	var r glmQuotaResponse
	require.NoError(t, json.Unmarshal([]byte(fixture), &r))
	return &r
}

func TestGLMResponseToUsageData_MaxPlanNoWeekly(t *testing.T) {
	resp := parseFixture(t, fixtureQuotaMaxPlan)

	got := glmResponseToUsageData(resp, providerGLMZhipu)

	require.NotNil(t, got)
	assert.Equal(t, "glm-zhipu", got.Provider)
	assert.Equal(t, "max", got.PlanLevel)

	// TOKENS_LIMIT(3,5) → FiveHour
	assert.InDelta(t, 1.0, got.FiveHour, 0.01)
	assert.False(t, got.FiveHourResetAt.IsZero(), "FiveHourResetAt should come from nextResetTime")
	assert.Equal(t, time.UnixMilli(1779707699596), got.FiveHourResetAt)

	// Max plan has no weekly window
	assert.Equal(t, 0.0, got.SevenDay)
	assert.True(t, got.SevenDayResetAt.IsZero())

	// TIME_LIMIT → MCP with details
	require.NotNil(t, got.MCP)
	assert.Equal(t, int64(42), got.MCP.Used)
	assert.Equal(t, int64(4000), got.MCP.Limit)
	assert.InDelta(t, 1.0, got.MCP.Percent, 0.01)
	assert.Equal(t, time.UnixMilli(1781574177997), got.MCP.ResetAt)
	require.Len(t, got.MCP.Details, 3)
	assert.Equal(t, MCPDetail{Tool: "web-reader", Usage: 42}, got.MCP.Details[1])

	// No unknown windows
	assert.Empty(t, got.ExtraWindows)
}

func TestGLMResponseToUsageData_ProPlanWithWeekly(t *testing.T) {
	resp := parseFixture(t, fixtureQuotaProPlanWithWeekly)

	got := glmResponseToUsageData(resp, providerGLMZhipu)

	require.NotNil(t, got)
	assert.Equal(t, "pro", got.PlanLevel)
	// TOKENS_LIMIT(3,5) → FiveHour
	assert.InDelta(t, 22.0, got.FiveHour, 0.01)
	// TOKENS_LIMIT(6,1) → SevenDay (the whole point of the dispatch table)
	assert.InDelta(t, 50.0, got.SevenDay, 0.01)
	assert.Equal(t, time.UnixMilli(1780000000000), got.SevenDayResetAt)
	// TIME_LIMIT → MCP
	require.NotNil(t, got.MCP)
	assert.Equal(t, int64(380), got.MCP.Used)
}

func TestGLMResponseToUsageData_UnknownTokenWindowGoesToExtra(t *testing.T) {
	resp := parseFixture(t, fixtureQuotaUnknownTokenWindow)

	got := glmResponseToUsageData(resp, providerGLMZhipu)

	require.NotNil(t, got)
	// Unknown (unit=8, number=2) must not be silently mapped to FiveHour/SevenDay.
	assert.Equal(t, 0.0, got.FiveHour)
	assert.Equal(t, 0.0, got.SevenDay)
	require.Len(t, got.ExtraWindows, 1)
	assert.Equal(t, "Tok(u8,n2)", got.ExtraWindows[0].Label)
	assert.InDelta(t, 11.0, got.ExtraWindows[0].Percent, 0.01)
}

func TestGLMResponseToUsageData_UnknownLimitTypeGoesToExtra(t *testing.T) {
	resp := parseFixture(t, fixtureQuotaUnknownLimitType)

	got := glmResponseToUsageData(resp, providerGLMZhipu)

	require.NotNil(t, got)
	require.Len(t, got.ExtraWindows, 1)
	assert.Equal(t, "banana_limit", got.ExtraWindows[0].Label)
	assert.InDelta(t, 42.0, got.ExtraWindows[0].Percent, 0.01)
}

func TestGLMResponseToUsageData_NilResponse(t *testing.T) {
	assert.Nil(t, glmResponseToUsageData(nil, providerGLMZhipu))
}

func TestMsToTime(t *testing.T) {
	assert.True(t, msToTime(0).IsZero())
	assert.True(t, msToTime(-1).IsZero())
	assert.Equal(t, time.UnixMilli(1779707699596), msToTime(1779707699596))
}

// ---------------------------------------------------------------------------
// formatMCPWindow: independent rendering check
// ---------------------------------------------------------------------------

func TestFormatMCPWindow(t *testing.T) {
	fixed := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)

	t.Run("compacts whole thousands with k suffix", func(t *testing.T) {
		m := &MCPWindow{Used: 42, Limit: 4000, Percent: 1.05}
		assert.Equal(t, "🧩 42/4k", formatMCPWindow(m, fixed))
	})

	t.Run("monthly reset is intentionally not rendered", func(t *testing.T) {
		// 21+ days out isn't actionable, and the cell width is precious.
		m := &MCPWindow{
			Used:    42,
			Limit:   4000,
			ResetAt: fixed.Add(21 * 24 * time.Hour),
		}
		assert.Equal(t, "🧩 42/4k", formatMCPWindow(m, fixed))
	})

	t.Run("falls back to percentage when limit unknown", func(t *testing.T) {
		// 8% falls in the lowest tier so it's wrapped in bright green.
		m := &MCPWindow{Percent: 7.5}
		assert.Equal(t, "🧩 \x1b[1;92m8%\x1b[0m", formatMCPWindow(m, fixed))
	})
}

func TestCompactCount(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1k"},
		{1234, "1.2k"},
		{4000, "4k"},
		{12345, "12.3k"},
		{999_999, "1000.0k"}, // borderline but format-correct
		{1_000_000, "1M"},
		{1_500_000, "1.5M"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, compactCount(tc.in), "compactCount(%d)", tc.in)
	}
}

// ---------------------------------------------------------------------------
// getSubscriptionQuota: end-to-end rendering for GLM via injected UsageData
// ---------------------------------------------------------------------------

func TestGetSubscriptionQuota_GLMMaxPlanLayout(t *testing.T) {
	// Pin "now" so the countdown text is deterministic.
	fixedNow := time.Date(2026, 5, 25, 7, 10, 0, 0, time.UTC)
	oldNow := nowFn
	nowFn = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFn = oldNow })

	canned := &UsageData{
		Provider:        "glm-zhipu",
		PlanLevel:       "max",
		FiveHour:        1,
		FiveHourResetAt: fixedNow.Add(4*time.Hour + 7*time.Minute),
		MCP: &MCPWindow{
			Used:    42,
			Limit:   4000,
			Percent: 1.05,
			ResetAt: fixedNow.Add(21*24*time.Hour + 20*time.Hour),
		},
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	require.NotEmpty(t, out)
	// Compact format: [Plan] prefix (title-cased), MCP uses k suffix, MCP
	// reset hidden (monthly countdown is not actionable on a live statusline).
	// 1% sits in the bright-green tier.
	assert.Equal(t, "📊 [Max] \x1b[1;92m1%\x1b[0m 5h ↻ 4h7m · 🧩 42/4k", out)
	// Max plan has no weekly window — must NOT render the 7d segment.
	assert.NotContains(t, out, "7d")
	// Plan label is title-cased ("Max"), never the raw uppercase form.
	assert.NotContains(t, out, "[MAX]")
}

func TestGetSubscriptionQuota_GLMProPlanLayoutIncludesWeekly(t *testing.T) {
	fixedNow := time.Date(2026, 5, 25, 7, 10, 0, 0, time.UTC)
	oldNow := nowFn
	nowFn = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFn = oldNow })

	canned := &UsageData{
		Provider:        "glm-zhipu",
		PlanLevel:       "pro",
		FiveHour:        22,
		FiveHourResetAt: fixedNow.Add(4*time.Hour + 32*time.Minute),
		SevenDay:        50,
		SevenDayResetAt: fixedNow.Add(3*24*time.Hour + 4*time.Hour),
		MCP: &MCPWindow{
			Used:  380,
			Limit: 4000,
		},
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	assert.Contains(t, out, "[Pro]", "plan label is title-cased and rendered as prefix")
	assert.NotContains(t, out, "[PRO]", "raw uppercase plan name is never rendered")
	// 22% → green tier; 50% → cyan tier.
	assert.Contains(t, out, "\x1b[1;32m22%\x1b[0m 5h ↻ 4h32m")
	assert.Contains(t, out, "\x1b[1;36m50%\x1b[0m 7d ↻ 3d4h")
	assert.Contains(t, out, "🧩 380/4k")
}

func TestGetSubscriptionQuota_AnthropicLayoutWithPlanLabel(t *testing.T) {
	// Anthropic-mode rendering: the legacy "📊 X% 5h ↻ ... · Y% 7d ↻ ..."
	// shape is preserved, with the title-cased plan label prefixed (and the
	// raw uppercase form NEVER appearing — that was the v0.2.x bug).
	fixedNow := time.Date(2026, 5, 25, 7, 10, 0, 0, time.UTC)
	oldNow := nowFn
	nowFn = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFn = oldNow })

	canned := &UsageData{
		Provider:        "anthropic",
		PlanLevel:       "Max",
		FiveHour:        22,
		FiveHourResetAt: fixedNow.Add(4*time.Hour + 32*time.Minute),
		SevenDay:        2,
		SevenDayResetAt: fixedNow.Add(1*24*time.Hour + 22*time.Hour),
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	// 22% → green tier; 2% → bright-green tier.
	assert.Equal(t, "📊 [Max] \x1b[1;32m22%\x1b[0m 5h ↻ 4h32m · \x1b[1;92m2%\x1b[0m 7d ↻ 1d22h", out)
	assert.NotContains(t, out, "[MAX]")
}

// TestGetSubscriptionQuota_APIUserNoPlanLabel covers the API-key-only
// Anthropic account: PlanLevel is empty so the [Plan] prefix must be
// completely absent (no leading space, no empty brackets).
func TestGetSubscriptionQuota_APIUserNoPlanLabel(t *testing.T) {
	fixedNow := time.Date(2026, 5, 25, 7, 10, 0, 0, time.UTC)
	oldNow := nowFn
	nowFn = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFn = oldNow })

	canned := &UsageData{
		Provider:        "anthropic",
		PlanLevel:       "", // API user
		FiveHour:        12,
		FiveHourResetAt: fixedNow.Add(2 * time.Hour),
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	// 12% → bright-green tier; 0% → bright-green tier.
	assert.Equal(t, "📊 \x1b[1;92m12%\x1b[0m 5h ↻ 2h0m · \x1b[1;92m0%\x1b[0m 7d ↻ now", out)
	assert.NotContains(t, out, "[]", "empty PlanLevel must not produce empty brackets")
}

func TestGetSubscriptionQuota_GLMExtraWindowsRendered(t *testing.T) {
	canned := &UsageData{
		Provider:  "glm-zhipu",
		PlanLevel: "lite",
		ExtraWindows: []UsageWindow{
			{Label: "Tok(u8,n2)", Percent: 11},
		},
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	assert.NotContains(t, out, "[LITE]")
	// 11% sits in the bright-green tier.
	assert.Contains(t, out, "\x1b[1;92m11%\x1b[0m Tok(u8,n2)")
}

// ---------------------------------------------------------------------------
// getSubscriptionUsage dispatcher
// ---------------------------------------------------------------------------

func TestGetSubscriptionUsage_DispatcherCustomReturnsNil(t *testing.T) {
	// Third-party proxy → no way to query its quota → must return nil so the
	// statusline simply hides the segment.
	t.Setenv("ANTHROPIC_BASE_URL", "https://my-proxy.example.com")
	t.Setenv("ANTHROPIC_API_BASE_URL", "")

	assert.Nil(t, getSubscriptionUsage(nil))
}

func TestGetSubscriptionUsage_DispatcherGLMNoTokenReturnsNil(t *testing.T) {
	// GLM detected but no token in env → return nil (config issue, not API
	// failure) so we don't write a poisoned failure cache entry.
	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	setupTempHomeDir(t)

	assert.Nil(t, getSubscriptionUsage(nil))
}

func TestGetSubscriptionUsage_GLMRateLimitHonorsRetryAfter(t *testing.T) {
	// GLM/Z.ai return Retry-After on rate limits too. The GLM path must feed
	// that into the shared cache backoff instead of treating 429 as a plain
	// network failure; otherwise the next statusline render can immediately
	// re-hit the quota endpoint.
	setupTempHomeDir(t)
	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "90")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	old := glmBaseURLOverride
	glmBaseURLOverride = srv.URL
	t.Cleanup(func() { glmBaseURLOverride = old })

	before := time.Now()
	got := getSubscriptionUsage(nil)

	assert.Nil(t, got, "no prior cache means a GLM 429 has no usage data to render")

	accountKey := glmAccountFingerprint("test-token")
	cached := readUsageCache("glm-zhipu", accountKey)
	require.NotNil(t, cached)
	assert.Equal(t, "rate-limited", cached.APIError)
	assert.Equal(t, 1, cached.RateLimitedCount)
	assert.True(t, cached.RetryAfterUntil.After(before.Add(80*time.Second)),
		"RetryAfterUntil=%v should honor the Retry-After window", cached.RetryAfterUntil)
	assert.True(t, cached.RetryAfterUntil.Before(before.Add(100*time.Second)),
		"RetryAfterUntil=%v should be close to Retry-After, not exponential fallback", cached.RetryAfterUntil)
}

func TestProviderCacheMatches(t *testing.T) {
	// Empty Provider in cache means "written by pre-multiprovider binary",
	// which historically wrote only Anthropic data. Must match "anthropic"
	// (no needless refresh on upgrade) and NOT match "glm-zhipu" (must
	// invalidate when user is now on GLM).
	old := &usageCacheData{Provider: ""}
	assert.True(t, providerCacheMatches(old, "anthropic"))
	assert.False(t, providerCacheMatches(old, "glm-zhipu"))

	glm := &usageCacheData{Provider: "glm-zhipu"}
	assert.True(t, providerCacheMatches(glm, "glm-zhipu"))
	assert.False(t, providerCacheMatches(glm, "anthropic"))

	// Nil cache: treat as "anything goes" so the caller doesn't trip on the
	// first-run-no-cache case.
	assert.True(t, providerCacheMatches(nil, "anthropic"))
}

func TestGetGLMUsage_EmptyCacheProviderTreatedAsAnthropicAndInvalidatedForGLM(t *testing.T) {
	// Regression: a cache written by a pre-multiprovider binary has empty
	// Provider. When the user switches the env to GLM, that cache must NOT
	// be served (it carries Anthropic-shaped data that we'd render under a
	// GLM PlanLevel prefix). After a fetch failure, we must return nil.
	homeDir := setupTempHomeDir(t)
	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")

	writeTestCacheFile(t, homeDir, &usageCacheData{
		// Provider intentionally empty — simulates a cache file written by
		// the v0.2.3 binary before this feature shipped.
		FiveHour:  53,
		SevenDay:  51,
		FetchedAt: time.Now().Add(-10 * time.Second),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	old := glmBaseURLOverride
	glmBaseURLOverride = srv.URL
	t.Cleanup(func() { glmBaseURLOverride = old })

	assert.Nil(t, getSubscriptionUsage(nil),
		"empty-Provider cache must be treated as Anthropic and invalidated for GLM")
}

func TestGetGLMUsage_ProviderMismatchOnFailureReturnsNilNotStaleData(t *testing.T) {
	// Regression: when the cache holds Anthropic data and the user switches
	// to GLM (new env vars), a fresh fetch failure must NOT render the
	// previous provider's numbers under a GLM [PLAN] prefix — that would be
	// more misleading than showing nothing.
	homeDir := setupTempHomeDir(t)
	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")

	// Seed cache with Anthropic data and an active timestamp.
	writeTestCacheFile(t, homeDir, &usageCacheData{
		Provider:        "anthropic",
		PlanLevel:       "Max",
		FiveHour:        53,
		SevenDay:        51,
		FiveHourResetAt: time.Now().Add(4 * time.Hour),
		SevenDayResetAt: time.Now().Add(17 * time.Hour),
		FetchedAt:       time.Now().Add(-30 * time.Second), // still fresh
	})

	// Server always 500s — fetch will fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	old := glmBaseURLOverride
	glmBaseURLOverride = srv.URL
	t.Cleanup(func() { glmBaseURLOverride = old })

	got := getSubscriptionUsage(nil)

	assert.Nil(t, got, "must not surface Anthropic cache data when GLM fetch fails")
}

func TestGetSubscriptionUsage_DispatcherGLMFetchesViaOverride(t *testing.T) {
	// Full GLM happy path: detector picks Zhipu, env has a token, override
	// redirects to httptest, response parses, cache gets written.
	setupTempHomeDir(t)
	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sanity-check the request reached us with the right path & token.
		if r.URL.Path != "/api/monitor/usage/quota/limit" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, fixtureQuotaMaxPlan)
	}))
	t.Cleanup(srv.Close)

	old := glmBaseURLOverride
	glmBaseURLOverride = srv.URL
	t.Cleanup(func() { glmBaseURLOverride = old })

	got := getSubscriptionUsage(nil)

	require.NotNil(t, got)
	assert.Equal(t, "glm-zhipu", got.Provider)
	assert.Equal(t, "max", got.PlanLevel)
	require.NotNil(t, got.MCP)
	assert.Equal(t, int64(42), got.MCP.Used)
}

// ---------------------------------------------------------------------------
// glmAccountFingerprint — multi-account cache discriminator
// ---------------------------------------------------------------------------

func TestGLMAccountFingerprint(t *testing.T) {
	cases := []struct {
		name     string
		token    string
		wantLen  int
		wantSame string // empty = no equality assertion
	}{
		{"empty token → empty fingerprint", "", 0, ""},
		{"whitespace-only token → empty", "   \t\n", 0, ""},
		{"non-empty token → 12 hex chars", "test-token", 12, ""},
		{"trimmed equals untrimmed", "  test-token  ", 12, "test-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := glmAccountFingerprint(tc.token)
			assert.Equal(t, tc.wantLen, len(got))
			if tc.wantSame != "" {
				assert.Equal(t, glmAccountFingerprint(tc.wantSame), got,
					"trimming must not change the fingerprint")
			}
		})
	}
}

func TestGLMAccountFingerprint_DistinctTokensProduceDistinctFingerprints(t *testing.T) {
	// Two synthetic, clearly-different tokens must produce different
	// fingerprints. Picked at random — any salt-of-the-earth pair works
	// because SHA-256 collisions in 48 bits are astronomically unlikely.
	a := glmAccountFingerprint("plan-pro-test-token-A")
	b := glmAccountFingerprint("plan-lite-test-token-B")
	assert.NotEqual(t, a, b)
}

func TestGLMAccountFingerprint_Deterministic(t *testing.T) {
	// Same input must produce the same output across repeated calls — the
	// cache file naming depends on it.
	first := glmAccountFingerprint("test-token")
	for i := 0; i < 5; i++ {
		assert.Equal(t, first, glmAccountFingerprint("test-token"))
	}
}

// ---------------------------------------------------------------------------
// getCachePath — per-(provider, accountKey) filename layout
// ---------------------------------------------------------------------------

func TestGetCachePath_PerProviderAndAccount(t *testing.T) {
	dir := filepath.FromSlash("/tmp/.claude")

	cases := []struct {
		name       string
		provider   string
		accountKey string
		want       string
	}{
		{
			"anthropic with empty key → legacy filename",
			"anthropic", "",
			filepath.Join(dir, ".usage-cache.json"),
		},
		{
			"anthropic ignores accountKey (config dir is the discriminator)",
			"anthropic", "abc123",
			filepath.Join(dir, ".usage-cache.json"),
		},
		{
			"glm-zhipu with accountKey → per-account filename",
			"glm-zhipu", "a1b2c3d4e5f6",
			filepath.Join(dir, ".usage-cache.glm-zhipu.a1b2c3d4e5f6.json"),
		},
		{
			"glm-zhipu without accountKey → defensive fallback (no FP suffix)",
			"glm-zhipu", "",
			filepath.Join(dir, ".usage-cache.glm-zhipu.json"),
		},
		{
			"glm-zai with accountKey → per-account filename",
			"glm-zai", "deadbeef1234",
			filepath.Join(dir, ".usage-cache.glm-zai.deadbeef1234.json"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := getCachePath(dir, tc.provider, tc.accountKey)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetCachePath_DistinctAccountsOnSameProviderGetDistinctFiles(t *testing.T) {
	// The whole reason the accountKey arg exists: two GLM accounts on the
	// same provider must not share a cache file.
	dir := filepath.FromSlash("/tmp/.claude")
	pro := getCachePath(dir, "glm-zhipu", "aaaaaaaaaaaa")
	lite := getCachePath(dir, "glm-zhipu", "bbbbbbbbbbbb")
	assert.NotEqual(t, pro, lite, "different accountKeys must land in different files")
}

// ---------------------------------------------------------------------------
// readUsageCache — defensive AccountKey check
// ---------------------------------------------------------------------------

func TestReadUsageCache_AccountKeyMismatchReturnsNil(t *testing.T) {
	// Belt-and-suspenders: in normal operation each (provider, accountKey)
	// pair has its own file, so a mismatch can't happen via the read API.
	// But a stale or hand-copied file with the wrong AccountKey inside it
	// must still be rejected — we don't want one account's quota leaking
	// into another's display.
	homeDir := setupTempHomeDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	claudeDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	stale := &usageCacheData{
		Provider:   "glm-zhipu",
		AccountKey: "aaaaaaaaaaaa",
		FiveHour:   42,
		FetchedAt:  time.Now(),
	}
	data, err := json.Marshal(stale)
	require.NoError(t, err)
	// Drop the file under the path the *new* fingerprint would read from,
	// then ask for that new fingerprint — the inner AccountKey will mismatch.
	target := filepath.Join(claudeDir, ".usage-cache.glm-zhipu.bbbbbbbbbbbb.json")
	require.NoError(t, os.WriteFile(target, data, 0644))

	got := readUsageCache("glm-zhipu", "bbbbbbbbbbbb")
	assert.Nil(t, got, "AccountKey mismatch inside the file must invalidate the read")
}

// ---------------------------------------------------------------------------
// getGLMUsage — round-trip persistence + multi-account isolation
// ---------------------------------------------------------------------------

func TestGetGLMUsage_PersistsAccountKeyInCache(t *testing.T) {
	// The cache file written after a successful fetch must carry the
	// fingerprint so that providerCacheMatches + the per-account filename
	// stay in sync.
	homeDir := setupTempHomeDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token-pro")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, fixtureQuotaMaxPlan)
	}))
	t.Cleanup(srv.Close)
	old := glmBaseURLOverride
	glmBaseURLOverride = srv.URL
	t.Cleanup(func() { glmBaseURLOverride = old })

	got := getSubscriptionUsage(nil)
	require.NotNil(t, got)

	wantFP := glmAccountFingerprint("test-token-pro")
	assert.Equal(t, wantFP, got.AccountKey)

	// File must land at the per-account path, not the legacy one.
	cachePath := filepath.Join(homeDir, ".claude", ".usage-cache.glm-zhipu."+wantFP+".json")
	_, err := os.Stat(cachePath)
	assert.NoError(t, err, "per-account cache file must exist at the fingerprinted path")

	// And the JSON on disk must carry AccountKey too — otherwise the
	// defensive check inside readUsageCache would reject the file on the
	// very next call.
	raw, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	var onDisk usageCacheData
	require.NoError(t, json.Unmarshal(raw, &onDisk))
	assert.Equal(t, wantFP, onDisk.AccountKey)
	assert.Equal(t, "glm-zhipu", onDisk.Provider)
}

func TestGetGLMUsage_TwoAccountsOnSameProviderHaveIsolatedCaches(t *testing.T) {
	// The headline scenario: one user, one provider (glm-zhipu), two
	// different tokens (Pro + Lite). Each must end up with its own cache
	// file; neither should clobber the other.
	homeDir := setupTempHomeDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")

	// Account A — Pro plan response.
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, fixtureQuotaProPlanWithWeekly)
	}))
	t.Cleanup(srvA.Close)

	old := glmBaseURLOverride
	t.Cleanup(func() { glmBaseURLOverride = old })

	glmBaseURLOverride = srvA.URL
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token-account-A")
	gotA := getSubscriptionUsage(nil)
	require.NotNil(t, gotA, "first account fetch must succeed")

	// Account B — Max plan response. Same provider, different token.
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, fixtureQuotaMaxPlan)
	}))
	t.Cleanup(srvB.Close)
	glmBaseURLOverride = srvB.URL
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token-account-B")
	gotB := getSubscriptionUsage(nil)
	require.NotNil(t, gotB, "second account fetch must succeed")

	// Both cache files must exist side by side.
	fpA := glmAccountFingerprint("test-token-account-A")
	fpB := glmAccountFingerprint("test-token-account-B")
	assert.NotEqual(t, fpA, fpB)

	pathA := filepath.Join(homeDir, ".claude", ".usage-cache.glm-zhipu."+fpA+".json")
	pathB := filepath.Join(homeDir, ".claude", ".usage-cache.glm-zhipu."+fpB+".json")
	_, errA := os.Stat(pathA)
	_, errB := os.Stat(pathB)
	assert.NoError(t, errA, "account A's cache file must exist")
	assert.NoError(t, errB, "account B's cache file must exist")

	// Switching back to account A must read A's file, not B's. We hit this
	// by re-invoking with A's token; the cache is still fresh (just written),
	// so the call returns from the cache layer without hitting the server.
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token-account-A")
	// Point the override at a 500-server to make sure we're NOT hitting the
	// network — if we are, the test will fail loudly.
	srv500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv500.Close)
	glmBaseURLOverride = srv500.URL

	gotARepeat := getSubscriptionUsage(nil)
	require.NotNil(t, gotARepeat, "cached A data must still be readable after writing B")
	assert.Equal(t, fpA, gotARepeat.AccountKey, "must read account A's cache, not B's")
	assert.Equal(t, "pro", gotARepeat.PlanLevel, "account A's plan (Pro) must be preserved")
}

// ---------------------------------------------------------------------------
// glmPlanWindows + getSubscriptionQuota: nil-quota / fresh-window rendering
// ---------------------------------------------------------------------------

// TestGLMPlanWindows pins the plan → (has5h, has7d) mapping. Without these
// flags, the 5h segment for a GLM Max user disappears every time the window
// resets (API returns percentage=0 with nextResetTime=0 until a token is
// spent in the new window).
func TestGLMPlanWindows(t *testing.T) {
	cases := []struct {
		plan   string
		want5h bool
		want7d bool
	}{
		{"max", true, false},
		{"Max", true, false},     // case-insensitive
		{"  MAX  ", true, false}, // trimmed
		{"pro", true, true},
		{"Pro", true, true},
		{"lite", true, true},
		{"Lite", true, true},
		{"", false, false}, // unknown / missing
		{"enterprise", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.plan, func(t *testing.T) {
			got5h, got7d := glmPlanWindows(tc.plan)
			assert.Equal(t, tc.want5h, got5h)
			assert.Equal(t, tc.want7d, got7d)
		})
	}
}

// TestGetSubscriptionQuota_GLMMax_FreshWindowStillShows5h is the direct
// regression for the BUG the user reported: cache says five_hour=0 with a
// zero reset_at (because the API returned nextResetTime=0 right after a
// reset), and previously the 5h segment vanished — leaving just
// "📊 [Max] 🧩 42/4k". With glmPlanWindows in place, the 5h line stays
// visible as "0% 5h ↻ now".
func TestGetSubscriptionQuota_GLMMax_FreshWindowStillShows5h(t *testing.T) {
	canned := &UsageData{
		Provider:  "glm-zhipu",
		PlanLevel: "max",
		FiveHour:  0,
		// FiveHourResetAt left at zero — this is the post-reset state.
		MCP: &MCPWindow{Used: 42, Limit: 4000, Percent: 1},
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	// Must include the 5h segment with "↻ now" countdown.
	assert.Contains(t, out, "5h ↻ now", "5h segment must stay visible during a fresh window")
	// Must NOT include the 7d segment (Max has no weekly window).
	assert.NotContains(t, out, "7d", "Max plan has no 7d window — must not be synthesized")
	// MCP still renders.
	assert.Contains(t, out, "🧩 42/4k")
}

// TestGetSubscriptionQuota_GLMPro_FreshWindowStillShows5hAnd7d covers the
// Pro variant where both 5h and 7d should stay visible across a reset.
func TestGetSubscriptionQuota_GLMPro_FreshWindowStillShows5hAnd7d(t *testing.T) {
	canned := &UsageData{
		Provider:  "glm-zhipu",
		PlanLevel: "pro",
		// Both windows freshly reset.
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	assert.Contains(t, out, "5h ↻ now")
	assert.Contains(t, out, "7d ↻ now")
}

// TestGetSubscriptionQuota_GLMUnknownPlan_KeepsOldHideBehaviour pins that
// the fix is plan-level-gated: an empty PlanLevel falls back to the prior
// "render only when there's data" behaviour, so we don't synthesize ghost
// segments for MCP-only / API-key accounts we know nothing about.
func TestGetSubscriptionQuota_GLMUnknownPlan_KeepsOldHideBehaviour(t *testing.T) {
	canned := &UsageData{
		Provider: "glm-zhipu",
		// PlanLevel empty, no live data, MCP only.
		MCP: &MCPWindow{Used: 1, Limit: 100, Percent: 1},
	}
	oldFn := getSubscriptionUsageFn
	getSubscriptionUsageFn = func() *UsageData { return canned }
	t.Cleanup(func() { getSubscriptionUsageFn = oldFn })

	out := getSubscriptionQuota(&StatusLineInput{})

	assert.NotContains(t, out, "5h", "unknown plan must not synthesize a 5h segment")
	assert.NotContains(t, out, "7d", "unknown plan must not synthesize a 7d segment")
	assert.Contains(t, out, "🧩 1/100")
}
