//go:build darwin

package content

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── parseDarwinRSSMB ─────────────────────────────────────────────────

func TestParseDarwinRSSMB(t *testing.T) {
	tests := []struct {
		name    string
		out     []byte
		want    float64
		wantErr bool
	}{
		{"plain kb", []byte("1512"), 1512.0 / 1024, false},
		{"leading whitespace (ps pads columns)", []byte("  1512"), 1512.0 / 1024, false},
		{"trailing newline", []byte("1512\n"), 1512.0 / 1024, false},
		{"large value", []byte("2097152"), 2048.0, false}, // 2 GB
		{"empty", []byte(""), 0, true},
		{"whitespace only", []byte("   "), 0, true},
		{"non-numeric", []byte("RSS"), 0, true},
		{"zero is rejected", []byte("0"), 0, true},
		{"negative is rejected", []byte("-10"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDarwinRSSMB(tt.out)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 0.0001)
		})
	}
}

// ── parseDarwinCommPPID ──────────────────────────────────────────────

func TestParseDarwinCommPPID(t *testing.T) {
	tests := []struct {
		name     string
		out      []byte
		wantName string
		wantPPID int
		wantErr  bool
	}{
		{"simple path", []byte("/bin/zsh 1195"), "zsh", 1195, false},
		{"padded columns", []byte("  /bin/zsh  1195"), "zsh", 1195, false},
		{"trailing newline", []byte("/bin/zsh 1195\n"), "zsh", 1195, false},
		{"path with spaces", []byte("/Applications/Some App.app/Contents/MacOS/x 1"), "x", 1, false},
		{"relative name", []byte("claude 42"), "claude", 42, false},
		{"empty output", []byte(""), "", 0, true},
		{"only one field", []byte("claude"), "", 0, true},
		{"non-numeric ppid", []byte("claude notapid"), "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, ppid, err := parseDarwinCommPPID(tt.out)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantPPID, ppid)
		})
	}
}

// ── platform functions (via darwinRunnerFn seam) ─────────────────────

// withDarwinRunner swaps darwinRunnerFn for one that returns canned output
// keyed by command string, restoring it on test completion.
func withDarwinRunner(t *testing.T, fn func(name string, args ...string) ([]byte, error)) {
	t.Helper()
	old := darwinRunnerFn
	darwinRunnerFn = fn
	t.Cleanup(func() { darwinRunnerFn = old })
}

func TestGetProcessMemoryMBPlatform_Darwin_Success(t *testing.T) {
	withDarwinRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("  51200\n"), nil // 50 MB
	})

	mb, err := getProcessMemoryMBPlatform(1234)
	require.NoError(t, err)
	assert.InDelta(t, 50.0, mb, 0.001)
}

func TestGetProcessMemoryMBPlatform_Darwin_PsError(t *testing.T) {
	withDarwinRunner(t, func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("ps: not found")
	})

	_, err := getProcessMemoryMBPlatform(1234)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ps rss")
}

func TestGetProcessMemoryMBPlatform_Darwin_PassesCorrectArgs(t *testing.T) {
	var gotName string
	var gotArgs []string
	withDarwinRunner(t, func(name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args
		return []byte("1024"), nil
	})

	_, _ = getProcessMemoryMBPlatform(777)
	assert.Equal(t, "ps", gotName)
	assert.Equal(t, []string{"-o", "rss=", "-p", "777"}, gotArgs)
}

func TestGetProcessNameAndPPIDPlatform_Darwin_Success(t *testing.T) {
	withDarwinRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("/usr/local/bin/claude 4321"), nil
	})

	name, ppid, err := getProcessNameAndPPIDPlatform(1234)
	require.NoError(t, err)
	assert.Equal(t, "claude", name)
	assert.Equal(t, 4321, ppid)
}

func TestGetProcessNameAndPPIDPlatform_Darwin_PsError(t *testing.T) {
	withDarwinRunner(t, func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exec failed")
	})

	_, _, err := getProcessNameAndPPIDPlatform(1234)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ps comm/ppid")
}

func TestGetProcessNameAndPPIDPlatform_Darwin_PassesCorrectArgs(t *testing.T) {
	var gotArgs []string
	withDarwinRunner(t, func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte("/bin/zsh 1"), nil
	})

	_, _, _ = getProcessNameAndPPIDPlatform(888)
	assert.Equal(t, []string{"-o", "comm=,ppid=", "-p", "888"}, gotArgs)
}
