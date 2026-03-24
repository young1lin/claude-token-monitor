package content

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatMemoryFilesDisplay(t *testing.T) {
	tests := []struct {
		name string
		info MemoryFilesInfo
		want string
	}{
		{
			name: "all fields populated",
			info: MemoryFilesInfo{
				CLAUDEMdCount: 2,
				RulesCount:    3,
				MCPCount:      1,
			},
			want: "📦 2 CLAUDE.md + 3 rules + 1 MCPs",
		},
		{
			name: "single CLAUDE.md only",
			info: MemoryFilesInfo{
				CLAUDEMdCount: 1,
			},
			want: "📦 CLAUDE.md",
		},
		{
			name: "no fields returns empty",
			info: MemoryFilesInfo{},
			want: "",
		},
		{
			name: "rules and MCPs without CLAUDE.md",
			info: MemoryFilesInfo{
				RulesCount: 2,
				MCPCount:   3,
			},
			want: "📦 2 rules + 3 MCPs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := formatMemoryFilesDisplay(tt.info)

			// Assert
			if got != tt.want {
				t.Errorf("formatMemoryFilesDisplay() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountRulesRecursive(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  int
	}{
		{
			name: "non-existent directory returns zero",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			want: 0,
		},
		{
			name: "empty directory returns zero",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: 0,
		},
		{
			name: "only .md files",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule1.md"), []byte("# rule1"), 0644)
				os.WriteFile(filepath.Join(dir, "rule2.md"), []byte("# rule2"), 0644)
				return dir
			},
			want: 2,
		},
		{
			name: "non-.md files are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule.md"), []byte("# rule"), 0644)
				os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(dir, "data.yaml"), []byte("key: val"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "hidden files (dot prefix) are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule.md"), []byte("# rule"), 0644)
				os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("hidden"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "underscore-prefixed files are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule.md"), []byte("# rule"), 0644)
				os.WriteFile(filepath.Join(dir, "_draft.md"), []byte("draft"), 0644)
				os.WriteFile(filepath.Join(dir, "_internal.md"), []byte("internal"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "subdirectories are recursed into",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule1.md"), []byte("# rule1"), 0644)
				subDir := filepath.Join(dir, "subgroup")
				require.NoError(t, os.Mkdir(subDir, 0755))
				os.WriteFile(filepath.Join(subDir, "rule2.md"), []byte("# rule2"), 0644)
				os.WriteFile(filepath.Join(subDir, "rule3.md"), []byte("# rule3"), 0644)
				return dir
			},
			want: 3,
		},
		{
			name: "hidden subdirectories are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule.md"), []byte("# rule"), 0644)
				hiddenDir := filepath.Join(dir, ".hidden")
				require.NoError(t, os.Mkdir(hiddenDir, 0755))
				os.WriteFile(filepath.Join(hiddenDir, "secret.md"), []byte("secret"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "underscore-prefixed subdirectories are skipped",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule.md"), []byte("# rule"), 0644)
				underscoreDir := filepath.Join(dir, "_internal")
				require.NoError(t, os.Mkdir(underscoreDir, 0755))
				os.WriteFile(filepath.Join(underscoreDir, "internal.md"), []byte("internal"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "deeply nested subdirectories",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "rule1.md"), []byte("# rule1"), 0644)
				level1 := filepath.Join(dir, "group1")
				require.NoError(t, os.Mkdir(level1, 0755))
				os.WriteFile(filepath.Join(level1, "rule2.md"), []byte("# rule2"), 0644)
				level2 := filepath.Join(level1, "group2")
				require.NoError(t, os.Mkdir(level2, 0755))
				os.WriteFile(filepath.Join(level2, "rule3.md"), []byte("# rule3"), 0644)
				return dir
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			rulesDir := tt.setup(t)

			// Act
			got := countRulesRecursive(rulesDir)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCountClaudeMdUpward(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  int
	}{
		{
			name: "no CLAUDE.md files",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: 0,
		},
		{
			name: "CLAUDE.md in current directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# rules"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "CLAUDE.md in .claude subdirectory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				claudeDir := filepath.Join(dir, ".claude")
				require.NoError(t, os.Mkdir(claudeDir, 0755))
				os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("# project rules"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "CLAUDE.local.md in current directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "CLAUDE.local.md"), []byte("# local rules"), 0644)
				return dir
			},
			want: 1,
		},
		{
			name: "multiple CLAUDE.md at different levels",
			setup: func(t *testing.T) string {
				baseDir := t.TempDir()
				// CLAUDE.md in base dir
				os.WriteFile(filepath.Join(baseDir, "CLAUDE.md"), []byte("# base rules"), 0644)
				// Create subdirectory
				subDir := filepath.Join(baseDir, "subproject")
				require.NoError(t, os.Mkdir(subDir, 0755))
				os.WriteFile(filepath.Join(subDir, "CLAUDE.md"), []byte("# sub rules"), 0644)
				// CLAUDE.local.md in sub dir
				os.WriteFile(filepath.Join(subDir, "CLAUDE.local.md"), []byte("# local sub rules"), 0644)
				return subDir
			},
			want: 3, // sub CLAUDE.md + sub CLAUDE.local.md + base CLAUDE.md
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cwd := tt.setup(t)

			// Act
			got := countClaudeMdUpward(cwd)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetMCPCount(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  int
	}{
		{
			name: "no mcp_servers.json and no settings.json returns zero",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				t.Setenv("HOME", dir)
				t.Setenv("USERPROFILE", dir)
				return dir
			},
			want: 0,
		},
		{
			name: "valid mcp_servers.json with mcpServers array",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				claudeDir := filepath.Join(dir, ".claude")
				require.NoError(t, os.Mkdir(claudeDir, 0755))
				mcpData := map[string]interface{}{
					"mcpServers": []interface{}{
						map[string]string{"name": "server1"},
						map[string]string{"name": "server2"},
						map[string]string{"name": "server3"},
					},
				}
				data, err := json.Marshal(mcpData)
				require.NoError(t, err)
				os.WriteFile(filepath.Join(claudeDir, "mcp_servers.json"), data, 0644)

				t.Setenv("HOME", dir)
				t.Setenv("USERPROFILE", dir)
				return dir
			},
			want: 3,
		},
		{
			name: "valid mcp_servers.json with flat object (no mcpServers key)",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				claudeDir := filepath.Join(dir, ".claude")
				require.NoError(t, os.Mkdir(claudeDir, 0755))
				mcpData := map[string]interface{}{
					"server-a": map[string]string{"url": "http://a"},
					"server-b": map[string]string{"url": "http://b"},
				}
				data, err := json.Marshal(mcpData)
				require.NoError(t, err)
				os.WriteFile(filepath.Join(claudeDir, "mcp_servers.json"), data, 0644)

				t.Setenv("HOME", dir)
				t.Setenv("USERPROFILE", dir)
				return dir
			},
			want: 2,
		},
		{
			name: "empty mcp_servers.json object returns zero",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				claudeDir := filepath.Join(dir, ".claude")
				require.NoError(t, os.Mkdir(claudeDir, 0755))
				os.WriteFile(filepath.Join(claudeDir, "mcp_servers.json"), []byte("{}"), 0644)

				t.Setenv("HOME", dir)
				t.Setenv("USERPROFILE", dir)
				return dir
			},
			want: 0,
		},
		{
			name: "falls back to settings.json when no mcp_servers.json",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				claudeDir := filepath.Join(dir, ".claude")
				require.NoError(t, os.Mkdir(claudeDir, 0755))

				settings := map[string]interface{}{
					"mcpServers": map[string]interface{}{
						"server-x": map[string]string{"url": "http://x"},
						"server-y": map[string]string{"url": "http://y"},
					},
				}
				data, err := json.Marshal(settings)
				require.NoError(t, err)
				os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644)

				t.Setenv("HOME", dir)
				t.Setenv("USERPROFILE", dir)
				return dir
			},
			want: 2,
		},
		{
			name: "mcp_servers.json takes priority over settings.json",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				claudeDir := filepath.Join(dir, ".claude")
				require.NoError(t, os.Mkdir(claudeDir, 0755))

				// mcp_servers.json with 2 servers
				mcpData := map[string]interface{}{
					"mcpServers": []interface{}{
						map[string]string{"name": "primary1"},
						map[string]string{"name": "primary2"},
					},
				}
				mcpJSON, err := json.Marshal(mcpData)
				require.NoError(t, err)
				os.WriteFile(filepath.Join(claudeDir, "mcp_servers.json"), mcpJSON, 0644)

				// settings.json with 3 servers (should be ignored)
				settings := map[string]interface{}{
					"mcpServers": map[string]interface{}{
						"fallback-a": map[string]string{},
						"fallback-b": map[string]string{},
						"fallback-c": map[string]string{},
					},
				}
				settingsJSON, err := json.Marshal(settings)
				require.NoError(t, err)
				os.WriteFile(filepath.Join(claudeDir, "settings.json"), settingsJSON, 0644)

				t.Setenv("HOME", dir)
				t.Setenv("USERPROFILE", dir)
				return dir
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cwd := tt.setup(t)

			// Act
			got := getMCPCount(cwd)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetMemoryFilesInfo_WithGlobalRules(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir:     "/home/test",
		StatReturns: map[string]error{},
		ReadDirReturns: map[string][]fs.DirEntry{
			"/home/test/.claude/rules": {
				stubDirEntry{name: "global-rule.md", isDir: false},
			},
		},
		ReadFileReturns: map[string][]byte{},
	}

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 1, info.RulesCount) // global rule
}

