package content

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/young1lin/claude-token-monitor/internal/claudedir"
)

// Env is the per-process context, built once at startup and passed to every
// collector. Bundles all "parse once" info so collectors never read env vars,
// globals, or time.Now directly.
type Env struct {
	Input     *StatusLineInput
	Summary   *TranscriptSummary
	OS        OSInfo
	Terminal  TerminalInfo
	Now       time.Time
	Provider  ProviderInfo
	ClaudeDir string
}

type OSInfo struct {
	Name      string // runtime.GOOS
	IsWindows bool
	IsDarwin  bool
}

type TerminalInfo struct {
	Program     string // raw TERM_PROGRAM
	IsWTSession bool   // WT_SESSION non-empty
	NarrowBlock bool   // Block Elements (█░) render at width 1
	AmbigWide   bool   // East Asian Ambiguous (·↻) render at width 2
}

type ProviderInfo struct {
	Kind      providerKind // resolved from BaseURL once in BuildEnv (providerAnthropic / providerGLMZai / providerGLMZhipu / providerCustom)
	BaseURL   string       // resolved ANTHROPIC_BASE_URL / ANTHROPIC_API_BASE_URL (trimmed)
	AuthToken string       // ANTHROPIC_AUTH_TOKEN (trimmed)
}

// Injectable seams for unit tests (FIRST: no real env/process).
//
// nowFn is intentionally NOT redeclared here: the package already declares a
// shared time seam at quota.go (used by the time + quota render paths). Env
// reuses it so there is a single pinned clock across the whole package.
var (
	goosFn           = func() string { return runtime.GOOS }
	detectTerminalFn = detectTerminal
	detectProviderFn = detectProviderInfo
)

// BuildEnv reads all env/config once and assembles the context.
func BuildEnv(input *StatusLineInput, summary *TranscriptSummary) *Env {
	osName := goosFn()
	claudeDir, _ := claudedir.Resolve(os.UserHomeDir)
	return &Env{
		Input:     input,
		Summary:   summary,
		OS:        OSInfo{Name: osName, IsWindows: osName == "windows", IsDarwin: osName == "darwin"},
		Terminal:  detectTerminalFn(),
		Now:       nowFn(),
		Provider:  detectProviderFn(),
		ClaudeDir: claudeDir,
	}
}

// detectTerminal reads terminal env once. Migrated from main.go's
// detectWideCharTerminal / detectNarrowBlockTerminal. Statusline stdin carries
// NO terminal fields (verified via --debug 2026-07-18), so this must come from
// the process environment.
func detectTerminal() TerminalInfo {
	program := os.Getenv("TERM_PROGRAM")
	ambigWide := os.Getenv("STATUSLINE_AMBIGUOUS_WIDE") == "1"
	narrow := false
	switch program {
	case "Apple_Terminal", "vscode", "WarpTerminal":
		narrow = true
	}
	if goosFn() == "windows" && os.Getenv("WT_SESSION") == "" {
		narrow = true
	}
	return TerminalInfo{
		Program:     program,
		IsWTSession: os.Getenv("WT_SESSION") != "",
		NarrowBlock: narrow,
		AmbigWide:   ambigWide,
	}
}

// detectProviderInfo resolves the API provider once by reading the three
// ANTHROPIC_* env vars (primary base URL, legacy fallback base URL, auth
// token) and classifying the resolved base URL via classifyProviderBaseURL.
// The result is stashed on env.Provider so the quota collectors never read
// os.Getenv again — getSubscriptionUsage switches on provider.Kind (and
// getGLMUsage / getAnthropicUsage consume this struct directly).
//
// Trimming happens here so the rest of the chain can treat BaseURL and
// AuthToken as already-normalized.
func detectProviderInfo() ProviderInfo {
	baseURL := strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("ANTHROPIC_API_BASE_URL"))
	}
	return ProviderInfo{
		Kind:      classifyProviderBaseURL(baseURL),
		BaseURL:   baseURL,
		AuthToken: strings.TrimSpace(os.Getenv("ANTHROPIC_AUTH_TOKEN")),
	}
}
