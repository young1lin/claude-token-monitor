package parser

import "github.com/young1lin/claude-token-monitor/internal/cmdrunner"

// CommandRunner / RealCommandRunner alias the shared cmdrunner package so the
// parser keeps its local names while the implementation lives in exactly one
// place (see internal/cmdrunner). Tests can replace defaultCommandRunner with
// a stub.
type CommandRunner = cmdrunner.Runner

// RealCommandRunner executes real commands via os/exec.
type RealCommandRunner = cmdrunner.Real

// defaultCommandRunner is the runner used by parser functions.
var defaultCommandRunner CommandRunner = &cmdrunner.Real{}
