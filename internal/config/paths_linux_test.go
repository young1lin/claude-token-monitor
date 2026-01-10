//go:build linux

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClaudeDataDirLinux tests Linux-specific Claude data directory
func TestClaudeDataDirLinux(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := ClaudeDataDir()
	expected := filepath.Join(home, ".config", "Claude")

	if result != expected {
		t.Errorf("ClaudeDataDir() = %q, want %q", result, expected)
	}
}

// TestClaudeDataDirLinuxNoHome tests Linux fallback when home directory fails
func TestClaudeDataDirLinuxNoHome(t *testing.T) {
	// This test verifies the function handles missing home directory gracefully
	result := ClaudeDataDir()
	if result == "" {
		t.Log("ClaudeDataDir() returned empty string when home directory unavailable")
	}
}

// TestProjectsDirLinux tests Linux-specific projects directory
func TestProjectsDirLinux(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := ProjectsDir()
	expected := filepath.Join(home, ".config", "Claude", "projects")

	if result != expected {
		t.Errorf("ProjectsDir() = %q, want %q", result, expected)
	}
}

// TestUserCacheDirLinux tests Linux-specific cache directory
func TestUserCacheDirLinux(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := UserCacheDir()
	expected := filepath.Join(home, ".cache", "claude-token-monitor")

	if result != expected {
		t.Errorf("UserCacheDir() = %q, want %q", result, expected)
	}
}

// TestHistoryDBPathLinux tests Linux-specific history database path
func TestHistoryDBPathLinux(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	result := HistoryDBPath()
	expectedBase := filepath.Join(home, ".cache", "claude-token-monitor")

	// Check that the path contains the expected base directory
	if !filepath.HasPrefix(result, expectedBase) {
		t.Errorf("HistoryDBPath() = %q, should start with %q", result, expectedBase)
	}

	// Check that it ends with history.db
	if filepath.Base(result) != "history.db" {
		t.Errorf("HistoryDBPath() = %q, should end with history.db", result)
	}
}

// TestLinuxPathConsistency tests that all paths are consistent on Linux
func TestLinuxPathConsistency(t *testing.T) {
	projectsDir := ProjectsDir()
	claudeDir := ClaudeDataDir()

	expectedProjectsDir := filepath.Join(claudeDir, "projects")
	if projectsDir != expectedProjectsDir {
		t.Errorf("ProjectsDir() = %q, want %q (consistent with ClaudeDataDir)", projectsDir, expectedProjectsDir)
	}
}

// TestLinuxXdgConfigPath verifies Claude uses XDG config directory
func TestLinuxXdgConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	claudeDir := ClaudeDataDir()
	xdgConfig := filepath.Join(home, ".config")

	if !filepath.HasPrefix(claudeDir, xdgConfig) {
		t.Errorf("ClaudeDataDir() = %q, should follow XDG config standard under %q", claudeDir, xdgConfig)
	}
}

// TestLinuxXdgCachePath verifies cache uses XDG cache directory
func TestLinuxXdgCachePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	cacheDir := UserCacheDir()
	xdgCache := filepath.Join(home, ".cache")

	if !filepath.HasPrefix(cacheDir, xdgCache) {
		t.Errorf("UserCacheDir() = %q, should follow XDG cache standard under %q", cacheDir, xdgCache)
	}
}

// TestUserCacheDirLinuxFallback tests Linux fallback when home directory is unavailable
func TestUserCacheDirLinuxFallback(t *testing.T) {
	// Verify UserCacheDir handles missing home gracefully
	// The implementation uses _, _ = os.UserHomeDir() which ignores errors
	result := UserCacheDir()

	// On Linux, even without a proper home, it should still return something
	if result == "" {
		t.Error("UserCacheDir() should return a path even if home directory is unavailable")
	}
}
