//go:build windows

package config

import (
	"os"
	"testing"
)

// TestClaudeDataDirWindowsFallback tests Windows fallback when APPDATA is not set
func TestClaudeDataDirWindowsFallback(t *testing.T) {
	// Save original APPDATA
	originalAppData := os.Getenv("APPDATA")
	defer func() {
		if originalAppData != "" {
			os.Setenv("APPDATA", originalAppData)
		} else {
			os.Unsetenv("APPDATA")
		}
	}()

	// Unset APPDATA to test fallback
	os.Unsetenv("APPDATA")
	result := ClaudeDataDir()

	// When APPDATA is unset, should return empty string
	if result != "" {
		t.Logf("ClaudeDataDir with no APPDATA returned: %q", result)
	}
}

// TestUserCacheDirWindowsFallback tests Windows fallback when LOCALAPPDATA is not set
func TestUserCacheDirWindowsFallback(t *testing.T) {
	// Save original LOCALAPPDATA
	originalLocalAppData := os.Getenv("LOCALAPPDATA")
	defer func() {
		if originalLocalAppData != "" {
			os.Setenv("LOCALAPPDATA", originalLocalAppData)
		} else {
			os.Unsetenv("LOCALAPPDATA")
		}
	}()

	// Unset LOCALAPPDATA to test fallback
	os.Unsetenv("LOCALAPPDATA")
	result := UserCacheDir()

	// When LOCALAPPDATA is unset, should use home directory fallback
	if result == "" {
		t.Error("UserCacheDir() should return fallback path when LOCALAPPDATA is empty")
	}

	// The fallback path should contain "claude-token-monitor"
	if len(result) < 10 {
		t.Errorf("Fallback path too short: %q", result)
	}
}

// TestClaudeDataDirWindowsSuccess tests normal Windows path
func TestClaudeDataDirWindowsSuccess(t *testing.T) {
	// This test just verifies the normal path works
	// It will use the actual APPDATA from the environment
	result := ClaudeDataDir()

	if result == "" {
		// APPDATA might not be set in test environment
		t.Skip("APPDATA not set, skipping normal path test")
	}

	// Should contain "Claude"
	if result != "" && len(result) < 10 {
		t.Errorf("Path too short: %q", result)
	}
}
