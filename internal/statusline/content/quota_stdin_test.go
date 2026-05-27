package content

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadRealCCInput parses an anonymized snapshot of the JSON Claude Code 2.1.150
// emits on the statusline's stdin (see testdata/cc_2.1.150_stdin.json). Using
// the real on-the-wire shape — not a hand-rolled struct literal — protects us
// from schema drift: if CC renames a field or changes a unit, this fixture
// surfaces it as a test failure instead of a silent regression at runtime.
func loadRealCCInput(t *testing.T) *StatusLineInput {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cc_2.1.150_stdin.json"))
	require.NoError(t, err, "fixture must exist")
	var input StatusLineInput
	require.NoError(t, json.Unmarshal(data, &input), "fixture must decode into StatusLineInput")
	return &input
}

// ---------------------------------------------------------------------------
// Schema check — decoding the real CC payload populates the new fields
// ---------------------------------------------------------------------------

func TestStatusLineInput_DecodesRealCCPayload(t *testing.T) {
	input := loadRealCCInput(t)

	// Core fields we already handled — sanity check the fixture didn't drift.
	assert.Equal(t, "Opus 4.7 (1M context)", input.Model.DisplayName)
	assert.Equal(t, 1000000, input.ContextWindow.ContextWindowSize)

	// New fields added for CC 2.1.x stdin payload.
	assert.Equal(t, "2.1.150", input.Version, "version must decode from stdin")

	require.NotNil(t, input.RateLimits, "rate_limits must decode")
	require.NotNil(t, input.RateLimits.FiveHour)
	require.NotNil(t, input.RateLimits.SevenDay)

	assert.InDelta(t, 27.0, input.RateLimits.FiveHour.UsedPercentage, 0.01)
	assert.Equal(t, int64(1779798600), input.RateLimits.FiveHour.ResetsAt)
	assert.InDelta(t, 24.0, input.RateLimits.SevenDay.UsedPercentage, 0.01)
	assert.Equal(t, int64(1780272000), input.RateLimits.SevenDay.ResetsAt)
}

// ---------------------------------------------------------------------------
// buildAnthropicUsageFromStdin — fed by the real CC payload
// ---------------------------------------------------------------------------

func TestBuildAnthropicUsageFromStdin_FromRealPayload(t *testing.T) {
	homeDir := setupTempHomeDir(t)
	writeTestCredentials(t, homeDir, "tok", "max", time.Now().Add(24*time.Hour).UnixMilli())

	input := loadRealCCInput(t)
	got := buildAnthropicUsageFromStdin(input.RateLimits)

	require.NotNil(t, got)
	assert.Equal(t, "anthropic", got.Provider)
	assert.InDelta(t, 27.0, got.FiveHour, 0.01)
	assert.InDelta(t, 24.0, got.SevenDay, 0.01)
	assert.Equal(t, time.Unix(1779798600, 0), got.FiveHourResetAt)
	assert.Equal(t, time.Unix(1780272000, 0), got.SevenDayResetAt)
	assert.Equal(t, "Max", got.PlanLevel, "plan label must come from credentials")
}

func TestBuildAnthropicUsageFromStdin_MissingResetsAtStaysZero(t *testing.T) {
	// resets_at == 0 means "host doesn't know yet" — we must leave the time
	// field zero so countdown rendering knows to skip the "↻" decoration.
	setupTempHomeDir(t)

	got := buildAnthropicUsageFromStdin(&StdinRateLimits{
		FiveHour: &StdinRateLimitWindow{UsedPercentage: 5, ResetsAt: 0},
	})
	require.NotNil(t, got)
	assert.True(t, got.FiveHourResetAt.IsZero(), "ResetsAt=0 must produce zero time")
	assert.True(t, got.SevenDayResetAt.IsZero(), "nil SevenDay window must produce zero time")
}

func TestBuildAnthropicUsageFromStdin_NoCredentialsStillReturnsPercentages(t *testing.T) {
	// API-only / fresh install: no .credentials.json. PlanLevel must end up
	// empty, but percentages still flow through so the user sees their usage.
	setupTempHomeDir(t)

	input := loadRealCCInput(t)
	got := buildAnthropicUsageFromStdin(input.RateLimits)

	require.NotNil(t, got)
	assert.InDelta(t, 27.0, got.FiveHour, 0.01)
	assert.Empty(t, got.PlanLevel, "missing credentials → no plan label")
}

