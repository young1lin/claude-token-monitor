// Package cmdrunner provides a tiny injectable wrapper around os/exec so that
// command-executing code (git lookups, `claude --version`, etc.) can be unit
// tested with a stub instead of forking real processes.
package cmdrunner

import "os/exec"

// Runner executes a command in a directory and returns its combined stdout.
// Tests swap a real Runner for a stub to avoid real process execution.
type Runner interface {
	Run(dir string, name string, args ...string) ([]byte, error)
}

// Real executes real commands via os/exec.
type Real struct{}

// Run executes the given command in the specified directory.
func (r *Real) Run(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Output()
}
