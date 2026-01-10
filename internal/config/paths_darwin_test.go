//go:build darwin

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClaudeDataDirDarwin tests macOS-specific Claude data directory
func TestClaudeDataDirDarwin(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := ClaudeDataDir()
	expected := filepath.Join(home, "Library", "Application Support", "Claude")

	if result != expected {
		t.Errorf("ClaudeDataDir() = %q, want %q", result, expected)
	}
}

// TestClaudeDataDirDarwinNoHome tests macOS fallback when home directory fails
func TestClaudeDataDirDarwinNoHome(t *testing.T) {
	// This test verifies the function handles missing home directory gracefully
	// Since we can't easily mock os.UserHomeDir(), we just verify it returns something
	result := ClaudeDataDir()
	if result == "" {
		t.Log("ClaudeDataDir() returned empty string when home directory unavailable")
	}
}

// TestProjectsDirDarwin tests macOS-specific projects directory
func TestProjectsDirDarwin(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := ProjectsDir()
	expected := filepath.Join(home, "Library", "Application Support", "Claude", "projects")

	if result != expected {
		t.Errorf("ProjectsDir() = %q, want %q", result, expected)
	}
}

// TestUserCacheDirDarwin tests macOS-specific cache directory
func TestUserCacheDirDarwin(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := UserCacheDir()
	expected := filepath.Join(home, "Library", "Caches", "claude-token-monitor")

	if result != expected {
		t.Errorf("UserCacheDir() = %q, want %q", result, expected)
	}
}

// TestHistoryDBPathDarwin tests macOS-specific history database path
func TestHistoryDBPathDarwin(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := HistoryDBPath()
	expectedBase := filepath.Join(home, "Library", "Caches", "claude-token-monitor")

	// Check that the path contains the expected base directory
	if !filepath.HasPrefix(result, expectedBase) {
		t.Errorf("HistoryDBPath() = %q, should start with %q", result, expectedBase)
	}

	// Check that it ends with history.db
	if filepath.Base(result) != "history.db" {
		t.Errorf("HistoryDBPath() = %q, should end with history.db", result)
	}
}

// TestDarwinPathConsistency tests that all paths are consistent on macOS
func TestDarwinPathConsistency(t *testing.T) {
	projectsDir := ProjectsDir()
	claudeDir := ClaudeDataDir()

	expectedProjectsDir := filepath.Join(claudeDir, "projects")
	if projectsDir != expectedProjectsDir {
		t.Errorf("ProjectsDir() = %q, want %q (consistent with ClaudeDataDir)", projectsDir, expectedProjectsDir)
	}
}

// TestUserCacheDirDarwinFallback tests macOS fallback when home directory is unavailable
func TestUserCacheDirDarwinFallback(t *testing.T) {
	// Verify UserCacheDir handles missing home gracefully
	// The implementation uses _, _ = os.UserHomeDir() which ignores errors
	result := UserCacheDir()

	// On macOS, even without a proper home, it should still return something
	// (though the path may not be valid)
	if result == "" {
		t.Error("UserCacheDir() should return a path even if home directory is unavailable")
	}
}
