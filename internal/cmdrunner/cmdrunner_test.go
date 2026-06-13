package cmdrunner

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoTestCommand returns a portable echo invocation for the host OS.
func echoTestCommand(text string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo", text}
	}
	return "echo", []string{text}
}

func TestReal_Run_EchoCommand(t *testing.T) {
	// Arrange
	runner := &Real{}
	name, args := echoTestCommand("hello")

	// Act
	out, err := runner.Run("", name, args...)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello")
}

func TestReal_Run_WithDir(t *testing.T) {
	// Arrange
	runner := &Real{}
	name, args := echoTestCommand("test")

	// Act
	out, err := runner.Run(t.TempDir(), name, args...)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, string(out), "test")
}

func TestReal_Run_NonexistentCommand(t *testing.T) {
	// Arrange
	runner := &Real{}

	// Act
	_, err := runner.Run("", "nonexistent_command_xyz_123")

	// Assert
	assert.Error(t, err)
}

func TestReal_Run_EmptyDirUsesCwd(t *testing.T) {
	// Arrange — dir="" means cmd.Dir is not set, uses current working directory
	runner := &Real{}
	name, args := echoTestCommand("no-dir")

	// Act
	out, err := runner.Run("", name, args...)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, string(out), "no-dir")
}
