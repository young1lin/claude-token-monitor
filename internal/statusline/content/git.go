package content

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Git caches
var (
	gitBranchCache     string
	gitBranchCacheMu   sync.RWMutex
	gitBranchCacheTime time.Time
	gitBranchCacheTTL  = 30 * time.Second

	gitStatusCache     *GitStatusData
	gitStatusCacheMu   sync.RWMutex
	gitStatusCacheTime time.Time
	gitStatusCacheTTL  = 30 * time.Second
)

// GitStatusData holds git status information
type GitStatusData struct {
	Added       int
	Deleted     int
	Modified    int
	RemoteAhead int
	RemoteBehind int
}

// GitBranchCollector collects the current git branch
type GitBranchCollector struct {
	*BaseCollector
}

// NewGitBranchCollector creates a new git branch collector
func NewGitBranchCollector() *GitBranchCollector {
	return &GitBranchCollector{
		BaseCollector: NewBaseCollector(ContentGitBranch, 30*time.Second, false),
	}
}

// Collect returns the current git branch
func (c *GitBranchCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	return getGitBranchCached(statusInput.Cwd), nil
}

// GitStatusCollector collects git file status
type GitStatusCollector struct {
	*BaseCollector
}

// NewGitStatusCollector creates a new git status collector
func NewGitStatusCollector() *GitStatusCollector {
	return &GitStatusCollector{
		BaseCollector: NewBaseCollector(ContentGitStatus, 30*time.Second, false),
	}
}

// Collect returns the git file status
func (c *GitStatusCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	return getGitStatusCached(statusInput.Cwd), nil
}

// GitRemoteCollector collects git remote sync status
type GitRemoteCollector struct {
	*BaseCollector
}

// NewGitRemoteCollector creates a new git remote collector
func NewGitRemoteCollector() *GitRemoteCollector {
	return &GitRemoteCollector{
		BaseCollector: NewBaseCollector(ContentGitRemote, 30*time.Second, true),
	}
}

// Collect returns the git remote status
func (c *GitRemoteCollector) Collect(input interface{}, summary interface{}) (string, error) {
	statusInput, ok := input.(*StatusLineInput)
	if !ok {
		return "", fmt.Errorf("invalid input type")
	}
	return getGitRemoteStatusCached(statusInput.Cwd), nil
}

// getGitBranchCached returns cached git branch
func getGitBranchCached(cwd string) string {
	now := time.Now()

	gitBranchCacheMu.RLock()
	if gitBranchCache != "" && now.Sub(gitBranchCacheTime) < gitBranchCacheTTL {
		cached := gitBranchCache
		gitBranchCacheMu.RUnlock()
		return cached
	}
	gitBranchCacheMu.RUnlock()

	branch := getGitBranch(cwd)

	gitBranchCacheMu.Lock()
	gitBranchCache = branch
	gitBranchCacheTime = now
	gitBranchCacheMu.Unlock()

	return branch
}

// getGitStatusCached returns cached git status
func getGitStatusCached(cwd string) string {
	now := time.Now()

	gitStatusCacheMu.RLock()
	if gitStatusCache != nil && now.Sub(gitStatusCacheTime) < gitStatusCacheTTL {
		cached := *gitStatusCache
		gitStatusCacheMu.RUnlock()
		return formatGitStatus(cached.Added, cached.Deleted, cached.Modified)
	}
	gitStatusCacheMu.RUnlock()

	added, deleted, modified := getGitStatus(cwd)
	status := &GitStatusData{Added: added, Deleted: deleted, Modified: modified}

	gitStatusCacheMu.Lock()
	gitStatusCache = status
	gitStatusCacheTime = now
	gitStatusCacheMu.Unlock()

	return formatGitStatus(added, deleted, modified)
}

// formatGitStatus formats git status as a string
func formatGitStatus(added, deleted, modified int) string {
	var statusParts []string
	if added > 0 {
		statusParts = append(statusParts, fmt.Sprintf("+%d", added))
	}
	if modified > 0 {
		statusParts = append(statusParts, fmt.Sprintf("~%d", modified))
	}
	if deleted > 0 {
		statusParts = append(statusParts, fmt.Sprintf("-%d", deleted))
	}
	return strings.Join(statusParts, " ")
}

