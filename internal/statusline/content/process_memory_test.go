package content

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Stub ──────────────────────────────────────────────────────────────

// StubProcessMemoryReader returns canned values for testing.
type StubProcessMemoryReader struct {
	MemoryMB float64
	Err      error
}

func (s *StubProcessMemoryReader) ReadParentMemoryMB() (float64, error) {
	return s.MemoryMB, s.Err
}

// StubProcessNameAndPPID returns canned process name and parent PID.
type StubProcessNameAndPPID struct {
	Results map[int]struct {
		Name string
		PPID int
		Err  error
	}
}

func (s *StubProcessNameAndPPID) Get(pid int) (string, int, error) {
	r, ok := s.Results[pid]
	if !ok {
		return "", 0, fmt.Errorf("no stub for pid %d", pid)
	}
	return r.Name, r.PPID, r.Err
}

// ── formatParentMemory ────────────────────────────────────────────────

func TestFormatParentMemory(t *testing.T) {
	tests := []struct {
		name string
		mb   float64
		want string
	}{
		{"integer value", 256.0, "\U0001F4BE 256.0 MB"},
		{"one decimal", 123.4, "\U0001F4BE 123.4 MB"},
		{"rounds up", 123.456, "\U0001F4BE 123.5 MB"},
		{"small value", 0.1, "\U0001F4BE 0.1 MB"},
		{"large value", 4096.7, "\U0001F4BE 4096.7 MB"},
		{"zero", 0.0, "\U0001F4BE 0.0 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatParentMemory(tt.mb)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ── ParentMemoryCollector ─────────────────────────────────────────────

func TestParentMemoryCollector_Type(t *testing.T) {
	c := NewParentMemoryCollector()
	assert.Equal(t, ContentParentMemory, c.Type())
}

func TestParentMemoryCollector_CacheTTL(t *testing.T) {
	c := NewParentMemoryCollector()
	assert.Equal(t, 5*time.Second, c.CacheTTL())
}

func TestParentMemoryCollector_Optional(t *testing.T) {
	c := NewParentMemoryCollector()
	assert.True(t, c.Optional())
}

func TestParentMemoryCollector_Success(t *testing.T) {
	// Arrange
	old := defaultProcessMemoryReader
	defaultProcessMemoryReader = &StubProcessMemoryReader{MemoryMB: 256.7}
	defer func() { defaultProcessMemoryReader = old }()

	c := NewParentMemoryCollector()

	// Act
	got, err := c.Collect(nil, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "\U0001F4BE 256.7 MB", got)
}

func TestParentMemoryCollector_ErrorReturnsEmpty(t *testing.T) {
	// Arrange
	old := defaultProcessMemoryReader
	defaultProcessMemoryReader = &StubProcessMemoryReader{Err: fmt.Errorf("access denied")}
	defer func() { defaultProcessMemoryReader = old }()

	c := NewParentMemoryCollector()

	// Act
	got, err := c.Collect(nil, nil)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParentMemoryCollector_NilInputAndSummary(t *testing.T) {
	// Arrange
	old := defaultProcessMemoryReader
	defaultProcessMemoryReader = &StubProcessMemoryReader{MemoryMB: 100.0}
	defer func() { defaultProcessMemoryReader = old }()

	c := NewParentMemoryCollector()

	// Act — nil input and nil summary should still work
	got, err := c.Collect(nil, nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "\U0001F4BE 100.0 MB", got)
}

// ── findClaudeCodePID ─────────────────────────────────────────────────

func TestFindClaudeCodePID_DirectParentIsClaude(t *testing.T) {
	// Arrange
	old := getProcessNameAndPPIDFn
	defer func() { getProcessNameAndPPIDFn = old }()

	stub := &StubProcessNameAndPPID{
		Results: map[int]struct {
			Name string
			PPID int
			Err  error
		}{
			100: {Name: "claude.exe", PPID: 1},
		},
	}
	getProcessNameAndPPIDFn = stub.Get

	// Act
	pid, err := findClaudeCodePID(100)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 100, pid)
}

func TestFindClaudeCodePID_GrandparentIsClaude(t *testing.T) {
	// Arrange — statusline(10) → shell(20) → claude(30)
	old := getProcessNameAndPPIDFn
	defer func() { getProcessNameAndPPIDFn = old }()

	stub := &StubProcessNameAndPPID{
		Results: map[int]struct {
			Name string
			PPID int
			Err  error
		}{
			10: {Name: "statusline.exe", PPID: 20},
			20: {Name: "cmd.exe", PPID: 30},
			30: {Name: "claude.exe", PPID: 1},
		},
	}
	getProcessNameAndPPIDFn = stub.Get

	// Act
	pid, err := findClaudeCodePID(10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 30, pid)
}

func TestFindClaudeCodePID_ElectronMatch(t *testing.T) {
	// Arrange — electron is also a valid match
	old := getProcessNameAndPPIDFn
	defer func() { getProcessNameAndPPIDFn = old }()

	stub := &StubProcessNameAndPPID{
		Results: map[int]struct {
			Name string
			PPID int
			Err  error
		}{
			10: {Name: "statusline.exe", PPID: 50},
			50: {Name: "electron.exe", PPID: 1},
		},
	}
	getProcessNameAndPPIDFn = stub.Get

	// Act
	pid, err := findClaudeCodePID(10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 50, pid)
}

func TestFindClaudeCodePID_NoMatchReturnsStart(t *testing.T) {
	// Arrange — none of the ancestors are claude/electron
	old := getProcessNameAndPPIDFn
	defer func() { getProcessNameAndPPIDFn = old }()

	stub := &StubProcessNameAndPPID{
		Results: map[int]struct {
			Name string
			PPID int
			Err  error
		}{
			10: {Name: "statusline.exe", PPID: 20},
			20: {Name: "cmd.exe", PPID: 30},
			30: {Name: "explorer.exe", PPID: 1},
		},
	}
	getProcessNameAndPPIDFn = stub.Get

	// Act
	pid, err := findClaudeCodePID(10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 10, pid, "should fallback to start PID when no match")
}

func TestFindClaudeCodePID_StopsAtRoot(t *testing.T) {
	// Arrange — parent is self (root)
	old := getProcessNameAndPPIDFn
	defer func() { getProcessNameAndPPIDFn = old }()

	stub := &StubProcessNameAndPPID{
		Results: map[int]struct {
			Name string
			PPID int
			Err  error
		}{
			10: {Name: "something.exe", PPID: 10},
		},
	}
	getProcessNameAndPPIDFn = stub.Get

	// Act
	pid, err := findClaudeCodePID(10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 10, pid)
}

func TestFindClaudeCodePID_ErrorReturnsCurrentPID(t *testing.T) {
	// Arrange — getProcessNameAndPPID fails
	old := getProcessNameAndPPIDFn
	defer func() { getProcessNameAndPPIDFn = old }()

	getProcessNameAndPPIDFn = func(pid int) (string, int, error) {
		return "", 0, fmt.Errorf("access denied")
	}

	// Act
	pid, err := findClaudeCodePID(42)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 42, pid)
}

func TestFindClaudeCodePID_MaxDepth(t *testing.T) {
	// Arrange — 10 levels deep, none match
	old := getProcessNameAndPPIDFn
	defer func() { getProcessNameAndPPIDFn = old }()

	results := map[int]struct {
		Name string
		PPID int
		Err  error
	}{}
	for i := 0; i <= 10; i++ {
		results[i] = struct {
			Name string
			PPID int
			Err  error
		}{Name: "generic.exe", PPID: i + 1}
	}
	stub := &StubProcessNameAndPPID{Results: results}
	getProcessNameAndPPIDFn = stub.Get

	// Act
	pid, err := findClaudeCodePID(0)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, pid, "should fallback after max depth")
}

// ── RealProcessMemoryReader (via DI) ─────────────────────────────────

func TestRealProcessMemoryReader_InvalidParentPID(t *testing.T) {
	// Arrange — inject getppidFn returning 0
	oldPPID := getppidFn
	defer func() { getppidFn = oldPPID }()
	getppidFn = func() int { return 0 }

	reader := &RealProcessMemoryReader{}

	// Act
	_, err := reader.ReadParentMemoryMB()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent pid")
}

func TestRealProcessMemoryReader_NegativeParentPID(t *testing.T) {
	// Arrange
	oldPPID := getppidFn
	defer func() { getppidFn = oldPPID }()
	getppidFn = func() int { return -1 }

	reader := &RealProcessMemoryReader{}

	// Act
	_, err := reader.ReadParentMemoryMB()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent pid")
}

func TestRealProcessMemoryReader_Success(t *testing.T) {
	// Arrange — inject stubs so no real OS calls are made
	oldNamePPID := getProcessNameAndPPIDFn
	oldMemFn := getProcessMemoryMBFn
	defer func() {
		getProcessNameAndPPIDFn = oldNamePPID
		getProcessMemoryMBFn = oldMemFn
	}()

	getProcessNameAndPPIDFn = func(pid int) (string, int, error) {
		return "claude.exe", 1, nil
	}
	getProcessMemoryMBFn = func(pid int) (float64, error) {
		return 512.3, nil
	}

	reader := &RealProcessMemoryReader{}

	// Act
	mb, err := reader.ReadParentMemoryMB()

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 512.3, mb, 0.001)
}

