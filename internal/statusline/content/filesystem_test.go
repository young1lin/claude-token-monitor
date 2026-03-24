package content

import (
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// normalizePath converts OS-specific path separators to forward slashes
// so that stub map keys match regardless of platform.
func normalizePath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// StubFileSystem provides canned filesystem responses for unit tests.
type StubFileSystem struct {
	StatReturns     map[string]error         // path → error (nil = exists)
	ReadDirReturns  map[string][]fs.DirEntry // path → entries
	ReadFileReturns map[string][]byte        // path → file contents
	HomeDir         string
	HomeDirErr      error
}

func (s *StubFileSystem) Stat(name string) (fs.FileInfo, error) {
	if err, ok := s.StatReturns[normalizePath(name)]; ok {
		if err != nil {
			return nil, err
		}
		return stubFileInfo{name: name}, nil
	}
	return nil, os.ErrNotExist
}

func (s *StubFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	if entries, ok := s.ReadDirReturns[normalizePath(name)]; ok {
		return entries, nil
	}
	return nil, os.ErrNotExist
}

func (s *StubFileSystem) ReadFile(name string) ([]byte, error) {
	if data, ok := s.ReadFileReturns[normalizePath(name)]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (s *StubFileSystem) UserHomeDir() (string, error) {
	return s.HomeDir, s.HomeDirErr
}

// stubFileInfo implements fs.FileInfo for testing.
type stubFileInfo struct {
	name string
}

func (s stubFileInfo) Name() string       { return s.name }
func (s stubFileInfo) Size() int64        { return 0 }
func (s stubFileInfo) Mode() fs.FileMode  { return 0 }
func (s stubFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubFileInfo) IsDir() bool        { return false }
func (s stubFileInfo) Sys() interface{}   { return nil }

// stubDirEntry implements fs.DirEntry for testing.
type stubDirEntry struct {
	name  string
	isDir bool
}

func (s stubDirEntry) Name() string               { return s.name }
func (s stubDirEntry) IsDir() bool                { return s.isDir }
func (s stubDirEntry) Type() fs.FileMode          { return 0 }
func (s stubDirEntry) Info() (fs.FileInfo, error) { return stubFileInfo{name: s.name}, nil }

func restoreFileSystem() {
	defaultFileSystem = &RealFileSystem{}
}

// --- MemoryFilesCollector tests ---

func TestMemoryFilesCollector(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			"/home/test/.claude/CLAUDE.md": nil,
		},
		ReadDirReturns:  map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{},
	}

	collector := NewMemoryFilesCollector()
	assert.Equal(t, ContentMemoryFiles, collector.Type())
	assert.True(t, collector.Optional())

	// Invalid input
	_, err := collector.Collect("wrong", nil)
	assert.Error(t, err)

	// Valid input
	result, err := collector.Collect(&StatusLineInput{Cwd: "/project"}, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestMemoryFilesCollector_EmptyProject(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir:         "/home/test",
		HomeDirErr:      nil,
		StatReturns:     map[string]error{},
		ReadDirReturns:  map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{},
	}

	result := getMemoryFilesInfoCached("/project")
	assert.Equal(t, MemoryFilesInfo{}, result)
}

func TestGetMemoryFilesInfo_WithRules(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	fs := &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			"/project/.claude/rules": nil,
		},
		ReadDirReturns: map[string][]fs.DirEntry{
			"/project/.claude/rules": {
				stubDirEntry{name: "rule1.md", isDir: false},
				stubDirEntry{name: "rule2.md", isDir: false},
				stubDirEntry{name: "_private.md", isDir: false}, // skipped: prefix _
				stubDirEntry{name: ".hidden.md", isDir: false},  // skipped: prefix .
			},
		},
		ReadFileReturns: map[string][]byte{},
	}
	defaultFileSystem = fs

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 2, info.RulesCount)
}

func TestGetMemoryFilesInfo_WithRecursiveRules(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			"/project/.claude/rules": nil,
		},
		ReadDirReturns: map[string][]fs.DirEntry{
			"/project/.claude/rules": {
				stubDirEntry{name: "rule1.md", isDir: false},
				stubDirEntry{name: "subdir", isDir: true},
			},
			"/project/.claude/rules/subdir": {
				stubDirEntry{name: "nested.md", isDir: false},
			},
		},
		ReadFileReturns: map[string][]byte{},
	}

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 2, info.RulesCount) // rule1.md + nested.md
}

func TestGetMemoryFilesInfo_WithMCP(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir:        "/home/test",
		StatReturns:    map[string]error{},
		ReadDirReturns: map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{
			"/project/.claude/mcp_servers.json": []byte(`{"mcpServers":["server1","server2"]}`),
		},
	}

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 2, info.MCPCount)
}

func TestGetMemoryFilesInfo_MCPFromSettings(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir:        "/home/test",
		StatReturns:    map[string]error{},
		ReadDirReturns: map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{
			"/home/test/.claude/settings.json": []byte(`{"mcpServers":{"server1":{},"server2":{}}}`),
		},
	}

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 2, info.MCPCount)
}

func TestGetMemoryFilesInfo_WithClaudeMd(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			"/project/CLAUDE.md":           nil,
			"/project/.claude/CLAUDE.md":   nil,
			"/project/CLAUDE.local.md":     nil,
			"/home/test/.claude/CLAUDE.md": nil,
		},
		ReadDirReturns:  map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{},
	}

	info := getMemoryFilesInfo("/project")
	assert.Equal(t, 4, info.CLAUDEMdCount) // 3 project + 1 global
}

