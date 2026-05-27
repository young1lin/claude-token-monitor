package content

import (
	"os"
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
