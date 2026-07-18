package content

import (
	"strings"
	"sync"
	"time"
)

// Claude Code version cache
var (
	claudeVersionCache     string
	claudeVersionCacheMu   sync.RWMutex
	claudeVersionCacheTime time.Time
	claudeVersionCacheTTL  = 5 * time.Minute
)

// ClaudeVersionCollector collects the Claude Code version
type ClaudeVersionCollector struct {
	*BaseCollector
}

// NewClaudeVersionCollector creates a new Claude version collector
func NewClaudeVersionCollector() *ClaudeVersionCollector {
	return &ClaudeVersionCollector{
		BaseCollector: NewBaseCollector(ContentClaudeVersion, 5*time.Minute, true),
	}
}

// Collect returns the Claude Code version.
//
// Fast path: CC 2.1.x+ supplies "version" in the stdin payload, so we just
// echo it back. Fallback path: older CC builds don't send the field, so we
// run `claude --version` and cache the result for 5 minutes.
func (c *ClaudeVersionCollector) Collect(env *Env) (string, error) {
	statusInput := env.Input
	var version string
	if statusInput != nil && statusInput.Version != "" {
		version = statusInput.Version // stdin fast path — skip the subprocess fork
	} else {
		version = getClaudeVersionCached(env.Now)
	}
	if version == "" {
		return "", nil
	}
	return "v" + version, nil
}

// getClaudeVersionCached returns cached Claude Code version. The caller
// threads env.Now so the cache TTL check uses the same snapshot the rest of
// the statusline render sees (pinnable in tests via the nowFn seam BuildEnv
// reads from).
func getClaudeVersionCached(now time.Time) string {
	claudeVersionCacheMu.RLock()
	if claudeVersionCache != "" && now.Sub(claudeVersionCacheTime) < claudeVersionCacheTTL {
		cached := claudeVersionCache
		claudeVersionCacheMu.RUnlock()
		return cached
	}
	claudeVersionCacheMu.RUnlock()

	version := getClaudeVersion()

	claudeVersionCacheMu.Lock()
	claudeVersionCache = version
	claudeVersionCacheTime = now
	claudeVersionCacheMu.Unlock()

	return version
}

// clearVersionCache resets the version cache (for tests).
func clearVersionCache() {
	claudeVersionCacheMu.Lock()
	claudeVersionCache = ""
	claudeVersionCacheTime = time.Time{}
	claudeVersionCacheMu.Unlock()
}

// getClaudeVersion fetches Claude Code version by running "claude --version"
func getClaudeVersion() string {
	output, err := defaultCommandRunner.Run("", "claude", "--version")
	if err != nil {
		return ""
	}

	versionStr := strings.TrimSpace(string(output))
	parts := strings.Fields(versionStr)
	if len(parts) >= 1 {
		version := parts[0]
		version = strings.TrimPrefix(version, "v")
		version = strings.TrimPrefix(version, "V")
		return version
	}

	return versionStr
}
