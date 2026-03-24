package content

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StubCommandRunner returns canned responses for unit tests.
// Key format: "git arg1 arg2 ..."
type StubCommandRunner struct {
	Outputs map[string][]byte
	Errors  map[string]error
}

func (s *StubCommandRunner) Run(dir, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if err, ok := s.Errors[key]; ok {
		return nil, err
	}
	if out, ok := s.Outputs[key]; ok {
		return out, nil
	}
	return nil, errors.New("stub: unknown command: " + key)
}

// resetGitCache clears the combined git cache.
func resetGitCache() {
	gitCombinedCache.mu.Lock()
	gitCombinedCache.branch = ""
	gitCombinedCache.status = ""
	gitCombinedCache.remote = ""
	gitCombinedCache.lastUpdate = time.Time{}
	gitCombinedCache.mu.Unlock()
}

// restoreDefaultRunner resets defaultCommandRunner to the real implementation.
func restoreDefaultRunner() {
	defaultCommandRunner = &RealCommandRunner{}
}

// --- getGitBranch tests ---

func TestGetGitBranch_SymbolicRef(t *testing.T) {
	// Arrange
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD": []byte("main\n"),
		},
	}

	// Act
	branch := getGitBranch("/project")

	// Assert
	if branch != "main" {
		t.Errorf("expected %q, got %q", "main", branch)
	}
}

func TestGetGitBranch_SymbolicRefEmpty(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":   []byte("\n"),
			"git rev-parse --abbrev-ref HEAD": []byte("develop\n"),
		},
	}

	branch := getGitBranch("/project")
	if branch != "develop" {
		t.Errorf("expected %q, got %q", "develop", branch)
	}
}

func TestGetGitBranch_FallsBackToRevParse(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("not on a branch"),
		},
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD": []byte("feature/test\n"),
		},
	}

	branch := getGitBranch("/project")
	if branch != "feature/test" {
		t.Errorf("expected %q, got %q", "feature/test", branch)
	}
}

func TestGetGitBranch_DetachedHead(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("detached"),
		},
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD": []byte("HEAD\n"),
			"git status --porcelain":          []byte(""),
		},
	}

	branch := getGitBranch("/project")
	if branch != "(empty)" {
		t.Errorf("expected %q, got %q", "(empty)", branch)
	}
}

func TestGetGitBranch_EmptyRepo(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("not a branch"),
		},
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD": []byte("HEAD\n"),
			"git status --porcelain":          []byte(""),
		},
	}

	branch := getGitBranch("/project")
	if branch != "(empty)" {
		t.Errorf("expected %q, got %q", "(empty)", branch)
	}
}

func TestGetGitBranch_OriginHeadFallback(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("detached"),
		},
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD":        []byte("HEAD\n"),
			"git status --porcelain":                 []byte(""),
			"git rev-parse --abbrev-ref origin/HEAD": []byte("origin/main\n"),
		},
	}

	branch := getGitBranch("/project")
	if branch != "main" {
		t.Errorf("expected %q from origin/HEAD fallback, got %q", "main", branch)
	}
}

func TestGetGitBranch_DetachedHeadNoGitStatus(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("detached"),
			"git status --porcelain":        errors.New("not a git repo"),
		},
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD": []byte("HEAD\n"),
		},
	}

	branch := getGitBranch("/project")
	if branch != "" {
		t.Errorf("expected empty, got %q", branch)
	}
}

func TestGetGitBranch_AllFail(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git symbolic-ref --short HEAD":   errors.New("fail"),
			"git rev-parse --abbrev-ref HEAD": errors.New("fail"),
		},
	}

	branch := getGitBranch("/project")
	if branch != "" {
		t.Errorf("expected empty, got %q", branch)
	}
}

func TestGetGitBranch_EmptyCwd(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{}

	branch := getGitBranch("")
	if branch != "" {
		t.Errorf("expected empty for empty cwd, got %q", branch)
	}
}

// --- getGitStatus tests ---

