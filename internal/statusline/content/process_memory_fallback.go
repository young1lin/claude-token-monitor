//go:build !windows && !darwin && !linux

package content

import "fmt"

func getProcessMemoryMBPlatform(pid int) (float64, error) {
	return 0, fmt.Errorf("unsupported platform for process memory reading")
}

func getProcessNameAndPPIDPlatform(pid int) (string, int, error) {
	return "", 0, fmt.Errorf("unsupported platform")
}
