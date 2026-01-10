package config

import (
	"os"
	"runtime"
)

// PlatformProvider abstracts platform-specific operations for testability
type PlatformProvider interface {
	// GetOS returns the operating system name ("windows", "darwin", "linux")
	GetOS() string

	// GetEnv returns the value of an environment variable
	GetEnv(key string) string

	// UserHomeDir returns the current user's home directory
	UserHomeDir() (string, error)
}

// OSPlatformProvider implements PlatformProvider using real OS calls
type OSPlatformProvider struct{}

func (OSPlatformProvider) GetOS() string {
	return runtime.GOOS
}

func (OSPlatformProvider) GetEnv(key string) string {
	return os.Getenv(key)
}

func (OSPlatformProvider) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

// DefaultPlatform is the default platform provider (can be overridden for tests)
var DefaultPlatform PlatformProvider = OSPlatformProvider{}