func TestGetGitStatus_EmptyCwd(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()

	added, deleted, modified := getGitStatus("")
	if added != 0 || deleted != 0 || modified != 0 {
		t.Errorf("expected 0,0,0, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_CommandFails(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git status --porcelain --untracked-files=all": errors.New("not a repo"),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 0 || deleted != 0 || modified != 0 {
		t.Errorf("expected 0,0,0, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_CleanRepo(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(""),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 0 || deleted != 0 || modified != 0 {
		t.Errorf("expected 0,0,0, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_UntrackedFiles(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(
				"?? new1.txt\n?? new2.txt\n",
			),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 2 || deleted != 0 || modified != 0 {
		t.Errorf("expected 2,0,0, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_StagedAddition(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(
				"A  added.txt\n",
			),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 1 || deleted != 0 || modified != 0 {
		t.Errorf("expected 1,0,0, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_StagedDeletion(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(
				"D  removed.txt\n",
			),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 0 || deleted != 1 || modified != 0 {
		t.Errorf("expected 0,1,0, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_StagedModification(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(
				"M  modified.txt\n",
			),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 0 || deleted != 0 || modified != 1 {
		t.Errorf("expected 0,0,1, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_WorktreeModification(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(
				" M modified.txt\n",
			),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 0 || deleted != 0 || modified != 1 {
		t.Errorf("expected 0,0,1, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_WorktreeDeletion(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(
				" D deleted.txt\n",
			),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 0 || deleted != 1 || modified != 0 {
		t.Errorf("expected 0,1,0, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_MixedStates(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte(
				"?? new.txt\nA  staged.txt\nM  mod.txt\n D gone.txt\n",
			),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 2 || deleted != 1 || modified != 1 {
		t.Errorf("expected 2,1,1, got %d,%d,%d", added, deleted, modified)
	}
}

func TestGetGitStatus_EmptyLines(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	// Empty lines should be skipped (len < 2)
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git status --porcelain --untracked-files=all": []byte("\n\n\n"),
		},
	}

	added, deleted, modified := getGitStatus("/project")
	if added != 0 || deleted != 0 || modified != 0 {
		t.Errorf("expected 0,0,0, got %d,%d,%d", added, deleted, modified)
	}
}

// --- getGitRemoteStatus tests ---

func TestGetGitRemoteStatus_EmptyCwd(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()

	result := getGitRemoteStatus("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestGetGitRemoteStatus_NoUpstream(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": errors.New("no upstream"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestGetGitRemoteStatus_Ahead(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("3\t0\n"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "🔄 ↑3" {
		t.Errorf("expected %q, got %q", "🔄 ↑3", result)
	}
}

func TestGetGitRemoteStatus_Behind(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("0\t5\n"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "🔄 ↓5" {
		t.Errorf("expected %q, got %q", "🔄 ↓5", result)
	}
}

func TestGetGitRemoteStatus_Diverged(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("2\t3\n"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "🔄 ↑2↓3" {
		t.Errorf("expected %q, got %q", "🔄 ↑2↓3", result)
	}
}

func TestGetGitRemoteStatus_InSync(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("0\t0\n"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "" {
		t.Errorf("expected empty for in-sync, got %q", result)
	}
}

func TestGetGitRemoteStatus_EmptyRemoteBranch(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("@{u}\n"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestGetGitRemoteStatus_MalformedCount(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("bad\tformat\n"),
		},
	}

	result := getGitRemoteStatus("/project")
	// Atoi fails, so ahead=0, behind=0 → empty
	if result != "" {
		t.Errorf("expected empty for malformed, got %q", result)
	}
}

func TestGetGitRemoteStatus_WrongPartCount(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("only_one_part\n"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestGetGitRemoteStatus_RevListFails(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
		},
		Errors: map[string]error{
			"git rev-list --left-right --count HEAD...@{u}": errors.New("fatal"),
		},
	}

	result := getGitRemoteStatus("/project")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// --- getGitRemoteStatusRaw tests ---

func TestGetGitRemoteStatusRaw_EmptyCwd(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()

	ahead, behind := getGitRemoteStatusRaw("")
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0,0, got %d,%d", ahead, behind)
	}
}

func TestGetGitRemoteStatusRaw_NoUpstream(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": errors.New("no upstream"),
		},
	}

	ahead, behind := getGitRemoteStatusRaw("/project")
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0,0, got %d,%d", ahead, behind)
	}
}

func TestGetGitRemoteStatusRaw_Ahead(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("4\t1\n"),
		},
	}

	ahead, behind := getGitRemoteStatusRaw("/project")
	if ahead != 4 || behind != 1 {
		t.Errorf("expected 4,1, got %d,%d", ahead, behind)
	}
}

func TestGetGitRemoteStatusRaw_InSync(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("0\t0\n"),
		},
	}

	ahead, behind := getGitRemoteStatusRaw("/project")
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0,0, got %d,%d", ahead, behind)
	}
}

func TestGetGitRemoteStatusRaw_EmptyRemoteBranch(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte(""),
		},
	}

	ahead, behind := getGitRemoteStatusRaw("/project")
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0,0, got %d,%d", ahead, behind)
	}
}

func TestGetGitRemoteStatusRaw_AtUReturned(t *testing.T) {
	// When git returns literal "@{u}" (not resolved)
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("@{u}\n"),
		},
	}

	ahead, behind := getGitRemoteStatusRaw("/project")
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0,0, got %d,%d", ahead, behind)
	}
}

func TestGetGitRemoteStatusRaw_RevListFails(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
		},
		Errors: map[string]error{
			"git rev-list --left-right --count HEAD...@{u}": errors.New("fatal"),
		},
	}

	ahead, behind := getGitRemoteStatusRaw("/project")
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0,0, got %d,%d", ahead, behind)
	}
}

