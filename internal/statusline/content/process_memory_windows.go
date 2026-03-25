//go:build windows

package content

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                      = syscall.NewLazyDLL("kernel32.dll")
	ntdll                         = syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")
	procK32GetProcessMemoryInfo   = kernel32.NewProc("K32GetProcessMemoryInfo")
	procCreateToolhelp32Snapshot  = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW           = kernel32.NewProc("Process32FirstW")
	procProcess32NextW            = kernel32.NewProc("Process32NextW")
	procProcess32SnapshotClose    = kernel32.NewProc("CloseHandle")
)

// ── Injectable seams for unit testing ────────────────────────────────
// Tests replace these to avoid real Windows API calls (FIRST principle).

var openProcessFn = syscall.OpenProcess
var closeHandleFn = syscall.CloseHandle

// ntQueryInfoProcessFn wraps NtQueryInformationProcess.Call.
// Signature: (handle, infoClass, buffer, bufferSize, returnLength) → (ntStatus, unused, unused)
var ntQueryInfoProcessFn = func(handle uintptr, infoClass uintptr, buf unsafe.Pointer, bufSize uintptr, retLen *uint32) uintptr {
	ret, _, _ := procNtQueryInformationProcess.Call(handle, infoClass, uintptr(buf), bufSize, uintptr(unsafe.Pointer(retLen)))
	return ret
}

// k32GetMemInfoFn wraps K32GetProcessMemoryInfo.Call.
// Returns nil on success, non-nil error on failure.
var k32GetMemInfoFn = func(handle uintptr, pmc unsafe.Pointer, cb uint32) error {
	_, _, err := procK32GetProcessMemoryInfo.Call(handle, uintptr(pmc), uintptr(cb))
	if err != nil && err != syscall.Errno(0) {
		return err
	}
	return nil
}

// createSnapshotFn wraps CreateToolhelp32Snapshot.Call.
var createSnapshotFn = func(flags, pid uint32) (uintptr, error) {
	snap, _, err := procCreateToolhelp32Snapshot.Call(uintptr(flags), uintptr(pid))
	if snap == invalidHandle || snap == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	return snap, nil
}

// closeSnapshotFn wraps CloseHandle for snapshot handles.
var closeSnapshotFn = func(handle uintptr) {
	procProcess32SnapshotClose.Call(handle)
}

// process32FirstWFn wraps Process32FirstW.Call.
var process32FirstWFn = func(snap uintptr, entry *processEntry32W) bool {
	ret, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(entry)))
	return ret != 0
}

// process32NextWFn wraps Process32NextW.Call.
var process32NextWFn = func(snap uintptr, entry *processEntry32W) bool {
	ret, _, _ := procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(entry)))
	return ret != 0
}

// PROCESS_QUERY_INFORMATION | PROCESS_VM_READ
const processQueryVM = 0x0410

const (
	th32csSnapProcess = 0x00000002
	maxProcessNameLen = 260
	invalidHandle     = ^uintptr(0)
	processVmCounters = 3 // NtQueryInformationProcess info class for VM counters
)

type processEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClass        int32
	Flags           uint32
	ExeFile         [maxProcessNameLen]uint16
}

// vmCountersEx2 maps to Windows VM_COUNTERS_EX2 (Windows 10 1607+).
// NtQueryInformationProcess with ProcessVmCounters populates PrivateWorkingSetSize
// when the buffer is large enough for VM_COUNTERS_EX2.
//
// Go's implicit struct padding ensures correct alignment on both 32-bit and 64-bit:
//   - 64-bit: 4 bytes padding after PageFaultCount (uint32) before PeakWorkingSetSize (uintptr=8)
//   - 32-bit: no padding needed (uint32 and uintptr are both 4 bytes)
type vmCountersEx2 struct {
	PeakVirtualSize            uintptr
	VirtualSize                uintptr
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
	PrivateWorkingSetSize      uintptr // matches Task Manager "Memory" column
	SharedCommitUsage          uintptr
}

// processMemoryCountersEx is the fallback struct for K32GetProcessMemoryInfo.
type processMemoryCountersEx struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func getProcessMemoryMBPlatform(pid int) (float64, error) {
	handle, err := openProcessFn(processQueryVM, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer closeHandleFn(handle)

	// Try NtQueryInformationProcess with VM_COUNTERS_EX2 for Private Working Set.
	// This matches Task Manager's "Memory" column (Private Working Set),
	// unlike K32GetProcessMemoryInfo's WorkingSetSize which includes shared DLL pages.
	var counters vmCountersEx2
	var returnLength uint32
	ret := ntQueryInfoProcessFn(
		uintptr(handle),
		uintptr(processVmCounters),
		unsafe.Pointer(&counters),
		unsafe.Sizeof(counters),
		&returnLength,
	)
	if ret == 0 && counters.PrivateWorkingSetSize > 0 {
		return float64(counters.PrivateWorkingSetSize) / 1024 / 1024, nil
	}

	// Fallback: K32GetProcessMemoryInfo for older Windows (returns total Working Set)
	var pmc processMemoryCountersEx
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	if err := k32GetMemInfoFn(uintptr(handle), unsafe.Pointer(&pmc), pmc.CB); err != nil {
		return 0, fmt.Errorf("K32GetProcessMemoryInfo(%d): %w", pid, err)
	}
	return float64(pmc.WorkingSetSize) / 1024 / 1024, nil
}

// getProcessNameAndPPIDPlatform returns the process executable name and parent PID
// using CreateToolhelp32Snapshot on Windows.
func getProcessNameAndPPIDPlatform(pid int) (string, int, error) {
	snap, err := createSnapshotFn(th32csSnapProcess, 0)
	if err != nil {
		return "", 0, err
	}
	defer closeSnapshotFn(snap)

	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	if !process32FirstWFn(snap, &entry) {
		return "", 0, fmt.Errorf("process %d not found", pid)
	}

	for {
		if entry.ProcessID == uint32(pid) {
			name := utf16ToString(entry.ExeFile[:])
			return name, int(entry.ParentProcessID), nil
		}

		entry.Size = uint32(unsafe.Sizeof(entry))
		if !process32NextWFn(snap, &entry) {
			break
		}
	}

	return "", 0, fmt.Errorf("process %d not found", pid)
}

// utf16ToString converts a null-terminated UTF-16 array to a Go string.
func utf16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			s = s[:i]
			break
		}
	}
	return strings.ToLower(filepath.Base(string(utf16.Decode(s))))
}
