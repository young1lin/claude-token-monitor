package content

import (
	"os/exec"
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

// Collect returns the Claude Code version
func (c *ClaudeVersionCollector) Collect(input interface{}, summary interface{}) (string, error) {
	return getClaudeVersionCached(), nil
}

// getClaudeVersionCached returns cached Claude Code version
func getClaudeVersionCached() string {
	now := time.Now()

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

// getClaudeVersion fetches Claude Code version by running "claude --version"
func getClaudeVersion() string {
	cmd := exec.Command("claude", "--version")
	output, err := cmd.Output()
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