func TestGetMemoryFilesInfo_UpwardTraversal(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			// Parent dir has rules
			"/parent/.claude/rules": nil,
		},
		ReadDirReturns: map[string][]fs.DirEntry{
			"/parent/.claude/rules": {
				stubDirEntry{name: "parent-rule.md", isDir: false},
			},
		},
		ReadFileReturns: map[string][]byte{},
	}

	info := getMemoryFilesInfo("/parent/sub/project")
	assert.Equal(t, 1, info.RulesCount)
}

func TestGetMemoryFilesInfo_EnterprisePolicy(t *testing.T) {
	// Simulates Windows with Enterprise CLAUDE.md file present.
	defer restoreFileSystem()
	clearMemoryCache()
	oldOS := currentOS
	currentOS = "windows"
	defer func() { currentOS = oldOS }()

	// StubFileSystem normalizes paths via normalizePath (backslash → forward slash).
	// On Windows, filepath.Join produces backslashes, so use forward slashes for the key.
	enterprisePath := normalizePath(filepath.Join("C:", "Program Files", "ClaudeCode", "CLAUDE.md"))
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			enterprisePath: nil, // Enterprise file exists
		},
		ReadDirReturns:  map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{},
	}

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 1, info.CLAUDEMdCount, "enterprise CLAUDE.md should be counted")
}

