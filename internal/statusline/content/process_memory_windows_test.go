//go:build windows

package content

import (
	"fmt"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────

// saveAndRestoreWindowsFns saves all injectable Windows seams and returns
// a cleanup function that restores them. Call via defer in every test.
func saveAndRestoreWindowsFns() func() {
	oldOpen := openProcessFn
	oldClose := closeHandleFn
	oldNtQuery := ntQueryInfoProcessFn
	oldK32 := k32GetMemInfoFn
	oldSnap := createSnapshotFn
	oldCloseSnap := closeSnapshotFn
	oldFirst := process32FirstWFn
	oldNext := process32NextWFn
	return func() {
		openProcessFn = oldOpen
		closeHandleFn = oldClose
		ntQueryInfoProcessFn = oldNtQuery
		k32GetMemInfoFn = oldK32
		createSnapshotFn = oldSnap
		closeSnapshotFn = oldCloseSnap
		process32FirstWFn = oldFirst
		process32NextWFn = oldNext
	}
}

// ── utf16ToString ────────────────────────────────────────────────────

func TestUtf16ToString(t *testing.T) {
	tests := []struct {
		name  string
		input []uint16
		want  string
	}{
		{
			name:  "simple exe name",
			input: []uint16{'h', 'e', 'l', 'l', 'o', '.', 'e', 'x', 'e', 0},
			want:  "hello.exe",
		},
		{
			name:  "full path extracts base name",
			input: []uint16{'C', ':', '\\', 'f', 'o', 'o', '\\', 'b', 'a', 'r', '.', 'e', 'x', 'e', 0},
			want:  "bar.exe",
		},
		{
			name:  "uppercase converted to lower",
			input: []uint16{'C', 'L', 'A', 'U', 'D', 'E', '.', 'E', 'X', 'E', 0},
			want:  "claude.exe",
		},
		{
			name:  "null terminator at start",
			input: []uint16{0, 'a', 'b'},
			want:  ".",
		},
		{
			name:  "no null terminator uses full slice",
			input: []uint16{'a', 'b', 'c'},
			want:  "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utf16ToString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ── getProcessMemoryMBPlatform ───────────────────────────────────────

func TestGetProcessMemoryMBPlatform_OpenProcessError(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	openProcessFn = func(access uint32, inherit bool, pid uint32) (syscall.Handle, error) {
		return 0, fmt.Errorf("access denied")
	}

	// Act
	_, err := getProcessMemoryMBPlatform(1234)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenProcess(1234)")
	assert.Contains(t, err.Error(), "access denied")
}

func TestGetProcessMemoryMBPlatform_NtQuerySuccess(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	openProcessFn = func(access uint32, inherit bool, pid uint32) (syscall.Handle, error) {
		return syscall.Handle(0xBEEF), nil
	}
	closeHandleFn = func(h syscall.Handle) error { return nil }

	// NtQuery returns SUCCESS (0) and fills PrivateWorkingSetSize
	ntQueryInfoProcessFn = func(handle, infoClass uintptr, buf unsafe.Pointer, bufSize uintptr, retLen *uint32) uintptr {
		counters := (*vmCountersEx2)(buf)
		counters.PrivateWorkingSetSize = 512 * 1024 * 1024 // 512 MB
		*retLen = uint32(unsafe.Sizeof(vmCountersEx2{}))
		return 0 // NTSTATUS SUCCESS
	}

	// Act
	mb, err := getProcessMemoryMBPlatform(42)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 512.0, mb, 0.001)
}

func TestGetProcessMemoryMBPlatform_NtQueryZero_K32Fallback(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	openProcessFn = func(access uint32, inherit bool, pid uint32) (syscall.Handle, error) {
		return syscall.Handle(0xBEEF), nil
	}
	closeHandleFn = func(h syscall.Handle) error { return nil }

	// NtQuery returns success but PrivateWorkingSetSize = 0 (old Windows)
	ntQueryInfoProcessFn = func(handle, infoClass uintptr, buf unsafe.Pointer, bufSize uintptr, retLen *uint32) uintptr {
		// Leave all counters at zero
		return 0
	}

	// K32 fallback succeeds
	k32GetMemInfoFn = func(handle uintptr, pmc unsafe.Pointer, cb uint32) error {
		counters := (*processMemoryCountersEx)(pmc)
		counters.WorkingSetSize = 256 * 1024 * 1024 // 256 MB
		return nil
	}

	// Act
	mb, err := getProcessMemoryMBPlatform(42)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 256.0, mb, 0.001)
}

func TestGetProcessMemoryMBPlatform_NtQueryFail_K32Fallback(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	openProcessFn = func(access uint32, inherit bool, pid uint32) (syscall.Handle, error) {
		return syscall.Handle(0xBEEF), nil
	}
	closeHandleFn = func(h syscall.Handle) error { return nil }

	// NtQuery returns non-zero NTSTATUS (failure)
	ntQueryInfoProcessFn = func(handle, infoClass uintptr, buf unsafe.Pointer, bufSize uintptr, retLen *uint32) uintptr {
		return 0xC0000001 // STATUS_UNSUCCESSFUL
	}

	// K32 fallback succeeds
	k32GetMemInfoFn = func(handle uintptr, pmc unsafe.Pointer, cb uint32) error {
		counters := (*processMemoryCountersEx)(pmc)
		counters.WorkingSetSize = 128 * 1024 * 1024 // 128 MB
		return nil
	}

	// Act
	mb, err := getProcessMemoryMBPlatform(42)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 128.0, mb, 0.001)
}

func TestGetProcessMemoryMBPlatform_BothFail(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	openProcessFn = func(access uint32, inherit bool, pid uint32) (syscall.Handle, error) {
		return syscall.Handle(0xBEEF), nil
	}
	closeHandleFn = func(h syscall.Handle) error { return nil }

	ntQueryInfoProcessFn = func(handle, infoClass uintptr, buf unsafe.Pointer, bufSize uintptr, retLen *uint32) uintptr {
		return 0xC0000001
	}

	k32GetMemInfoFn = func(handle uintptr, pmc unsafe.Pointer, cb uint32) error {
		return fmt.Errorf("K32 failed")
	}

	// Act
	_, err := getProcessMemoryMBPlatform(42)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "K32GetProcessMemoryInfo(42)")
}

