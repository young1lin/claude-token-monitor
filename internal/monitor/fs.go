package monitor

import (
	"os"
	"path/filepath"
)

// FileSystem is an interface for file system operations to allow mocking
type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
	Walk(root string, walkFn filepath.WalkFunc) error
	Open(name string) (*os.File, error)
	MkdirAll(path string, perm os.FileMode) error
}

// OSFileSystem implements FileSystem using os package
type OSFileSystem struct{}

// Stat calls os.Stat
func (OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// ReadDir calls os.ReadDir
func (OSFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

// Walk calls filepath.Walk
func (OSFileSystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return filepath.Walk(root, walkFn)
}

// Open calls os.Open
func (OSFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// MkdirAll calls os.MkdirAll
func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
