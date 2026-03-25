//go:build linux

package content

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func getProcessMemoryMBPlatform(pid int) (float64, error) {
	data, err := defaultFileSystem.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, fmt.Errorf("read /proc/%d/status: %w", pid, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				continue
			}
			return float64(kb) / 1024, nil
		}
	}

	return 0, fmt.Errorf("VmRSS not found in /proc/%d/status", pid)
}

func getProcessNameAndPPIDPlatform(pid int) (string, int, error) {
	stat, err := defaultFileSystem.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", 0, err
	}

	// Format: pid (comm) state ppid ...
	// comm can contain spaces and parentheses, so find the last ')' first.
	fields := strings.SplitN(string(stat), " ", 3)
	if len(fields) < 3 {
		return "", 0, fmt.Errorf("unexpected /proc/%d/stat format", pid)
	}

	// Extract from "(comm) state ppid ..."
	rest := fields[2]
	rpIdx := strings.Index(rest, ") ")
	if rpIdx < 0 {
		return "", 0, fmt.Errorf("unexpected /proc/%d/stat format", pid)
	}

	comm := rest[:rpIdx]
	afterRP := rest[rpIdx+2:]
	afterFields := strings.SplitN(afterRP, " ", 4)
	if len(afterFields) < 2 {
		return "", 0, fmt.Errorf("unexpected /proc/%d/stat format", pid)
	}

	ppid, err := strconv.Atoi(afterFields[1])
	if err != nil {
		return "", 0, fmt.Errorf("parse ppid from /proc/%d/stat: %w", pid, err)
	}

	name := strings.ToLower(filepath.Base(comm))
	return name, ppid, nil
}
