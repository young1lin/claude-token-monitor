//go:build darwin

package content

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/young1lin/claude-token-monitor/internal/cmdrunner"
)

// defaultDarwinRunner runs `ps` to read process info. Tests replace it with a
// stub to avoid forking a real binary (FIRST principle).
var defaultDarwinRunner cmdrunner.Runner = &cmdrunner.Real{}

// darwinRunnerFn is the seam the platform functions call. Tests override it
// to feed canned `ps` output without executing anything.
var darwinRunnerFn = func(name string, args ...string) ([]byte, error) {
	return defaultDarwinRunner.Run("", name, args...)
}

// getProcessMemoryMBPlatform reads resident memory (MB) via `ps -o rss= -p <pid>`.
//
// `ps` reports RSS already normalized by the kernel page size, so this works on
// both Intel (4K pages) and Apple Silicon (16K pages) without hardcoding a page
// size — unlike the previous sysctl approach which also used non-existent OIDs
// (kern.proc.pid.<pid>.rss does not exist on macOS).
func getProcessMemoryMBPlatform(pid int) (float64, error) {
	out, err := darwinRunnerFn("ps", "-o", "rss=", "-p", strconv.Itoa(pid))
	if err != nil {
		return 0, fmt.Errorf("ps rss for pid %d: %w", pid, err)
	}
	return parseDarwinRSSMB(out)
}

// parseDarwinRSSMB parses `ps -o rss=` output (kilobytes, possibly with leading
// whitespace) into megabytes. Separated from the runner call so it can be unit
// tested with canned input — no real `ps` invocation.
func parseDarwinRSSMB(out []byte) (float64, error) {
	kb, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse ps rss %q: %w", out, err)
	}
	if kb <= 0 {
		return 0, fmt.Errorf("ps rss returned non-positive value %q", out)
	}
	return float64(kb) / 1024.0, nil
}

// getProcessNameAndPPIDPlatform returns the process name and parent PID via
// `ps -o comm=,ppid= -p <pid>`.
func getProcessNameAndPPIDPlatform(pid int) (string, int, error) {
	out, err := darwinRunnerFn("ps", "-o", "comm=,ppid=", "-p", strconv.Itoa(pid))
	if err != nil {
		return "", 0, fmt.Errorf("ps comm/ppid for pid %d: %w", pid, err)
	}
	return parseDarwinCommPPID(out)
}

// parseDarwinCommPPID parses `ps -o comm=,ppid=` output.
//
// Output shape: "<comm> <ppid>" where comm may contain spaces (e.g. a path like
// "/Applications/Some App.app/Contents/MacOS/app"). ppid is always the final
// whitespace-separated field.
func parseDarwinCommPPID(out []byte) (string, int, error) {
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", 0, fmt.Errorf("ps returned empty output")
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", 0, fmt.Errorf("unexpected ps output %q: need at least comm and ppid", line)
	}
	ppid, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return "", 0, fmt.Errorf("parse ppid %q: %w", fields[len(fields)-1], err)
	}
	comm := strings.Join(fields[:len(fields)-1], " ")
	return strings.ToLower(filepath.Base(comm)), ppid, nil
}
