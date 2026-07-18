package content

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Git caches
var (
	// Combined cache for parallel git operations (replaces individual caches)
	gitCombinedCache struct {
		branch     string
		status     string
		remote     string
		worktree   string
		lastUpdate time.Time
		mu         sync.RWMutex
	}
	gitCombinedCacheTTL = 5 * time.Second
)

// GitStatusData holds git status information
type GitStatusData struct {
	Added        int
	Deleted      int
	Modified     int
	RemoteAhead  int
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
func (c *GitBranchCollector) Collect(env *Env) (string, error) {
	statusInput := env.Input
	return getGitBranchCached(statusInput.Cwd, env.OS.IsWindows, env.Now), nil
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
func (c *GitStatusCollector) Collect(env *Env) (string, error) {
	statusInput := env.Input
	return getGitStatusCached(statusInput.Cwd, env.OS.IsWindows, env.Now), nil
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
func (c *GitRemoteCollector) Collect(env *Env) (string, error) {
	statusInput := env.Input
	return getGitRemoteStatusCached(statusInput.Cwd, env.OS.IsWindows, env.Now), nil
}

// GitWorktreeCollector reports whether the cwd is inside a linked git worktree.
type GitWorktreeCollector struct {
	*BaseCollector
}

// NewGitWorktreeCollector creates a new git worktree collector
func NewGitWorktreeCollector() *GitWorktreeCollector {
	return &GitWorktreeCollector{
		BaseCollector: NewBaseCollector(ContentGitWorktree, 30*time.Second, true),
	}
}

// Collect returns "1" when the cwd is a linked worktree, otherwise "".
func (c *GitWorktreeCollector) Collect(env *Env) (string, error) {
	statusInput := env.Input
	return getGitWorktreeCached(statusInput.Cwd, env.OS.IsWindows, env.Now), nil
}

// getGitDataParallel fetches all git data (branch, status, remote) in parallel.
// This is the main optimization - instead of calling each git command sequentially,
// we run them concurrently and wait for all to complete. The caller threads
// env.Now so the cache TTL check uses the same snapshot the rest of the
// statusline render sees (pinnable in tests via the nowFn seam BuildEnv reads).
func getGitDataParallel(cwd string, isWindows bool, now time.Time) (branch, status, remote, worktree string) {
	// Check combined cache first
	gitCombinedCache.mu.RLock()
	if gitCombinedCache.branch != "" && now.Sub(gitCombinedCache.lastUpdate) < gitCombinedCacheTTL {
		branch = gitCombinedCache.branch
		status = gitCombinedCache.status
		remote = gitCombinedCache.remote
		worktree = gitCombinedCache.worktree
		gitCombinedCache.mu.RUnlock()
		return
	}
	gitCombinedCache.mu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(4)

	// Note: Direct assignment to named return values is safe here because:
	// 1. Named returns are allocated before goroutines spawn
	// 2. All assignments complete before wg.Wait() returns
	// 3. No loop variables are captured (cwd is immutable parameter)

	// Fetch branch in parallel
	go func() {
		defer wg.Done()
		branch = getGitBranch(cwd)
	}()

	// Fetch status in parallel
	go func() {
		defer wg.Done()
		added, deleted, modified := getGitStatus(cwd)
		status = formatGitStatus(added, deleted, modified)
	}()

	// Fetch remote in parallel
	go func() {
		defer wg.Done()
		ahead, behind := getGitRemoteStatusRaw(cwd)
		remote = formatGitRemote(ahead, behind)
	}()

	// Detect linked worktree in parallel
	go func() {
		defer wg.Done()
		if isLinkedWorktree(cwd, isWindows) {
			worktree = "1"
		}
	}()

	wg.Wait()

	// Update combined cache
	gitCombinedCache.mu.Lock()
	gitCombinedCache.branch = branch
	gitCombinedCache.status = status
	gitCombinedCache.remote = remote
	gitCombinedCache.worktree = worktree
	gitCombinedCache.lastUpdate = now
	gitCombinedCache.mu.Unlock()

	return
}

// getGitBranchCached returns cached git branch
func getGitBranchCached(cwd string, isWindows bool, now time.Time) string {
	branch, _, _, _ := getGitDataParallel(cwd, isWindows, now)
	return branch
}

// getGitStatusCached returns cached git status
func getGitStatusCached(cwd string, isWindows bool, now time.Time) string {
	_, status, _, _ := getGitDataParallel(cwd, isWindows, now)
	return status
}

// getGitRemoteStatusCached returns cached git remote status
func getGitRemoteStatusCached(cwd string, isWindows bool, now time.Time) string {
	_, _, remote, _ := getGitDataParallel(cwd, isWindows, now)
	return remote
}

// getGitWorktreeCached returns "1" when cwd is inside a linked git worktree,
// or "" for the main checkout / non-repo.
func getGitWorktreeCached(cwd string, isWindows bool, now time.Time) string {
	_, _, _, worktree := getGitDataParallel(cwd, isWindows, now)
	return worktree
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

// TruncateBranch limits branch name display length to 32 runes. Kept aligned
// with getProjectName in folder.go so the two cells share the same visual
// budget. Uses rune slicing for proper Unicode handling.
func TruncateBranch(branch string) string {
	runes := []rune(branch)
	if len(runes) > 32 {
		return string(runes[:29]) + ".."
	}
	return branch
}

// getGitBranch reads the current git branch using defaultCommandRunner.
func getGitBranch(cwd string) string {
	if cwd == "" {
		return ""
	}

	// Method 1: Try git symbolic-ref --short HEAD
	output, err := defaultCommandRunner.Run(cwd, "git", "symbolic-ref", "--short", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch != "" && branch != "HEAD" {
			return branch
		}
	}

	// Method 2: Try git rev-parse --abbrev-ref HEAD
	output, err = defaultCommandRunner.Run(cwd, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(string(output))
		if branch == "" || branch == "HEAD" {
			_, err = defaultCommandRunner.Run(cwd, "git", "status", "--porcelain")
			if err == nil {
				output, err = defaultCommandRunner.Run(cwd, "git", "rev-parse", "--abbrev-ref", "origin/HEAD")
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

// getGitStatus returns added, deleted, modified file counts.
func getGitStatus(cwd string) (int, int, int) {
	if cwd == "" {
		return 0, 0, 0
	}

	output, err := defaultCommandRunner.Run(cwd, "git", "status", "--porcelain", "--untracked-files=all")
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

// getGitRemoteStatusRaw returns raw ahead/behind counts.
func getGitRemoteStatusRaw(cwd string) (ahead, behind int) {
	if cwd == "" {
		return 0, 0
	}

	output, err := defaultCommandRunner.Run(cwd, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return 0, 0
	}

	remoteBranch := strings.TrimSpace(string(output))
	if remoteBranch == "" || remoteBranch == "@{u}" {
		return 0, 0
	}

	output, err = defaultCommandRunner.Run(cwd, "git", "rev-list", "--left-right", "--count", "HEAD...@{u}")
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

// isLinkedWorktree reports whether cwd is inside a linked git worktree (as
// opposed to the main checkout). A single `git rev-parse --git-dir
// --git-common-dir` returns two lines: a linked worktree points its git-dir at
// .git/worktrees/<name> while the common-dir still points at the main
// repository, so the two resolve to different directories.
//
// The two lines CANNOT be compared as raw strings: git reports --git-dir as an
// absolute path but --git-common-dir relative to cwd whenever cwd is a
// subdirectory of the main checkout (e.g. git-dir "/repo/.git" vs common-dir
// "../.git"). Those denote the SAME directory, so we must resolve both to a
// canonical absolute path (relative ones against cwd) before comparing.
func isLinkedWorktree(cwd string, isWindows bool) bool {
	if cwd == "" {
		return false
	}

	output, err := defaultCommandRunner.Run(cwd, "git", "rev-parse", "--git-dir", "--git-common-dir")
	if err != nil {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return false
	}

	gitDir := resolveGitPath(cwd, strings.TrimSpace(lines[0]))
	commonDir := resolveGitPath(cwd, strings.TrimSpace(lines[1]))
	return !pathsEqual(gitDir, commonDir, isWindows)
}

// resolveGitPath turns a path emitted by `git rev-parse` into a cleaned,
// absolute path. Git uses forward slashes on every platform and may return a
// path relative to cwd, so we normalise the separators and anchor relative
// paths at cwd before cleaning.
func resolveGitPath(cwd, p string) string {
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	return filepath.Clean(p)
}

// pathsEqual compares two cleaned paths, honouring the case-insensitivity of
// Windows filesystems so a drive-letter or casing difference is not mistaken
// for a separate directory.
func pathsEqual(a, b string, isWindows bool) bool {
	if isWindows {
		return strings.EqualFold(a, b)
	}
	return a == b
}
