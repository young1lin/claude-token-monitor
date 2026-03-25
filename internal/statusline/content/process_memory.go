package content

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ProcessMemoryReader reads the parent process memory usage in MB.
// Tests override defaultProcessMemoryReader with a stub.
type ProcessMemoryReader interface {
	ReadParentMemoryMB() (float64, error)
}

// getppidFn returns the parent process ID. Tests override this.
var getppidFn = os.Getppid

// RealProcessMemoryReader reads memory via platform-specific APIs.
type RealProcessMemoryReader struct{}

// ReadParentMemoryMB returns the parent process working set in MB.
// Walks up the process tree to find Claude Code (Node.js/Electron).
func (r *RealProcessMemoryReader) ReadParentMemoryMB() (float64, error) {
	pid := getppidFn()
	if pid <= 0 {
		return 0, fmt.Errorf("invalid parent pid: %d", pid)
	}

	// Walk up the process tree to find the Claude Code process.
	// Claude Code is a Node.js/Electron app; intermediate shells may exist
	// between it and the statusline child process.
	claudePID, err := findClaudeCodePID(pid)
	if err != nil {
		return 0, err
	}
	return getProcessMemoryMBFn(claudePID)
}

// defaultProcessMemoryReader is used by ParentMemoryCollector.
// Tests can replace this with a StubProcessMemoryReader.
var defaultProcessMemoryReader ProcessMemoryReader = &RealProcessMemoryReader{}

// getProcessMemoryMBFn reads process memory in MB. Tests override this.
var getProcessMemoryMBFn = getProcessMemoryMB

// getProcessMemoryMB dispatches to the platform-specific implementation.
func getProcessMemoryMB(pid int) (float64, error) {
	return getProcessMemoryMBPlatform(pid)
}

// getProcessMemoryMBPlatform is defined in platform-specific files:
//   - process_memory_windows.go (windows)
//   - process_memory_darwin.go (darwin)
//   - process_memory_linux.go (linux)
//   - process_memory_fallback.go (all other platforms)

// findClaudeCodePID walks up the process tree from the given PID,
// looking for a process whose name contains "claude" or "node".
// Falls back to the immediate parent if no match is found (max 10 levels).
var findClaudeCodePIDFn = findClaudeCodePID

func findClaudeCodePID(startPID int) (int, error) {
	pid := startPID
	for i := 0; i < 10; i++ {
		name, ppid, err := getProcessNameAndPPIDFn(pid)
		if err != nil {
			// Can't read process info, return startPID as fallback
			return startPID, nil
		}

		lower := strings.ToLower(name)
		if strings.Contains(lower, "claude") || strings.Contains(lower, "electron") {
			return pid, nil
		}

		// Stop if we've reached the root (no parent)
		if ppid <= 0 || ppid == pid {
			return startPID, nil
		}
		pid = ppid
	}

	// Fallback: return the immediate parent
	return startPID, nil
}

// getProcessNameAndPPIDFn returns the process name and parent PID.
// Tests override this for injection.
var getProcessNameAndPPIDFn = getProcessNameAndPPID

// getProcessNameAndPPID is defined in platform-specific files.
func getProcessNameAndPPID(pid int) (string, int, error) {
	return getProcessNameAndPPIDPlatform(pid)
}

// ── Collector ─────────────────────────────────────────────────────────

// ParentMemoryCollector collects the parent process (Claude Code) memory usage.
type ParentMemoryCollector struct {
	*BaseCollector
}

// NewParentMemoryCollector creates a new parent memory collector.
func NewParentMemoryCollector() *ParentMemoryCollector {
	return &ParentMemoryCollector{
		BaseCollector: NewBaseCollector(ContentParentMemory, 5*time.Second, true),
	}
}

// Collect returns the parent process memory as "💾 123.4 MB".
func (c *ParentMemoryCollector) Collect(input interface{}, summary interface{}) (string, error) {
	mb, err := defaultProcessMemoryReader.ReadParentMemoryMB()
	if err != nil {
		return "", nil // silent fail — optional content
	}
	return formatParentMemory(mb), nil
}

// formatParentMemory formats memory usage with one decimal place.
func formatParentMemory(mb float64) string {
	return fmt.Sprintf("\U0001F4BE %.1f MB", mb)
}
