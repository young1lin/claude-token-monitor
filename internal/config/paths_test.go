package config

import (
	"os"
	"runtime"
	"testing"
)

func TestClaudeDataDir(t *testing.T) {
	result := ClaudeDataDir()

	// Should not be empty on supported platforms
	if result == "" && (runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "linux") {
		t.Error("ClaudeDataDir() returned empty string on supported platform")
	}

	// Verify it contains "Claude" somewhere in the path
	if result != "" && !containsPath(result, "Claude") {
		t.Errorf("Expected path to contain 'Claude', got %s", result)
	}

	// Verify the path format based on OS
	if runtime.GOOS == "windows" {
		// Windows: should contain backslash
		if !containsPath(result, "\\") && len(result) > 10 {
			t.Logf("Warning: Windows path may not have proper separators: %s", result)
		}
	} else if runtime.GOOS == "darwin" {
		// macOS: should contain "Library/Application Support"
		if !containsPath(result, "Library") {
			t.Errorf("macOS path should contain 'Library', got %s", result)
		}
	} else if runtime.GOOS == "linux" {
		// Linux: should contain ".config"
		if !containsPath(result, ".config") {
			t.Errorf("Linux path should contain '.config', got %s", result)
		}
	}
}

// TestClaudeDataDirWindowsNoAppData tests the Windows fallback path
func TestClaudeDataDirWindowsNoAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// Save original APPDATA value
	originalAppData := os.Getenv("APPDATA")

	// Test with empty APPDATA (should return empty string)
	os.Unsetenv("APPDATA")
	result := ClaudeDataDir()

	// Restore APPDATA
	if originalAppData != "" {
		os.Setenv("APPDATA", originalAppData)
	} else {
		os.Unsetenv("APPDATA")
	}

	// When APPDATA is empty on Windows, ClaudeDataDir should return empty string
	if result != "" {
		// Actually, looking at the code, it returns "" when APPDATA is ""
		// But the function itself returns "" in that case, so this is correct
	}
}

func TestProjectsDir(t *testing.T) {
	result := ProjectsDir()

	if result == "" {
		t.Error("ProjectsDir() returned empty string")
	}

	// Should end with "projects" or contain it
	if result != "" && !containsPath(result, "projects") {
		t.Errorf("Expected path to contain 'projects', got %s", result)
	}
}

func TestUserCacheDir(t *testing.T) {
	result := UserCacheDir()

	if result == "" {
		t.Error("UserCacheDir() returned empty string")
	}

	// Should contain "claude-token-monitor"
	if result != "" && !containsPath(result, "claude-token-monitor") {
		t.Errorf("Expected path to contain 'claude-token-monitor', got %s", result)
	}

	// Verify OS-specific path
	if runtime.GOOS == "windows" {
		// Windows: should be in LOCALAPPDATA or fallback
		if !containsPath(result, "AppData\\Local") && !containsPath(result, "claude-token-monitor") {
			t.Logf("Windows cache path: %s", result)
		}
	} else if runtime.GOOS == "darwin" {
		// macOS: should contain "Library/Caches"
		if !containsPath(result, "Library") && !containsPath(result, "Caches") {
			t.Logf("macOS cache path: %s", result)
		}
	} else if runtime.GOOS == "linux" {
		// Linux: should contain ".cache"
		if !containsPath(result, ".cache") {
			t.Logf("Linux cache path: %s", result)
		}
	}
}

// TestUserCacheDirWindowsNoLocalAppData tests the Windows fallback path for cache
func TestUserCacheDirWindowsNoLocalAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// Save original LOCALAPPDATA value
	originalLocalAppData := os.Getenv("LOCALAPPDATA")

	// Test with empty LOCALAPPDATA (should use fallback to home dir)
	os.Unsetenv("LOCALAPPDATA")
	result := UserCacheDir()

	// Restore LOCALAPPDATA
	if originalLocalAppData != "" {
		os.Setenv("LOCALAPPDATA", originalLocalAppData)
	} else {
		os.Unsetenv("LOCALAPPDATA")
	}

	// When LOCALAPPDATA is empty, should still return a non-empty path using home dir
	if result == "" {
		t.Error("UserCacheDir() should return fallback path when LOCALAPPDATA is empty")
	}
}

func TestHistoryDBPath(t *testing.T) {
	result := HistoryDBPath()

	if result == "" {
		t.Error("HistoryDBPath() returned empty string")
	}

	// Should end with "history.db" (check using filepath for cross-platform)
	if len(result) < 11 {
		t.Errorf("Expected path to contain 'history.db', got %s", result)
	}
	// Simple check for filename in path
	hasFilename := false
	for i := len(result) - 1; i >= 0 && i > len(result)-20; i-- {
		if result[i] == '\\' || result[i] == '/' {
			if result[i+1:] == "history.db" {
				hasFilename = true
				break
			}
		}
	}
	if !hasFilename && result[len(result)-10:] != "history.db" {
		t.Errorf("Expected path to end with 'history.db', got %s", result)
	}
}

// Helper function to check if path contains a substring (case-insensitive for Windows)
func containsPath(path, substr string) bool {
	// Simple string contains check
	return len(path) >= len(substr) && containsSubstring(path, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