func TestGetMemoryFilesInfo_EnterprisePolicyNotWindows(t *testing.T) {
	// On non-Windows, Enterprise path is skipped even if Stat succeeds.
	defer restoreFileSystem()
	clearMemoryCache()
	oldOS := currentOS
	currentOS = "linux"
	defer func() { currentOS = oldOS }()

	enterprisePath := normalizePath(filepath.Join("C:", "Program Files", "ClaudeCode", "CLAUDE.md"))
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			enterprisePath: nil, // Stat succeeds but OS is not windows
		},
		ReadDirReturns:  map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{},
	}

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 0, info.CLAUDEMdCount, "enterprise path skipped on non-windows")
}

func TestFormatMemoryFilesDisplay_AllZero(t *testing.T) {
	got := formatMemoryFilesDisplay(MemoryFilesInfo{})
	assert.Empty(t, got)
}

func TestFormatMemoryFilesDisplay_OnlyMCPs(t *testing.T) {
	got := formatMemoryFilesDisplay(MemoryFilesInfo{MCPCount: 3})
	assert.Equal(t, "📦 3 MCPs", got)
}

func TestFormatMemoryFilesDisplay_SingleCLAUDEMd(t *testing.T) {
	got := formatMemoryFilesDisplay(MemoryFilesInfo{CLAUDEMdCount: 1})
	assert.Equal(t, "📦 CLAUDE.md", got)
}

func TestFormatMemoryFilesDisplay_MultipleCLAUDEMd(t *testing.T) {
	got := formatMemoryFilesDisplay(MemoryFilesInfo{CLAUDEMdCount: 3})
	assert.Equal(t, "📦 3 CLAUDE.md", got)
}
