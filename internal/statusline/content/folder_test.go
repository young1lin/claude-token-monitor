package content

import (
	"testing"
)

func TestGetProjectName(t *testing.T) {
	tests := []struct {
		name     string
		cwd      string
		expected string
	}{
		// Linux paths
		{
			name:     "Linux standard path",
			cwd:      "/home/user/minimal-mcp",
			expected: "minimal-mcp",
		},
		{
			name:     "Linux /opt path",
			cwd:      "/opt/projects/my-app",
			expected: "my-app",
		},
		{
			name:     "Linux deep path",
			cwd:      "/home/user/projects/go/my-project",
			expected: "my-project",
		},
		// macOS paths
		{
			name:     "macOS standard path",
			cwd:      "/Users/john/minimal-mcp",
			expected: "minimal-mcp",
		},
		{
			name:     "macOS Applications path",
			cwd:      "/Applications/MyApp",
			expected: "MyApp",
		},
		{
			name:     "macOS Documents path",
			cwd:      "/Users/john/Documents/my-project",
			expected: "my-project",
		},
		// Windows paths
		{
			name:     "Windows C drive",
			cwd:      "C:\\Users\\User\\minimal-mcp",
			expected: "minimal-mcp",
		},
		{
			name:     "Windows D drive",
			cwd:      "D:\\Projects\\myapp",
			expected: "myapp",
		},
		{
			name:     "Windows with spaces",
			cwd:      "C:\\Users\\John Doe\\My Project",
			expected: "My Project",
		},
		// Long names (>25 chars)
		{
			name:     "Linux long name",
			cwd:      "/home/user/very-long-project-name-that-exceeds-limit",
			expected: "very-long-project-name..",
		},
		{
			name:     "Windows long name",
			cwd:      "C:\\Users\\User\\another-very-long-project-name-here",
			expected: "another-very-long-proj..",
		},
		{
			name:     "macOS long name",
			cwd:      "/Users/john/extremely-long-project-folder-name-exceeds-max",
			expected: "extremely-long-project..",
		},
		// Edge cases
		{
			name:     "Empty string",
			cwd:      "",
			expected: "",
		},
		{
			name:     "Linux root",
			cwd:      "/",
			expected: "\\",
		},
		{
			name:     "Windows root",
			cwd:      "C:\\",
			expected: "\\",
		},
		{
			name:     "Single component",
			cwd:      "project",
			expected: "project",
		},
		{
			name:     "Current directory",
			cwd:      ".",
			expected: ".",
		},
		{
			name:     "Parent directory",
			cwd:      "..",
			expected: "..",
		},
		{
			name:     "Trailing slash Linux",
			cwd:      "/home/user/minimal-mcp/",
			expected: "minimal-mcp",
		},
		{
			name:     "Trailing slash Windows",
			cwd:      "C:\\Users\\User\\minimal-mcp\\",
			expected: "minimal-mcp",
		},
		// Exactly 25 characters (boundary test)
		{
			name:     "Exactly 25 characters",
			cwd:      "/home/user/1234567890123456789012345",
			expected: "1234567890123456789012345",
		},
		// 26 characters (should truncate)
		{
			name:     "Exactly 26 characters",
			cwd:      "/home/user/12345678901234567890123456",
			expected: "1234567890123456789012..",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// (test setup already done in table)

			// Act
			result := getProjectName(tt.cwd)

			// Assert
			if result != tt.expected {
				t.Errorf("getProjectName(%q) = %q; want %q", tt.cwd, result, tt.expected)
			}
		})
	}
}

func TestFolderCollector_Collect(t *testing.T) {
	tests := []struct {
		name      string
		input     *StatusLineInput
		expected  string
		shouldErr bool
	}{
		{
			name: "Valid Linux path",
			input: &StatusLineInput{
				Cwd: "/home/user/minimal-mcp",
			},
			expected:  "minimal-mcp",
			shouldErr: false,
		},
		{
			name: "Valid Windows path",
			input: &StatusLineInput{
				Cwd: "C:\\Users\\User\\my-project",
			},
			expected:  "my-project",
			shouldErr: false,
		},
		{
			name: "Empty cwd",
			input: &StatusLineInput{
				Cwd: "",
			},
			expected:  "",
			shouldErr: false,
		},
		{
			name:      "Invalid input type",
			input:     nil,
			expected:  "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			collector := NewFolderCollector()

			// Act
			var result string
			var err error
			if tt.input != nil {
				result, err = collector.Collect(tt.input, nil)
			} else {
				result, err = collector.Collect("invalid", nil)
			}

			// Assert
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Collect() = %q; want %q", result, tt.expected)
				}
			}
		})
	}
}

func TestFolderCollector_Properties(t *testing.T) {
	// Arrange
	collector := NewFolderCollector()

	// Act & Assert
	if collector.Type() != ContentFolder {
		t.Errorf("Type() = %v; want %v", collector.Type(), ContentFolder)
	}

	if collector.Optional() {
		t.Errorf("Optional() = true; want false")
	}
}