func TestBuildAnthropicUsageFromStdin_TeamPlan(t *testing.T) {
	// The user explicitly called out Team/Max as required to render — pin it.
	homeDir := setupTempHomeDir(t)
	writeTestCredentials(t, homeDir, "tok", "claude-team", time.Now().Add(24*time.Hour).UnixMilli())

	input := loadRealCCInput(t)
	got := buildAnthropicUsageFromStdin(input.RateLimits)
	require.NotNil(t, got)
	assert.Equal(t, "Team", got.PlanLevel)
}

// ---------------------------------------------------------------------------
// getAnthropicUsage — stdin fast path must NOT hit the OAuth API
// ---------------------------------------------------------------------------

func TestGetAnthropicUsage_StdinFastPath_SkipsAPI(t *testing.T) {
	// Spy on usageAPIURL. Any hit here means the fast path is broken.
	var hits int32
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	homeDir := setupTempHomeDir(t)
	writeTestCredentials(t, homeDir, "tok", "max", time.Now().Add(24*time.Hour).UnixMilli())

	got := getAnthropicUsage(loadRealCCInput(t))

	require.NotNil(t, got)
	assert.InDelta(t, 27.0, got.FiveHour, 0.01)
	assert.Equal(t, "Max", got.PlanLevel)
	assert.Equal(t, int32(0), atomic.LoadInt32(&hits), "fast path must not call OAuth /usage")
}

func TestGetAnthropicUsage_FallbackHitsAPIWhenStdinAbsent(t *testing.T) {
	// Strip rate_limits — emulates older CC. The slow path must take over.
	var hits int32
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"five_hour":{"utilization":42,"resets_at":"2030-01-01T00:00:00Z"},
			"seven_day":{"utilization":7, "resets_at":"2030-01-08T00:00:00Z"}
		}`))
	})

	homeDir := setupTempHomeDir(t)
	writeTestCredentials(t, homeDir, "tok", "claude-pro", time.Now().Add(24*time.Hour).UnixMilli())

	input := loadRealCCInput(t)
	input.RateLimits = nil // pretend host predates the field

	got := getAnthropicUsage(input)

	require.NotNil(t, got)
	assert.InDelta(t, 42.0, got.FiveHour, 0.01)
	assert.Equal(t, "Pro", got.PlanLevel)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "fallback path must call OAuth /usage exactly once")
}

func TestGetAnthropicUsage_NilInputStillUsesAPI(t *testing.T) {
	// Callers that pass nil (existing GLM tests, defensive paths) must keep
	// working. We don't crash and we route to the API.
	var hits int32
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	homeDir := setupTempHomeDir(t)
	writeTestCredentials(t, homeDir, "tok", "max", time.Now().Add(24*time.Hour).UnixMilli())

	_ = getAnthropicUsage(nil)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "nil input still routes to API")
}

// ---------------------------------------------------------------------------
// getSubscriptionQuota — end-to-end rendering through the stdin fast path
// ---------------------------------------------------------------------------

func TestGetSubscriptionQuota_StdinPath_RendersPlanLabel(t *testing.T) {
	// Pin "now" so the 4h32m countdown is deterministic. Fixture's 5h reset
	// is 1779798600; back off by 4h32m to land in the middle of the window.
	mockNow(t, time.Unix(1779798600, 0).Add(-4*time.Hour-32*time.Minute))

	// Force Anthropic provider regardless of the host's ANTHROPIC_BASE_URL.
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("ANTHROPIC_API_BASE_URL", "")

	homeDir := setupTempHomeDir(t)
	writeTestCredentials(t, homeDir, "tok", "max", time.Now().Add(24*time.Hour).UnixMilli())

	// Force the dispatcher to take the real path (not the test override).
	mockSubscriptionUsage(t, nil)
	// And shield from any leaked HTTP traffic: if the stdin path is broken
	// and someone calls the API, this server returns 99% so the assertion
	// below fails loudly instead of silently passing.
	setupTestAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":99,"resets_at":"2030-01-01T00:00:00Z"}}`))
	})

	out := getSubscriptionQuota(loadRealCCInput(t))
	assert.Contains(t, out, "[Max]", "Max plan label must appear when credentials say claude-max")
	assert.Contains(t, out, "27%", "5h percentage must come from stdin, not API")
	assert.NotContains(t, out, "99%", "API value must NOT leak into stdin-path output")
}
