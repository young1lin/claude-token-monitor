package content

import (
	"strings"
)

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

// classifyProviderBaseURL maps a resolved (already-trimmed) base URL to its
// provider kind. This is the single source of truth for GLM detection: it is
// called once from detectProviderInfo and stashed on env.Provider.Kind, so
// the quota chain never re-reads $ANTHROPIC_BASE_URL / $ANTHROPIC_API_BASE_URL
// to re-derive the kind. Both base-URL spellings occur in user configs; the
// fallback is resolved by the caller (detectProviderInfo) before this runs.
//
// Pure (no env / no I/O) so it is unit-testable without t.Setenv.
func classifyProviderBaseURL(baseURL string) providerKind {
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
