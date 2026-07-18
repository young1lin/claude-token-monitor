// Package content provides content collection for the statusline
// Content Layer: Data collection and type definitions
package content

import (
	"time"
)

// ContentType defines the type of content
type ContentType string

const (
	ContentFolder           ContentType = "folder"
	ContentModel            ContentType = "model"
	ContentTokenBar         ContentType = "token-bar"
	ContentTokenInfo        ContentType = "token-info"
	ContentClaudeVersion    ContentType = "claude-version"
	ContentGitBranch        ContentType = "git-branch"
	ContentGitStatus        ContentType = "git-status"
	ContentGitRemote        ContentType = "git-remote"
	ContentGitWorktree      ContentType = "git-worktree"
	ContentMemoryFiles      ContentType = "memory-files"
	ContentAgent            ContentType = "agent"
	ContentTodo             ContentType = "todo"
	ContentTools            ContentType = "tools"
	ContentCurrentTime      ContentType = "current-time"
	ContentSessionDuration  ContentType = "session-duration"
	ContentSkills           ContentType = "skills"
	ContentQuota            ContentType = "quota"
	ContentToolStatusDetail ContentType = "tool-status-detail"
	ContentParentMemory     ContentType = "parent-memory"
	ContentSessionTotal     ContentType = "session-total"
	ContentModeFlags        ContentType = "mode-flags"
)

// Content represents a content fragment
type Content struct {
	Type     ContentType
	Value    string
	Priority int           // For layout decisions
	CacheTTL time.Duration // Cache time
}

// ContentCollector is the interface for content collectors.
//
// Collect receives the per-process Env, which bundles the already-parsed stdin
// payload (env.Input) and transcript summary (env.Summary) along with the
// resolved OS / terminal / provider context. Both are guaranteed non-nil by
// the Manager in production (main.go substitutes an empty TranscriptSummary
// when none is parsed), so collectors only nil-check when they are also
// exercised directly in unit tests.
type ContentCollector interface {
	Type() ContentType
	Collect(env *Env) (string, error)
	CacheTTL() time.Duration
	Timeout() time.Duration // Collector-specific timeout (0 = use manager default)
	Optional() bool         // Returns true if content is optional (can be empty)
}

// cachedContent holds cached content with expiration
type cachedContent struct {
	value     string
	expiresAt time.Time
}

// isExpired checks if cached content has expired.
//
// time.Now (not env.Now) is correct here: this is in-process cache mechanics
// (has the TTL elapsed?), not a display cell. The expiry check needs real
// wall-clock so a stale env.Now snapshot (captured once at BuildEnv) can't
// trick the cache into serving expired data. See manager.go Get for the twin.
func (c *cachedContent) isExpired() bool {
	return time.Now().After(c.expiresAt)
}