func TestGetGitRemoteStatusRaw_WrongPartCount(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("only_one\n"),
		},
	}

	ahead, behind := getGitRemoteStatusRaw("/project")
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0,0, got %d,%d", ahead, behind)
	}
}

// --- Cached wrapper tests ---

func TestGetGitBranchCached(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                        []byte("main\n"),
			"git status --porcelain --untracked-files=all":         []byte(""),
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte(""),
		},
	}

	branch := getGitBranchCached("/project")
	if branch != "main" {
		t.Errorf("expected %q, got %q", "main", branch)
	}
}

func TestGetGitStatusCached(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                        []byte("main\n"),
			"git status --porcelain --untracked-files=all":         []byte("?? file.txt\n"),
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte(""),
		},
	}

	status := getGitStatusCached("/project")
	if status != "+1" {
		t.Errorf("expected %q, got %q", "+1", status)
	}
}

func TestGetGitRemoteStatusCached(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                        []byte("main\n"),
			"git status --porcelain --untracked-files=all":         []byte(""),
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("2\t0\n"),
		},
	}

	remote := getGitRemoteStatusCached("/project")
	if remote != "🔄 ↑2" {
		t.Errorf("expected %q, got %q", "🔄 ↑2", remote)
	}
}

// --- Collector tests ---

func TestGitBranchCollector(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                        []byte("main\n"),
			"git status --porcelain --untracked-files=all":         []byte(""),
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte(""),
		},
	}

	collector := NewGitBranchCollector()

	if collector.Type() != ContentGitBranch {
		t.Errorf("expected type %q, got %q", ContentGitBranch, collector.Type())
	}

	// Valid input
	input := &StatusLineInput{Cwd: "/project"}
	result, err := collector.Collect(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "main" {
		t.Errorf("expected %q, got %q", "main", result)
	}

	// Invalid input type
	_, err = collector.Collect("wrong", nil)
	if err == nil {
		t.Error("expected error for invalid input type")
	}
}

func TestGitStatusCollector(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                        []byte("main\n"),
			"git status --porcelain --untracked-files=all":         []byte("?? file.txt\n"),
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte(""),
		},
	}

	collector := NewGitStatusCollector()

	if collector.Type() != ContentGitStatus {
		t.Errorf("expected type %q, got %q", ContentGitStatus, collector.Type())
	}

	input := &StatusLineInput{Cwd: "/project"}
	result, err := collector.Collect(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "+1" {
		t.Errorf("expected %q, got %q", "+1", result)
	}

	// Invalid input type
	_, err = collector.Collect(42, nil)
	if err == nil {
		t.Error("expected error for invalid input type")
	}
}

func TestGitRemoteCollector(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Errors: map[string]error{
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": errors.New("no upstream"),
		},
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                []byte("main\n"),
			"git status --porcelain --untracked-files=all": []byte(""),
		},
	}

	collector := NewGitRemoteCollector()

	if collector.Type() != ContentGitRemote {
		t.Errorf("expected type %q, got %q", ContentGitRemote, collector.Type())
	}
	if !collector.Optional() {
		t.Error("expected remote collector to be optional")
	}

	// No upstream → empty but no error
	input := &StatusLineInput{Cwd: "/project"}
	result, err := collector.Collect(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty remote with no upstream, got %q", result)
	}

	// Invalid input type
	_, err = collector.Collect([]string{}, nil)
	if err == nil {
		t.Error("expected error for invalid input type")
	}
}

// --- getGitDataParallel cache tests ---

func TestGetGitDataParallel_CacheHit(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()

	// Prime cache with stub
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                        []byte("main\n"),
			"git status --porcelain --untracked-files=all":         []byte("?? f.txt\n"),
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("1\t0\n"),
		},
	}

	branch1, status1, remote1 := getGitDataParallel("/project")
	if branch1 != "main" {
		t.Errorf("expected branch %q, got %q", "main", branch1)
	}

	// Replace runner with one that would return different results
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD": []byte("other\n"),
		},
	}

	// Should still return cached "main"
	branch2, status2, remote2 := getGitDataParallel("/project")
	if branch2 != branch1 {
		t.Errorf("cache miss: expected branch %q, got %q", branch1, branch2)
	}
	if status2 != status1 {
		t.Errorf("cache miss: expected status %q, got %q", status1, status2)
	}
	if remote2 != remote1 {
		t.Errorf("cache miss: expected remote %q, got %q", remote1, remote2)
	}
}

