package content

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEnv_PopulatesCoreFields(t *testing.T) {
	oldT, oldP, oldN, oldG := detectTerminalFn, detectProviderFn, nowFn, goosFn
	defer func() {
		detectTerminalFn, detectProviderFn, nowFn, goosFn = oldT, oldP, oldN, oldG
	}()
	fixedNow := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	nowFn = func() time.Time { return fixedNow }
	goosFn = func() string { return "darwin" }
	detectTerminalFn = func() TerminalInfo {
		return TerminalInfo{Program: "Apple_Terminal", NarrowBlock: true}
	}
	detectProviderFn = func() ProviderInfo { return ProviderInfo{Kind: providerGLMZhipu} }

	in := &StatusLineInput{Cwd: "/x"}
	env := BuildEnv(in, &TranscriptSummary{})

	require.NotNil(t, env)
	assert.Equal(t, "darwin", env.OS.Name)
	assert.True(t, env.OS.IsDarwin)
	assert.False(t, env.OS.IsWindows)
	assert.Equal(t, "Apple_Terminal", env.Terminal.Program)
	assert.True(t, env.Terminal.NarrowBlock)
	assert.Equal(t, fixedNow, env.Now)
	assert.Equal(t, providerGLMZhipu, env.Provider.Kind)
	assert.Same(t, in, env.Input)
}

// TestBuildEnv_ClauDir pins the single claudedir.Resolve call site.
// Every collector that needs the active Claude config dir threads env.ClaudeDir
// from here; if this stops resolving, the multi-account regression (wrong
// account's cache/credentials/skills surfacing) silently returns.
func TestBuildEnv_ClaudeDir(t *testing.T) {
	t.Run("honors CLAUDE_CONFIG_DIR", func(t *testing.T) {
		custom := filepath.FromSlash("/tmp/account-ME")
		t.Setenv("CLAUDE_CONFIG_DIR", custom)

		env := BuildEnv(&StatusLineInput{}, &TranscriptSummary{})

		assert.Equal(t, custom, env.ClaudeDir,
			"env.ClaudeDir must mirror the dir claudedir.Resolve picks")
	})

	t.Run("falls back to <home>/.claude when env unset", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")

		env := BuildEnv(&StatusLineInput{}, &TranscriptSummary{})

		// Only assert shape — the actual home dir is host-dependent. The
		// claudedir package covers the resolution semantics in detail.
		require.NotEmpty(t, env.ClaudeDir, "must resolve a non-empty default")
		assert.True(t, filepath.IsAbs(env.ClaudeDir),
			"default ClaudeDir must be absolute, got %q", env.ClaudeDir)
	})
}

// TestDetectProviderInfo pins the single env-read point for the provider
// triple (kind / base URL / auth token). All downstream quota helpers consume
// the ProviderInfo this produces; if this stops reading an env var the whole
// chain silently degrades, hence the dedicated coverage.
func TestDetectProviderInfo(t *testing.T) {
	cases := []struct {
		name      string
		baseURL   string
		apiBase   string
		authToken string
		wantKind  providerKind
		wantURL   string
	}{
		{
			name:     "both base URLs unset → anthropic, empty url",
			baseURL:  "",
			apiBase:  "",
			wantKind: providerAnthropic,
			wantURL:  "",
		},
		{
			name:     "explicit Anthropic base URL",
			baseURL:  "https://api.anthropic.com",
			wantKind: providerAnthropic,
			wantURL:  "https://api.anthropic.com",
		},
		{
			name:     "GLM Zhipu subpath → glm-zhipu",
			baseURL:  "https://open.bigmodel.cn/api/anthropic",
			wantKind: providerGLMZhipu,
			wantURL:  "https://open.bigmodel.cn/api/anthropic",
		},
		{
			name:     "GLM Z.ai → glm-zai",
			baseURL:  "https://api.z.ai",
			wantKind: providerGLMZai,
			wantURL:  "https://api.z.ai",
		},
		{
			name:     "third-party proxy → custom",
			baseURL:  "https://my-router.example.com",
			wantKind: providerCustom,
			wantURL:  "https://my-router.example.com",
		},
		{
			name:     "falls back to ANTHROPIC_API_BASE_URL",
			baseURL:  "",
			apiBase:  "https://open.bigmodel.cn",
			wantKind: providerGLMZhipu,
			wantURL:  "https://open.bigmodel.cn",
		},
		{
			name:      "primary base URL wins over fallback",
			baseURL:   "https://api.z.ai",
			apiBase:   "https://open.bigmodel.cn",
			wantKind:  providerGLMZai,
			wantURL:   "https://api.z.ai",
			authToken: "tok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ANTHROPIC_BASE_URL", tc.baseURL)
			t.Setenv("ANTHROPIC_API_BASE_URL", tc.apiBase)
			t.Setenv("ANTHROPIC_AUTH_TOKEN", tc.authToken)

			got := detectProviderInfo()

			assert.Equal(t, tc.wantKind, got.Kind)
			assert.Equal(t, tc.wantURL, got.BaseURL)
			assert.Equal(t, tc.authToken, got.AuthToken)
		})
	}

	t.Run("trims whitespace around base URL and token", func(t *testing.T) {
		t.Setenv("ANTHROPIC_BASE_URL", "  https://api.z.ai  ")
		t.Setenv("ANTHROPIC_API_BASE_URL", "")
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "  test-token  ")

		got := detectProviderInfo()

		assert.Equal(t, providerGLMZai, got.Kind, "trimmed URL must still classify as GLM")
		assert.Equal(t, "https://api.z.ai", got.BaseURL, "BaseURL must be trimmed")
		assert.Equal(t, "test-token", got.AuthToken, "AuthToken must be trimmed")
	})
}

