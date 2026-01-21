// Package content provides content collection for the statusline
// Content Layer: Data collection and type definitions
package content

import (
	"time"
)

// ContentType defines the type of content
type ContentType string

const (
	ContentFolder          ContentType = "folder"
	ContentModel           ContentType = "model"
	ContentTokenBar        ContentType = "token-bar"
	ContentTokenInfo       ContentType = "token-info"
	ContentClaudeVersion   ContentType = "claude-version"
	ContentGitBranch       ContentType = "git-branch"
	ContentGitStatus       ContentType = "git-status"
	ContentGitRemote       ContentType = "git-remote"
	ContentMemoryFiles     ContentType = "memory-files"
	ContentAgent           ContentType = "agent"
	ContentTodo            ContentType = "todo"
	ContentTools           ContentType = "tools"
	ContentCurrentTime     ContentType = "current-time"
	ContentSessionDuration ContentType = "session-duration"
	ContentQuota           ContentType = "quota"
)

// Content represents a content fragment
type Content struct {
	Type     ContentType
	Value    string
	Priority int           // For layout decisions
	CacheTTL time.Duration // Cache time
}

// ContentCollector is the interface for content collectors
type ContentCollector interface {
	Type() ContentType
	Collect(input interface{}, summary interface{}) (string, error)
	CacheTTL() time.Duration
	Optional() bool // Returns true if content is optional (can be empty)
}

// cachedContent holds cached content with expiration
type cachedContent struct {
	value     string
	expiresAt time.Time
}

// isExpired checks if cached content has expired
func (c *cachedContent) isExpired() bool {
	return time.Now().After(c.expiresAt)
}