func TestGetGitDataParallel_Concurrent(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{
		Outputs: map[string][]byte{
			"git symbolic-ref --short HEAD":                        []byte("main\n"),
			"git status --porcelain --untracked-files=all":         []byte(""),
			"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/main\n"),
			"git rev-list --left-right --count HEAD...@{u}":        []byte("0\t0\n"),
		},
	}

	var wg sync.WaitGroup
	results := make([]struct{ branch, status, remote string }, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b, s, r := getGitDataParallel("/project")
			results[idx] = struct{ branch, status, remote string }{b, s, r}
		}(i)
	}

	wg.Wait()

	for i := 1; i < len(results); i++ {
		if results[i].branch != results[0].branch {
			t.Errorf("concurrent mismatch at %d: branch %q != %q", i, results[i].branch, results[0].branch)
		}
	}
}

func TestGetGitDataParallel_EmptyCwd(t *testing.T) {
	defer restoreDefaultRunner()
	resetGitCache()
	defaultCommandRunner = &StubCommandRunner{}

	branch, status, remote := getGitDataParallel("")
	if branch != "" || status != "" || remote != "" {
		t.Errorf("expected all empty, got branch=%q status=%q remote=%q", branch, status, remote)
	}
}

// --- Pure function tests (already existed, kept) ---

func TestTruncateBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected string
	}{
		{"empty branch", "", ""},
		{"main branch", "main", "main"},
		{"develop branch", "develop", "develop"},
		{"exactly 25 chars", "feature/25-characters-br!", "feature/25-characters-br!"},
		{"26 chars truncated", "feature/26-characters-br!!", "feature/26-characters-.."},
		{"long feature branch", "feature/young1lin/refactor-branch-cell", "feature/young1lin/refa.."},
		{"long fix branch", "fix/some-very-long-descriptive-bugfix-branch-name", "fix/some-very-long-des.."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateBranch(tt.branch)
			if got != tt.expected {
				t.Errorf("TruncateBranch(%q) = %q, want %q", tt.branch, got, tt.expected)
			}
		})
	}
}

func TestFormatGitStatus(t *testing.T) {
	tests := []struct {
		name     string
		added    int
		deleted  int
		modified int
		want     string
	}{
		{"no changes", 0, 0, 0, ""},
		{"only added", 5, 0, 0, "+5"},
		{"only modified", 0, 0, 3, "~3"},
		{"only deleted", 0, 2, 0, "-2"},
		{"all changes", 5, 2, 3, "+5 ~3 -2"},
		{"added and modified", 10, 0, 5, "+10 ~5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatGitStatus(tt.added, tt.deleted, tt.modified)
			if got != tt.want {
				t.Errorf("formatGitStatus(%d, %d, %d) = %q, want %q",
					tt.added, tt.deleted, tt.modified, got, tt.want)
			}
		})
	}
}

func TestFormatGitRemote(t *testing.T) {
	tests := []struct {
		name   string
		ahead  int
		behind int
		want   string
	}{
		{"in sync", 0, 0, ""},
		{"ahead only", 3, 0, "🔄 ↑3"},
		{"behind only", 0, 5, "🔄 ↓5"},
		{"diverged", 2, 3, "🔄 ↑2↓3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatGitRemote(tt.ahead, tt.behind)
			if got != tt.want {
				t.Errorf("formatGitRemote(%d, %d) = %q, want %q",
					tt.ahead, tt.behind, got, tt.want)
			}
		})
	}
}

// --- RealCommandRunner.Run integration test ---

func TestRealCommandRunner_EchoCommand(t *testing.T) {
	runner := &RealCommandRunner{}
	out, err := runner.Run("", "echo", "hello")
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello")
}

func TestRealCommandRunner_WithDir(t *testing.T) {
	runner := &RealCommandRunner{}
	out, err := runner.Run(t.TempDir(), "echo", "test")
	require.NoError(t, err)
	assert.Contains(t, string(out), "test")
}

func TestRealCommandRunner_NonexistentCommand(t *testing.T) {
	runner := &RealCommandRunner{}
	_, err := runner.Run("", "nonexistent_command_xyz_123")
	assert.Error(t, err)
}

func TestRealCommandRunner_EmptyDir(t *testing.T) {
	// dir="" means cmd.Dir is not set, uses current working directory
	runner := &RealCommandRunner{}
	out, err := runner.Run("", "echo", "no-dir")
	require.NoError(t, err)
	assert.Contains(t, string(out), "no-dir")
}

// --- Benchmarks (still use real git, that's fine for benchmarks) ---

func BenchmarkGetGitDataParallelCacheHit(b *testing.B) {
	restoreDefaultRunner()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getGitDataParallel("/project")
	}
}
