//go:build linux

package content

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── getProcessMemoryMBPlatform ───────────────────────────────────────

func TestGetProcessMemoryMBPlatform_ReadFileError(t *testing.T) {
	defer restoreFileSystem()

	// Arrange
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{},
	}

	// Act
	_, err := getProcessMemoryMBPlatform(9999)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read /proc/9999/status")
}

func TestGetProcessMemoryMBPlatform_VmRSSFound(t *testing.T) {
	defer restoreFileSystem()

	// Arrange
	status := "Name:\tbash\nVmSize:\t 123456 kB\nVmRSS:\t 51200 kB\nThreads:\t1\n"
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/1234/status": []byte(status),
		},
	}

	// Act
	mb, err := getProcessMemoryMBPlatform(1234)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 50.0, mb, 0.001)
}

func TestGetProcessMemoryMBPlatform_VmRSSNotFound(t *testing.T) {
	defer restoreFileSystem()

	// Arrange
	status := "Name:\tbash\nVmSize:\t 123456 kB\nThreads:\t1\n"
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/5678/status": []byte(status),
		},
	}

	// Act
	_, err := getProcessMemoryMBPlatform(5678)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VmRSS not found")
}

func TestGetProcessMemoryMBPlatform_VmRSSInvalidNumber(t *testing.T) {
	defer restoreFileSystem()

	// Arrange
	status := "Name:\tbash\nVmRSS:\t abc kB\n"
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/1111/status": []byte(status),
		},
	}

	// Act
	_, err := getProcessMemoryMBPlatform(1111)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VmRSS not found")
}

func TestGetProcessMemoryMBPlatform_VmRSSMissingKBField(t *testing.T) {
	defer restoreFileSystem()

	// Arrange
	status := "Name:\tbash\nVmRSS:\t\n"
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/2222/status": []byte(status),
		},
	}

	// Act
	_, err := getProcessMemoryMBPlatform(2222)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VmRSS not found")
}

func TestGetProcessMemoryMBPlatform_ZeroRSS(t *testing.T) {
	defer restoreFileSystem()

	// Arrange
	status := "Name:\tbash\nVmRSS:\t 0 kB\n"
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/3333/status": []byte(status),
		},
	}

	// Act
	mb, err := getProcessMemoryMBPlatform(3333)

	// Assert
	require.NoError(t, err)
	assert.InDelta(t, 0.0, mb, 0.001)
}

// ── getProcessNameAndPPIDPlatform ────────────────────────────────────

func TestGetProcessNameAndPPIDPlatform_ReadFileError(t *testing.T) {
	defer restoreFileSystem()

	// Arrange
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{},
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(9999)

	// Assert
	require.Error(t, err)
}

func TestGetProcessNameAndPPIDPlatform_SimpleName(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — "1234 (bash) S 1 ..."
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/1234/stat": []byte("1234 (bash) S 1 1234 1234 0 -1 4194304 123 0 0 0 0"),
		},
	}

	// Act
	name, ppid, err := getProcessNameAndPPIDPlatform(1234)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "bash", name)
	assert.Equal(t, 1, ppid)
}

func TestGetProcessNameAndPPIDPlatform_NameWithSpaces(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — comm contains spaces: "node /app/server.js"
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/5678/stat": []byte("5678 (node /app/server.js) S 1000 5678 5678 0 -1 4194304 456 0 0 0 0"),
		},
	}

	// Act
	name, ppid, err := getProcessNameAndPPIDPlatform(5678)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "server.js", name) // filepath.Base of comm
	assert.Equal(t, 1000, ppid)
}

func TestGetProcessNameAndPPIDPlatform_NameWithParens(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — comm contains parentheses: "app(test)"
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/9999/stat": []byte("9999 (app(test)) S 500 9999 9999 0 -1 4194304 789 0 0 0 0"),
		},
	}

	// Act
	name, ppid, err := getProcessNameAndPPIDPlatform(9999)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "app(test)", name) // last ')' used for comm boundary
	assert.Equal(t, 500, ppid)
}

func TestGetProcessNameAndPPIDPlatform_NoClosingParen(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — missing ')'
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/1111/stat": []byte("1111 bash S 1 1111"),
		},
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(1111)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected /proc/1111/stat format")
}

func TestGetProcessNameAndPPIDPlatform_NoOpeningParen(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — missing '('
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/2222/stat": []byte("2222 bash) S 1 2222"),
		},
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(2222)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected /proc/2222/stat format")
}

func TestGetProcessNameAndPPIDPlatform_InsufficientFieldsAfterParen(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — only state after ')', no ppid
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/3333/stat": []byte("3333 (bash) S"),
		},
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(3333)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected /proc/3333/stat format")
}

func TestGetProcessNameAndPPIDPlatform_InvalidPPID(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — ppid is not a number
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/4444/stat": []byte(fmt.Sprintf("4444 (bash) S abc 4444")),
		},
	}

	// Act
	_, _, err := getProcessNameAndPPIDPlatform(4444)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse ppid")
}

func TestGetProcessNameAndPPIDPlatform_FullPathComm(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — comm contains path separators: filepath.Base extracts last component
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/7777/stat": []byte("7777 (/usr/bin/python3) R 1 7777 7777 0 -1 4194304 100 0 0 0 0"),
		},
	}

	// Act
	name, ppid, err := getProcessNameAndPPIDPlatform(7777)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "python3", name) // filepath.Base of /usr/bin/python3
	assert.Equal(t, 1, ppid)
}

func TestGetProcessNameAndPPIDPlatform_EmptyComm(t *testing.T) {
	defer restoreFileSystem()

	// Arrange — empty comm between parens
	defaultFileSystem = &StubFileSystem{
		ReadFileReturns: map[string][]byte{
			"/proc/8888/stat": []byte("8888 () S 0 8888 8888 0 -1 4194304 10 0 0 0 0"),
		},
	}

	// Act
	name, ppid, err := getProcessNameAndPPIDPlatform(8888)

	// Assert — filepath.Base("") returns "."
	require.NoError(t, err)
	assert.Equal(t, ".", name)
	assert.Equal(t, 0, ppid)
}
