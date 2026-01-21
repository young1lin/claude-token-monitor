// Package composers provides built-in content composers for the statusline
package composers

import (
	"github.com/young1lin/claude-token-monitor/internal/statusline/content"
)

// GitComposer combines git branch, status, and remote into a single display
// Default format: 🌿 branch status remote
type GitComposer struct {
	composer content.Composer
}

// NewGitComposer creates a new git composer with the default format
func NewGitComposer() *GitComposer {
	return &GitComposer{
		composer: content.NewFormatComposer("git", []content.ContentType{
			content.ContentGitBranch,
			content.ContentGitStatus,
			content.ContentGitRemote,
		}, func(contents map[content.ContentType]string) string {
			branch := contents[content.ContentGitBranch]
			status := contents[content.ContentGitStatus]
			remote := contents[content.ContentGitRemote]

			line := ""
			if branch != "" {
				line = "🌿 " + branch
			}
			if status != "" {
				if line != "" {
					line += " " + status
				} else {
					line = status
				}
			}
			if remote != "" {
				if line != "" {
					line += " " + remote
				} else {
					line = remote
				}
			}
			return line
		}),
	}
}

// NewGitComposerBranchOnly creates a git composer that only shows the branch
func NewGitComposerBranchOnly() *GitComposer {
	return &GitComposer{
		composer: content.NewFormatComposer("git-branch-only", []content.ContentType{
			content.ContentGitBranch,
		}, func(contents map[content.ContentType]string) string {
			branch := contents[content.ContentGitBranch]
			if branch != "" {
				return "🌿 " + branch
			}
			return ""
		}),
	}
}

// NewGitComposerWithStatus creates a git composer that shows branch and status only
func NewGitComposerWithStatus() *GitComposer {
	return &GitComposer{
		composer: content.NewFormatComposer("git-branch-status", []content.ContentType{
			content.ContentGitBranch,
			content.ContentGitStatus,
		}, func(contents map[content.ContentType]string) string {
			branch := contents[content.ContentGitBranch]
			status := contents[content.ContentGitStatus]

			line := ""
			if branch != "" {
				line = "🌿 " + branch
			}
			if status != "" {
				if line != "" {
					line += " " + status
				} else {
					line = status
				}
			}
			return line
		}),
	}
}

// Name returns the composer's name
func (c *GitComposer) Name() string {
	return c.composer.Name()
}

// InputTypes returns the content types this composer consumes
func (c *GitComposer) InputTypes() []content.ContentType {
	return c.composer.InputTypes()
}

// Compose combines the git-related contents
func (c *GitComposer) Compose(contents map[content.ContentType]string) string {
	return c.composer.Compose(contents)
}

// GitComposerConfig represents configuration for a custom git composer
type GitComposerConfig struct {
	Name         string
	ShowStatus   bool
	ShowRemote   bool
	BranchPrefix string // E.g., "🌿 "
}

// NewGitComposerFromConfig creates a git composer from configuration
func NewGitComposerFromConfig(cfg GitComposerConfig) *GitComposer {
	inputTypes := []content.ContentType{content.ContentGitBranch}
	if cfg.ShowStatus {
		inputTypes = append(inputTypes, content.ContentGitStatus)
	}
	if cfg.ShowRemote {
		inputTypes = append(inputTypes, content.ContentGitRemote)
	}

	return &GitComposer{
		composer: content.NewFormatComposer(cfg.Name, inputTypes, func(contents map[content.ContentType]string) string {
			branch := contents[content.ContentGitBranch]
			status := contents[content.ContentGitStatus]
			remote := contents[content.ContentGitRemote]

			line := ""
			if branch != "" {
				if cfg.BranchPrefix != "" {
					line = cfg.BranchPrefix + branch
				} else {
					line = branch
				}
			}
			if cfg.ShowStatus && status != "" {
				if line != "" {
					line += " " + status
				} else {
					line = status
				}
			}
			if cfg.ShowRemote && remote != "" {
				if line != "" {
					line += " " + remote
				} else {
					line = remote
				}
			}
			return line
		}),
	}
}