// ── getProcessNameAndPPIDPlatform ────────────────────────────────────

func TestGetProcessNameAndPPIDPlatform_SnapshotFail(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	createSnapshotFn = func(flags, pid uint32) (uintptr, error) {
		return 0, fmt.Errorf("snapshot failed")
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(100)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot failed")
}

func TestGetProcessNameAndPPIDPlatform_ProcessFound(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	createSnapshotFn = func(flags, pid uint32) (uintptr, error) { return 0x1234, nil }
	closeSnapshotFn = func(handle uintptr) {}

	callIdx := 0
	entries := []processEntry32W{
		makeEntry32W(10, 1, "init"),
		makeEntry32W(200, 10, "Claude.exe"),
	}

	process32FirstWFn = func(snap uintptr, entry *processEntry32W) bool {
		callIdx = 0
		*entry = entries[0]
		return true
	}
	process32NextWFn = func(snap uintptr, entry *processEntry32W) bool {
		callIdx++
		if callIdx >= len(entries) {
			return false
		}
		*entry = entries[callIdx]
		return true
	}

	// Act
	name, ppid, err := getProcessNameAndPPIDPlatform(200)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "claude.exe", name) // lowercased
	assert.Equal(t, 10, ppid)
}

func TestGetProcessNameAndPPIDPlatform_ProcessNotFound(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	createSnapshotFn = func(flags, pid uint32) (uintptr, error) { return 0x1234, nil }
	closeSnapshotFn = func(handle uintptr) {}

	process32FirstWFn = func(snap uintptr, entry *processEntry32W) bool {
		*entry = makeEntry32W(10, 1, "init")
		return true
	}
	process32NextWFn = func(snap uintptr, entry *processEntry32W) bool {
		return false // no more entries
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(999)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "process 999 not found")
}

func TestGetProcessNameAndPPIDPlatform_EmptySnapshot(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	createSnapshotFn = func(flags, pid uint32) (uintptr, error) { return 0x1234, nil }
	closeSnapshotFn = func(handle uintptr) {}

	process32FirstWFn = func(snap uintptr, entry *processEntry32W) bool {
		return false // empty snapshot
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(100)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "process 100 not found")
}

func TestGetProcessNameAndPPIDPlatform_FirstEntryMatch(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	createSnapshotFn = func(flags, pid uint32) (uintptr, error) { return 0x1234, nil }
	closeSnapshotFn = func(handle uintptr) {}

	process32FirstWFn = func(snap uintptr, entry *processEntry32W) bool {
		*entry = makeEntry32W(42, 1, "node.exe")
		return true
	}

	// Act
	name, ppid, err := getProcessNameAndPPIDPlatform(42)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "node.exe", name)
	assert.Equal(t, 1, ppid)
}

// ── Delegation: getProcessMemoryMB → getProcessMemoryMBPlatform ──────

func TestGetProcessMemoryMB_DelegationViaStub(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange — stub at syscall level
	openProcessFn = func(access uint32, inherit bool, pid uint32) (syscall.Handle, error) {
		return syscall.Handle(0xBEEF), nil
	}
	closeHandleFn = func(h syscall.Handle) error { return nil }
	ntQueryInfoProcessFn = func(handle, infoClass uintptr, buf unsafe.Pointer, bufSize uintptr, retLen *uint32) uintptr {
		counters := (*vmCountersEx2)(buf)
		counters.PrivateWorkingSetSize = 300 * 1024 * 1024
		return 0
	}

	// Act — calls getProcessMemoryMBPlatform underneath
	mb, err := getProcessMemoryMB(42)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 300.0, mb, 0.001)
}

// ── Delegation: getProcessNameAndPPID → getProcessNameAndPPIDPlatform ─

func TestGetProcessNameAndPPID_DelegationViaStub(t *testing.T) {
	defer saveAndRestoreWindowsFns()()

	// Arrange
	createSnapshotFn = func(flags, pid uint32) (uintptr, error) { return 0x1234, nil }
	closeSnapshotFn = func(handle uintptr) {}
	process32FirstWFn = func(snap uintptr, entry *processEntry32W) bool {
		*entry = makeEntry32W(77, 1, "electron.exe")
		return true
	}

	// Act — calls getProcessNameAndPPIDPlatform underneath
	name, ppid, err := getProcessNameAndPPID(77)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "electron.exe", name)
	assert.Equal(t, 1, ppid)
}

// ── helper: build a processEntry32W ──────────────────────────────────

func makeEntry32W(pid, ppid uint32, exeName string) processEntry32W {
	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))
	entry.ProcessID = pid
	entry.ParentProcessID = ppid

	runes := []rune(exeName)
	for i := 0; i < len(runes) && i < maxProcessNameLen-1; i++ {
		entry.ExeFile[i] = uint16(runes[i])
	}
	if len(runes) < maxProcessNameLen {
		entry.ExeFile[len(runes)] = 0 // null terminator
	}
	return entry
}
