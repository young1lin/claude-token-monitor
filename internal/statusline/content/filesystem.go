package content

import (
	"io/fs"
	"os"
)

// FileSystem provides filesystem operations for testability.
// Tests can replace defaultFileSystem with a StubFileSystem.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
	UserHomeDir() (string, error)
}

// RealFileSystem uses actual os operations.
type RealFileSystem struct{}

// Stat wraps os.Stat.
func (f *RealFileSystem) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }

// ReadDir wraps os.ReadDir.
func (f *RealFileSystem) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }

// ReadFile wraps os.ReadFile.
func (f *RealFileSystem) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

// UserHomeDir wraps os.UserHomeDir.
func (f *RealFileSystem) UserHomeDir() (string, error) { return os.UserHomeDir() }

// defaultFileSystem is used by memory and skills collectors.
// Tests can replace this with a StubFileSystem.
var defaultFileSystem FileSystem = &RealFileSystem{}