func TestGetMemoryFilesInfo_CacheHit(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		StatReturns: map[string]error{
			"/project/.claude/rules": nil,
		},
		ReadDirReturns: map[string][]fs.DirEntry{
			"/project/.claude/rules": {stubDirEntry{name: "r.md", isDir: false}},
		},
		ReadFileReturns: map[string][]byte{},
	}

	info1 := getMemoryFilesInfoCached("/project")
	assert.Equal(t, 1, info1.RulesCount)

	// Replace fs — should still use cache
	defaultFileSystem = &StubFileSystem{
		HomeDir:         "/home/test",
		StatReturns:     map[string]error{},
		ReadDirReturns:  map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{},
	}

	info2 := getMemoryFilesInfoCached("/project")
	assert.Equal(t, info1, info2)
}

func TestGetMemoryFilesInfo_HomeDirError(t *testing.T) {
	defer restoreFileSystem()
	clearMemoryCache()
	defaultFileSystem = &StubFileSystem{
		HomeDir:         "",
		HomeDirErr:      os.ErrPermission,
		StatReturns:     map[string]error{},
		ReadDirReturns:  map[string][]fs.DirEntry{},
		ReadFileReturns: map[string][]byte{},
	}

	info := getMemoryFilesInfo("/project")
	// Should not crash, just skip global paths
	assert.Equal(t, 0, info.CLAUDEMdCount)
}

// --- SkillsCollector tests ---

func TestSkillsCollector(t *testing.T) {
	defer restoreFileSystem()
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		ReadDirReturns: map[string][]fs.DirEntry{
			"/home/test/.claude/skills": {
				stubDirEntry{name: "skill1", isDir: true},
				stubDirEntry{name: ".hidden", isDir: true},
			},
			"/project/.claude/commands": {
				stubDirEntry{name: "deploy.md", isDir: false},
				stubDirEntry{name: "README.md", isDir: false}, // not .md suffix matched
			},
		},
	}

	collector := NewSkillsCollector()
	assert.Equal(t, ContentSkills, collector.Type())
	assert.True(t, collector.Optional())

	// Invalid input
	_, err := collector.Collect(123, nil)
	assert.Error(t, err)

	// Valid input
	result, err := collector.Collect(&StatusLineInput{Cwd: "/project"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "🎯 3 skills(2 proj + 1 user)", result)
}

func TestSkillsCollector_NoSkills(t *testing.T) {
	defer restoreFileSystem()
	defaultFileSystem = &StubFileSystem{
		HomeDir:        "/home/test",
		HomeDirErr:     os.ErrPermission,
		ReadDirReturns: map[string][]fs.DirEntry{},
	}

	collector := NewSkillsCollector()
	result, err := collector.Collect(&StatusLineInput{Cwd: "/project"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestSkillsCollector_OnlyProjectSkills(t *testing.T) {
	defer restoreFileSystem()
	defaultFileSystem = &StubFileSystem{
		HomeDir:    "/home/test",
		HomeDirErr: nil,
		ReadDirReturns: map[string][]fs.DirEntry{
			"/home/test/.claude/skills": {}, // empty
			"/project/.claude/commands": {
				stubDirEntry{name: "test.md", isDir: false},
			},
		},
	}

	result, _ := NewSkillsCollector().Collect(&StatusLineInput{Cwd: "/project"}, nil)
	assert.Equal(t, "🎯 1 proj skills", result)
}

func TestSkillsCollector_OnlyUserSkills(t *testing.T) {
	defer restoreFileSystem()
	defaultFileSystem = &StubFileSystem{
		HomeDir: "/home/test",
		ReadDirReturns: map[string][]fs.DirEntry{
			"/home/test/.claude/skills": {
				stubDirEntry{name: "code-review", isDir: true},
			},
			"/project/.claude/commands": {}, // empty
		},
	}

	result, _ := NewSkillsCollector().Collect(&StatusLineInput{Cwd: "/project"}, nil)
	assert.Equal(t, "🎯 1 user skills", result)
}

func TestCountSkillDirs_HiddenSkipped(t *testing.T) {
	defer restoreFileSystem()
	defaultFileSystem = &StubFileSystem{
		ReadDirReturns: map[string][]fs.DirEntry{
			"/skills": {
				stubDirEntry{name: "visible", isDir: true},
				stubDirEntry{name: ".git", isDir: true},     // hidden
				stubDirEntry{name: "file.md", isDir: false}, // not a dir
			},
		},
	}

	count := countSkillDirs("/skills")
	assert.Equal(t, 1, count)
}

func TestCountSkillFiles_HiddenAndNonMdSkipped(t *testing.T) {
	defer restoreFileSystem()
	defaultFileSystem = &StubFileSystem{
		ReadDirReturns: map[string][]fs.DirEntry{
			"/cmds": {
				stubDirEntry{name: "deploy.md", isDir: false},
				stubDirEntry{name: ".hidden.md", isDir: false},
				stubDirEntry{name: "subdir", isDir: true},
				stubDirEntry{name: "readme.txt", isDir: false},
			},
		},
	}

	count := countSkillFiles("/cmds")
	assert.Equal(t, 1, count)
}
