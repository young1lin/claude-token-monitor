package parser

import (
	"errors"
	"strings"
	"testing"
)

// stubCommandRunner returns canned responses for unit tests.
type stubCommandRunner struct {
	outputs map[string][]byte
	errors  map[string]error
}

func (s *stubCommandRunner) Run(dir, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if err, ok := s.errors[key]; ok {
		return nil, err
	}
	if out, ok := s.outputs[key]; ok {
		return out, nil
	}
	return nil, errors.New("stub: unknown command: " + key)
}

func restoreParserRunner() {
	defaultCommandRunner = &RealCommandRunner{}
}

func TestGetGitBranchForPath_SymbolicRef(t *testing.T) {
	defer restoreParserRunner()
	defaultCommandRunner = &stubCommandRunner{
		outputs: map[string][]byte{
			"git symbolic-ref --short HEAD": []byte("main\n"),
		},
	}

	branch := getGitBranchForPath("/project")
	if branch != "main" {
		t.Errorf("expected %q, got %q", "main", branch)
	}
}

func TestGetGitBranchForPath_FallbackToRevParse(t *testing.T) {
	defer restoreParserRunner()
	defaultCommandRunner = &stubCommandRunner{
		errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("detached"),
		},
		outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD": []byte("feature/test\n"),
		},
	}

	branch := getGitBranchForPath("/project")
	if branch != "feature/test" {
		t.Errorf("expected %q, got %q", "feature/test", branch)
	}
}

func TestGetGitBranchForPath_DetachedHead(t *testing.T) {
	defer restoreParserRunner()
	defaultCommandRunner = &stubCommandRunner{
		errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("detached"),
		},
		outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD": []byte("HEAD\n"),
			"git status --porcelain":          []byte(""),
		},
	}

	branch := getGitBranchForPath("/project")
	if branch != "(empty)" {
		t.Errorf("expected %q, got %q", "(empty)", branch)
	}
}

func TestGetGitBranchForPath_OriginHeadFallback(t *testing.T) {
	defer restoreParserRunner()
	defaultCommandRunner = &stubCommandRunner{
		errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("detached"),
		},
		outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD":        []byte("HEAD\n"),
			"git status --porcelain":                 []byte(""),
			"git rev-parse --abbrev-ref origin/HEAD": []byte("origin/develop\n"),
		},
	}

	branch := getGitBranchForPath("/project")
	if branch != "develop" {
		t.Errorf("expected %q, got %q", "develop", branch)
	}
}

func TestGetGitBranchForPath_DetachedHeadNoGitStatus(t *testing.T) {
	defer restoreParserRunner()
	defaultCommandRunner = &stubCommandRunner{
		errors: map[string]error{
			"git symbolic-ref --short HEAD": errors.New("detached"),
			"git status --porcelain":        errors.New("not a repo"),
		},
		outputs: map[string][]byte{
			"git rev-parse --abbrev-ref HEAD": []byte("HEAD\n"),
		},
	}

	branch := getGitBranchForPath("/project")
	if branch != "" {
		t.Errorf("expected empty, got %q", branch)
	}
}

func TestGetGitBranchForPath_AllFail(t *testing.T) {
	defer restoreParserRunner()
	defaultCommandRunner = &stubCommandRunner{
		errors: map[string]error{
			"git symbolic-ref --short HEAD":   errors.New("fail"),
			"git rev-parse --abbrev-ref HEAD": errors.New("fail"),
		},
	}

	branch := getGitBranchForPath("/project")
	if branch != "" {
		t.Errorf("expected empty, got %q", branch)
	}
}

func TestGetGitBranchForPath_EmptyPath(t *testing.T) {
	defer restoreParserRunner()
	defaultCommandRunner = &stubCommandRunner{}

	branch := getGitBranchForPath("")
	if branch != "" {
		t.Errorf("expected empty, got %q", branch)
	}
}