func TestDetectTerminal_AppleTerminalNarrowBlock(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	t.Setenv("STATUSLINE_AMBIGUOUS_WIDE", "")
	old := goosFn
	t.Cleanup(func() { goosFn = old })
	goosFn = func() string { return "darwin" }
	ti := detectTerminal()
	assert.True(t, ti.NarrowBlock)
	assert.False(t, ti.AmbigWide)
	assert.Equal(t, "Apple_Terminal", ti.Program)
}

func TestDetectTerminal_AmbigWideOptIn(t *testing.T) {
	t.Setenv("STATUSLINE_AMBIGUOUS_WIDE", "1")
	ti := detectTerminal()
	assert.True(t, ti.AmbigWide)
}

func TestDetectTerminal_WindowsCmdNarrow(t *testing.T) {
	old := goosFn
	defer func() { goosFn = old }()
	goosFn = func() string { return "windows" }
	t.Setenv("WT_SESSION", "")
	ti := detectTerminal()
	assert.True(t, ti.NarrowBlock)
}

// Windows Terminal (WT_SESSION set) renders Block Elements (█░) at width 1 —
// the same as conhost — NOT width 2. Cascadia Code (WT's default font) draws
// every U+2580–U+259F glyph in a single cell. If NarrowBlock stays false here,
// displayWidth counts the 10-cell progress bar as 20, the progress-bar row's
// column gets padded ~10 cells too wide, and its trailing "|" drifts ~10 cells
// left of the other rows. Verified via repro: row1 col1 = 64 (narrow=false) vs
// 54 (narrow=true); the 10-cell delta is exactly the bar.
func TestDetectTerminal_WindowsTerminalNarrowBlock(t *testing.T) {
	old := goosFn
	t.Cleanup(func() { goosFn = old })
	goosFn = func() string { return "windows" }
	t.Setenv("WT_SESSION", "wt-test-session")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("STATUSLINE_AMBIGUOUS_WIDE", "")
	ti := detectTerminal()
	assert.True(t, ti.NarrowBlock,
		"Windows Terminal renders Block Elements narrow (width 1), so NarrowBlock must be true")
	assert.False(t, ti.AmbigWide,
		"Windows Terminal renders East Asian Ambiguous narrow (1 cell, probe-verified 2026-07-19), so AmbigWide defaults to false")
	assert.True(t, ti.IsWTSession)
}

// TestDetectTerminal_WTAmbigWideOverrideOff verifies STATUSLINE_AMBIGUOUS_WIDE=0
// keeps Ambiguous narrow on Windows Terminal — same as the unset default, kept
// as an explicit opt-out for symmetry with the =1 opt-in.
func TestDetectTerminal_WTAmbigWideOverrideOff(t *testing.T) {
	old := goosFn
	t.Cleanup(func() { goosFn = old })
	goosFn = func() string { return "windows" }
	t.Setenv("WT_SESSION", "wt-test-session")
	t.Setenv("STATUSLINE_AMBIGUOUS_WIDE", "0")
	ti := detectTerminal()
	assert.False(t, ti.AmbigWide, "=0 must keep Ambiguous narrow")
}
