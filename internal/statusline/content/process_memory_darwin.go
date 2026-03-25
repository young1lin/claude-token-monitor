//go:build darwin

package content

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

// darwinSysctlFn reads a sysctl value by name. Tests override this.
var darwinSysctlFn = syscall.Sysctl

func getProcessMemoryMBPlatform(pid int) (float64, error) {
	val, err := darwinSysctlFn(fmt.Sprintf("kern.proc.pid.%d.rss", pid))
	if err != nil {
		return 0, fmt.Errorf("sysctl kern.proc.pid.%d.rss: %w", pid, err)
	}

	pages, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse rss %q: %w", val, err)
	}

	// macOS page size is 4096 bytes
	return float64(pages) * 4096 / 1024 / 1024, nil
}

func getProcessNameAndPPIDPlatform(pid int) (string, int, error) {
	// kern.proc.pid.<pid> returns a struct: { struct proc { ... }; struct eproc { ppid; ... } }
	// The comm is embedded. Use sysctl to get it.
	val, err := darwinSysctlFn(fmt.Sprintf("kern.proc.pid.%d.comm", pid))
	if err != nil {
		return "", 0, fmt.Errorf("sysctl kern.proc.pid.%d.comm: %w", pid, err)
	}

	// ppid: kern.proc.pid.<pid>.ppid
	ppidVal, err := darwinSysctlFn(fmt.Sprintf("kern.proc.pid.%d.ppid", pid))
	if err != nil {
		return "", 0, fmt.Errorf("sysctl kern.proc.pid.%d.ppid: %w", pid, err)
	}

	ppid, err := strconv.Atoi(strings.TrimSpace(ppidVal))
	if err != nil {
		return "", 0, fmt.Errorf("parse ppid %q: %w", ppidVal, err)
	}

	return strings.ToLower(val), ppid, nil
}
