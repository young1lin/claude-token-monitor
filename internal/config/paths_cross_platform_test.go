package config

import (
	"errors"
	"path/filepath"
	"testing"
)

// MockPlatformProvider is a test double for PlatformProvider
type MockPlatformProvider struct {
	OS           string
	EnvVars      map[string]string
	HomeDirPath  string
	HomeDirError error
}

func (m *MockPlatformProvider) GetOS() string {
	return m.OS
}

func (m *MockPlatformProvider) GetEnv(key string) string {
	if m.EnvVars == nil {
		return ""
	}
	return m.EnvVars[key]
}

func (m *MockPlatformProvider) UserHomeDir() (string, error) {
	if m.HomeDirError != nil {
		return "", m.HomeDirError
	}
	return m.HomeDirPath, nil
}

func TestClaudeDataDirAllPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		platform *MockPlatformProvider
		want     string
	}{
		// Windows tests
		{
			name: "Windows with APPDATA",
			platform: &MockPlatformProvider{
				OS:      "windows",
				EnvVars: map[string]string{"APPDATA": "C:\\Users\\Test\\AppData\\Roaming"},
			},
			want: "C:\\Users\\Test\\AppData\\Roaming\\Claude",
		},
		{
			name: "Windows without APPDATA",
			platform: &MockPlatformProvider{
				OS:      "windows",
				EnvVars: map[string]string{},
			},
			want: "",
		},
		// macOS tests
		{
			name: "macOS happy path",
			platform: &MockPlatformProvider{
				OS:          "darwin",
				HomeDirPath: "/Users/test",
			},
			want: filepath.Join("/Users/test", "Library", "Application Support", "Claude"),
		},
		{
			name: "macOS UserHomeDir error",
			platform: &MockPlatformProvider{
				OS:           "darwin",
				HomeDirError: errors.New("no home directory"),
			},
			want: "",
		},
		// Linux tests
		{
			name: "Linux happy path",
			platform: &MockPlatformProvider{
				OS:          "linux",
				HomeDirPath: "/home/test",
			},
			want: filepath.Join("/home/test", ".config", "Claude"),
		},
		{
			name: "Linux UserHomeDir error",
			platform: &MockPlatformProvider{
				OS:           "linux",
				HomeDirError: errors.New("no home directory"),
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClaudeDataDirWithPlatform(tt.platform)
			if got != tt.want {
				t.Errorf("ClaudeDataDirWithPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserCacheDirAllPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		platform *MockPlatformProvider
		want     string
	}{
		{
			name: "Windows with LOCALAPPDATA",
			platform: &MockPlatformProvider{
				OS:      "windows",
				EnvVars: map[string]string{"LOCALAPPDATA": "C:\\Users\\Test\\AppData\\Local"},
			},
			want: "C:\\Users\\Test\\AppData\\Local\\claude-token-monitor",
		},
		{
			name: "Windows without LOCALAPPDATA",
			platform: &MockPlatformProvider{
				OS:          "windows",
				EnvVars:     map[string]string{},
				HomeDirPath: "C:\\Users\\Test",
			},
			want: "C:\\Users\\Test\\.claude-token-monitor",
		},
		{
			name: "macOS",
			platform: &MockPlatformProvider{
				OS:          "darwin",
				HomeDirPath: "/Users/test",
			},
			want: filepath.Join("/Users/test", "Library", "Caches", "claude-token-monitor"),
		},
		{
			name: "Linux",
			platform: &MockPlatformProvider{
				OS:          "linux",
				HomeDirPath: "/home/test",
			},
			want: filepath.Join("/home/test", ".cache", "claude-token-monitor"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UserCacheDirWithPlatform(tt.platform)
			if got != tt.want {
				t.Errorf("UserCacheDirWithPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}
