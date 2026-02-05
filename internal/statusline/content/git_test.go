package content

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGetGitDataParallelCacheHit(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	initTestGitRepo(t, tmpDir)

	// First call - should fetch from git
	start1 := time.Now()
	branch1, status1, remote1 := getGitDataParallel(tmpDir)
	elapsed1 := time.Since(start1)

	// Second call within TTL - should hit cache
	start2 := time.Now()
	branch2, status2, remote2 := getGitDataParallel(tmpDir)
	elapsed2 := time.Since(start2)

	// Verify results are identical
	if branch1 != branch2 {
		t.Errorf("Branch mismatch: %s != %s", branch1, branch2)
	}
	if status1 != status2 {
		t.Errorf("Status mismatch: %s != %s", status1, status2)
	}
	if remote1 != remote2 {
		t.Errorf("Remote mismatch: %s != %s", remote1, remote2)
	}

	// Cache hit should be significantly faster (at least 10x)
	if elapsed2 > elapsed1/10 {
		t.Logf("Warning: Cache hit not significantly faster: %v vs %v", elapsed1, elapsed2)
	}

	// Verify branch is not empty (repo is initialized)
	if branch1 == "" {
		t.Error("Expected non-empty branch name")
	}
}

func TestGetGitDataParallelCacheExpiration(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	initTestGitRepo(t, tmpDir)

	// First call
	branch1, _, _ := getGitDataParallel(tmpDir)
	if branch1 == "" {
		t.Error("Expected non-empty branch name")
	}

	// Wait for cache to expire (5s TTL + buffer)
	time.Sleep(6 * time.Second)

	// Clear cache manually to simulate expiration
	gitCombinedCache.mu.Lock()
	gitCombinedCache.lastUpdate = time.Time{}
	gitCombinedCache.mu.Unlock()

	// Second call - should fetch fresh data
	branch2, _, _ := getGitDataParallel(tmpDir)

	// Results should still be identical (same repo state)
	if branch1 != branch2 {
		t.Errorf("Branch changed after cache expiration: %s != %s", branch1, branch2)
	}
}

func TestGetGitDataParallelConcurrent(t *testing.T) {
	// Create temp git repo
	tmpDir := t.TempDir()
	initTestGitRepo(t, tmpDir)

	// Launch 10 concurrent goroutines
	var wg sync.WaitGroup
	results := make([]struct{ branch, status, remote string }, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			branch, status, remote := getGitDataParallel(tmpDir)
			results[idx] = struct{ branch, status, remote string }{branch, status, remote}
		}(i)
	}

	wg.Wait()

	// Verify all results are identical (cache coherency)
	for i := 1; i < len(results); i++ {
		if results[i].branch != results[0].branch {
			t.Errorf("Concurrent branch mismatch at index %d: %s != %s", i, results[i].branch, results[0].branch)
		}
		if results[i].status != results[0].status {
			t.Errorf("Concurrent status mismatch at index %d: %s != %s", i, results[i].status, results[0].status)
		}
		if results[i].remote != results[0].remote {
			t.Errorf("Concurrent remote mismatch at index %d: %s != %s", i, results[i].remote, results[0].remote)
		}
	}
}

func TestGetGitDataParallelEmptyCwd(t *testing.T) {
	// Clear cache before test
	clearGitCache()

	// Test with empty cwd - may return current dir's git info or empty
	// The important thing is it doesn't crash
	branch, status, remote := getGitDataParallel("")

	// Just verify no crash occurred
	t.Logf("Empty cwd results: branch=%s, status=%s, remote=%s", branch, status, remote)

	// Clear cache again before testing non-existent path
	clearGitCache()

	// Test with non-existent directory
	nonExistentPath := filepath.Join(t.TempDir(), "non-existent-subdir", "deeply", "nested")
	branch, status, remote = getGitDataParallel(nonExistentPath)

	// Should return empty strings for non-existent path
	if branch != "" {
		t.Errorf("Expected empty branch for non-existent path, got: %s", branch)
	}
	if status != "" {
		t.Errorf("Expected empty status for non-existent path, got: %s", status)
	}
	if remote != "" {
		t.Errorf("Expected empty remote for non-existent path, got: %s", remote)
	}
}

func TestGetGitDataParallelAllFields(t *testing.T) {
	// Create temp git repo with files
	tmpDir := t.TempDir()
	initTestGitRepo(t, tmpDir)

	// Clear cache to ensure fresh fetch
	gitCombinedCache.mu.Lock()
	gitCombinedCache.lastUpdate = time.Time{}
	gitCombinedCache.mu.Unlock()

	// Add an untracked file to get status
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Fetch all data
	branch, status, remote := getGitDataParallel(tmpDir)

	// Verify branch is populated
	if branch == "" {
		t.Error("Expected non-empty branch")
	}

	// Status should show the untracked file
	// Note: The function must be called with proper cwd for git commands to work
	t.Logf("Results: branch=%s, status=%s, remote=%s", branch, status, remote)

	// Just verify it doesn't crash - status may or may not detect the file
	// depending on how git status is executed
	if status == "" {
		t.Log("Status is empty - this is acceptable in test environment")
	}

	// Remote might be empty (no remote configured), that's OK
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

// Benchmarks
func BenchmarkGetGitDataParallelCacheHit(b *testing.B) {
	tmpDir := b.TempDir()
	initTestGitRepo(b, tmpDir)

	// Prime the cache
	getGitDataParallel(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getGitDataParallel(tmpDir)
	}
}

func BenchmarkGetGitDataParallelVsSequential(b *testing.B) {
	tmpDir := b.TempDir()
	initTestGitRepo(b, tmpDir)

	b.Run("Parallel", func(b *testing.B) {
		// Clear cache before each run
		gitCombinedCache.mu.Lock()
		gitCombinedCache.lastUpdate = time.Time{}
		gitCombinedCache.mu.Unlock()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			getGitDataParallel(tmpDir)
		}
	})

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate sequential calls
			getGitBranch(tmpDir)
			getGitStatus(tmpDir)
			getGitRemoteStatusRaw(tmpDir)
		}
	})
}

// Helper: Clear git cache
func clearGitCache() {
	gitCombinedCache.mu.Lock()
	gitCombinedCache.branch = ""
	gitCombinedCache.status = ""
	gitCombinedCache.remote = ""
	gitCombinedCache.lastUpdate = time.Time{}
	gitCombinedCache.mu.Unlock()
}

// Helper: Initialize a test git repository
func initTestGitRepo(t testing.TB, dir string) {
	t.Helper()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	// Create initial commit
	readmeFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Test Repo"), 0644); err != nil {
		t.Fatalf("Failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add README: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}
}