// getGitBranch reads the current git branch
func getGitBranch(cwd string) string {
	if cwd == "" {
		return ""
	}

	// Method 1: Try git symbolic-ref --short HEAD
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}

	// Method 2: Try git rev-parse --abbrev-ref HEAD
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = cwd
	output, err = cmd.Output()
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch == "" || branch == "HEAD" {
			cmd = exec.Command("git", "status", "--porcelain")
			cmd.Dir = cwd
			_, err = cmd.Output()
			if err == nil {
				cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "origin/HEAD")
				cmd.Dir = cwd
				output, err = cmd.Output()
				if err == nil {
					remoteBranch := strings.TrimSpace(string(output))
					if strings.HasPrefix(remoteBranch, "origin/") {
						return strings.TrimPrefix(remoteBranch, "origin/")
					}
				}
				return "(empty)"
			}
			return ""
		}
		return branch
	}

	return ""
}

// getGitStatus returns added, deleted, modified file counts
func getGitStatus(cwd string) (int, int, int) {
	if cwd == "" {
		return 0, 0, 0
	}

	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}

	lines := strings.Split(string(output), "\n")
	added, deleted, modified := 0, 0, 0

	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		xy := line[:2]
		x := xy[0]
		y := xy[1]

		if x == '?' && y == '?' {
			added++
			continue
		}

		switch x {
		case 'A':
			added++
		case 'M':
			modified++
		case 'D':
			deleted++
		}

		if x == ' ' {
			switch y {
			case 'M':
				modified++
			case 'D':
				deleted++
			}
		}
	}

	return added, deleted, modified
}

// getGitRemoteStatus returns the remote branch sync status
func getGitRemoteStatus(cwd string) string {
	if cwd == "" {
		return ""
	}

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	remoteBranch := strings.TrimSpace(string(output))
	if remoteBranch == "" || remoteBranch == "@{u}" {
		return ""
	}

	cmd = exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{u}")
	cmd.Dir = cwd
	output, err = cmd.Output()
	if err != nil {
		return ""
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) != 2 {
		return ""
	}

	ahead, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	behind, _ := strconv.Atoi(strings.TrimSpace(parts[1]))

	if ahead > 0 && behind > 0 {
		return fmt.Sprintf("🔄 ↑%d↓%d", ahead, behind)
	} else if ahead > 0 {
		return fmt.Sprintf("🔄 ↑%d", ahead)
	} else if behind > 0 {
		return fmt.Sprintf("🔄 ↓%d", behind)
	}

	return ""
}

// getGitRemoteStatusCached returns cached git remote status
func getGitRemoteStatusCached(cwd string) string {
	// Always fetch fresh data for git remote status
	// (don't share cache with git status to avoid conflicts)
	ahead, behind := getGitRemoteStatusRaw(cwd)
	return formatGitRemote(ahead, behind)
}

// formatGitRemote formats git remote status
func formatGitRemote(ahead, behind int) string {
	if ahead > 0 && behind > 0 {
		return fmt.Sprintf("🔄 ↑%d↓%d", ahead, behind)
	} else if ahead > 0 {
		return fmt.Sprintf("🔄 ↑%d", ahead)
	} else if behind > 0 {
		return fmt.Sprintf("🔄 ↓%d", behind)
	}
	return ""
}

// getGitRemoteStatusRaw returns raw ahead/behind counts
func getGitRemoteStatusRaw(cwd string) (ahead, behind int) {
	if cwd == "" {
		return 0, 0
	}

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	remoteBranch := strings.TrimSpace(string(output))
	if remoteBranch == "" || remoteBranch == "@{u}" {
		return 0, 0
	}

	cmd = exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{u}")
	cmd.Dir = cwd
	output, err = cmd.Output()
	if err != nil {
		return 0, 0
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) == 2 {
		ahead, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
		behind, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	}

	return ahead, behind
}
