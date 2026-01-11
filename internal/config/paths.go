package config

import (
	"os"
	"path/filepath"
)

// ClaudeDataDir returns the Claude Code data directory for the current platform
func ClaudeDataDir() string {
	return ClaudeDataDirWithPlatform(DefaultPlatform)
}

// ClaudeDataDirWithPlatform allows injecting a custom platform provider for testing
func ClaudeDataDirWithPlatform(platform PlatformProvider) string {
	switch platform.GetOS() {
	case "windows":
		// %APPDATA%\Claude\
		appData := platform.GetEnv("APPDATA")
		if appData == "" {
			return ""
		}
		return filepath.Join(appData, "Claude")
	case "darwin":
		// ~/Library/Application Support/Claude/
		home, err := platform.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, "Library", "Application Support", "Claude")
	default: // linux, etc.
		// ~/.config/Claude/
		home, err := platform.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, ".config", "Claude")
	}
}

// ProjectsDir returns the directory containing project session files
func ProjectsDir() string {
	return ProjectsDirWithPlatform(DefaultPlatform)
}

// ProjectsDirWithPlatform allows injecting a custom platform provider for testing
func ProjectsDirWithPlatform(platform PlatformProvider) string {
	// Use ~/.claude/projects for all platforms (consistent with Claude Code)
	home, err := platform.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// UserCacheDir returns the application cache directory for history storage
func UserCacheDir() string {
	return UserCacheDirWithPlatform(DefaultPlatform)
}

// UserCacheDirWithPlatform allows injecting a custom platform provider for testing
func UserCacheDirWithPlatform(platform PlatformProvider) string {
	switch platform.GetOS() {
	case "windows":
		// %LOCALAPPDATA%\claude-token-monitor\
		localAppData := platform.GetEnv("LOCALAPPDATA")
		if localAppData == "" {
			home, _ := platform.UserHomeDir()
			return filepath.Join(home, ".claude-token-monitor")
		}
		return filepath.Join(localAppData, "claude-token-monitor")
	case "darwin":
		// ~/Library/Caches/claude-token-monitor/
		home, _ := platform.UserHomeDir()
		return filepath.Join(home, "Library", "Caches", "claude-token-monitor")
	default:
		// ~/.cache/claude-token-monitor/
		home, _ := platform.UserHomeDir()
		return filepath.Join(home, ".cache", "claude-token-monitor")
	}
}

// HistoryDBPath returns the path to the SQLite history database
func HistoryDBPath() string {
	cacheDir := UserCacheDir()
	_ = os.MkdirAll(cacheDir, 0755)
	return filepath.Join(cacheDir, "history.db")
}