func TestRealProcessMemoryReader_MemoryReadError(t *testing.T) {
	// Arrange
	oldNamePPID := getProcessNameAndPPIDFn
	oldMemFn := getProcessMemoryMBFn
	defer func() {
		getProcessNameAndPPIDFn = oldNamePPID
		getProcessMemoryMBFn = oldMemFn
	}()

	getProcessNameAndPPIDFn = func(pid int) (string, int, error) {
		return "claude.exe", 1, nil
	}
	getProcessMemoryMBFn = func(pid int) (float64, error) {
		return 0, fmt.Errorf("access denied")
	}

	reader := &RealProcessMemoryReader{}

	// Act
	_, err := reader.ReadParentMemoryMB()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

// ── StubProcessMemoryReader ───────────────────────────────────────────

func TestStubProcessMemoryReader_ReturnsValue(t *testing.T) {
	stub := &StubProcessMemoryReader{MemoryMB: 512.3}
	mb, err := stub.ReadParentMemoryMB()
	require.NoError(t, err)
	assert.InDelta(t, 512.3, mb, 0.001)
}

func TestStubProcessMemoryReader_ReturnsError(t *testing.T) {
	stub := &StubProcessMemoryReader{Err: fmt.Errorf("test error")}
	_, err := stub.ReadParentMemoryMB()
	assert.Error(t, err)
}

// ── Delegation function: getProcessMemoryMB ──────────────────────────
// getProcessMemoryMB calls getProcessMemoryMBPlatform which uses real
// syscalls. Full delegation tests live in process_memory_windows_test.go
// (or other platform test files) where syscall seams can be stubbed.
// Here we only test the var-override path used by callers like
// ReadParentMemoryMB.

func TestGetProcessMemoryMBFn_OverrideWorks(t *testing.T) {
	old := getProcessMemoryMBFn
	defer func() { getProcessMemoryMBFn = old }()

	getProcessMemoryMBFn = func(pid int) (float64, error) {
		return 777.7, nil
	}

	// RealProcessMemoryReader uses getProcessMemoryMBFn, not getProcessMemoryMB directly
	oldPPID := getppidFn
	oldNamePPID := getProcessNameAndPPIDFn
	defer func() {
		getppidFn = oldPPID
		getProcessNameAndPPIDFn = oldNamePPID
	}()
	getppidFn = func() int { return 42 }
	getProcessNameAndPPIDFn = func(pid int) (string, int, error) {
		return "claude.exe", 1, nil
	}

	reader := &RealProcessMemoryReader{}
	mb, err := reader.ReadParentMemoryMB()
	require.NoError(t, err)
	assert.InDelta(t, 777.7, mb, 0.001)
}
