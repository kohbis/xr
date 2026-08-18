// Package runner executes one command in each workspace repository.
//
// The command is run directly, without a shell, so arguments reach it exactly
// as given. Pipelines and globbing need an explicit shell (for example
// `bash -c "..."`).
package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/parallel"
)

// Options configures Run.
type Options struct {
	// RepoFilter limits execution to the named repositories. Empty runs in all.
	RepoFilter []string

	// Jobs is the number of repositories running concurrently. Values below 2
	// run sequentially and stream each command's output live. Above that, output
	// is buffered per repository and flushed in configuration order, so the
	// result reads the same but appears in bursts.
	Jobs int

	// Quiet suppresses human-readable output. Each repository's output is
	// captured into its RepoRun instead, for callers rendering JSON.
	Quiet bool
}

// RepoRun is the outcome of running the command in one repository.
type RepoRun struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`

	// Missing reports that the repository is absent from the workspace, in which
	// case the command was not run.
	Missing bool `json:"missing,omitempty"`
}

// Failed reports whether the command did not succeed in this repository.
func (r RepoRun) Failed() bool {
	return !r.Missing && (r.ExitCode != 0 || r.Error != "")
}

// Result aggregates the outcome across repositories.
type Result struct {
	Runs    []RepoRun
	Ran     int
	Failed  int
	Missing int
}

// Run executes args in each configured repository under wsDir.
func Run(cfg *config.Config, wsDir string, args []string, opts Options) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("no command given")
	}

	targets := make([]config.Repository, 0, len(cfg.Repositories))
	for _, repo := range cfg.Repositories {
		if len(opts.RepoFilter) > 0 && !slices.Contains(opts.RepoFilter, repo.Name) {
			continue
		}
		targets = append(targets, repo)
	}

	runs := make([]RepoRun, len(targets))
	parallel.Run(len(targets), opts.Jobs, os.Stdout, os.Stderr,
		func(i int, out, errW io.Writer) {
			runs[i] = runRepo(targets[i], wsDir, args, opts, out, errW)
		})

	result := &Result{Runs: runs}
	for _, r := range runs {
		switch {
		case r.Missing:
			result.Missing++
		case r.Failed():
			result.Failed++
		default:
			result.Ran++
		}
	}
	return result, nil
}

// runRepo runs the command in one repository, writing human-readable progress
// and the command's own output to stdout and stderr.
func runRepo(repo config.Repository, wsDir string, args []string, opts Options, stdout, stderr io.Writer) RepoRun {
	dir := filepath.Join(wsDir, repo.Path)
	run := RepoRun{Name: repo.Name, Path: repo.Path}

	if _, err := os.Stat(dir); err != nil {
		run.Missing = true
		run.Error = fmt.Sprintf("missing in workspace: %s", dir)
		if !opts.Quiet {
			printHeader(stdout, repo.Name)
			_, _ = fmt.Fprintf(stdout, "%s\n", warn("skipped: missing in workspace (run 'xr repo sync --clone-missing')"))
		}
		return run
	}

	var outBuf, errBuf bytes.Buffer
	cmdOut, cmdErr := stdout, stderr
	if opts.Quiet {
		cmdOut, cmdErr = &outBuf, &errBuf
	} else {
		printHeader(stdout, repo.Name)
	}

	c := exec.Command(args[0], args[1:]...)
	c.Dir = dir
	c.Stdout = cmdOut
	c.Stderr = cmdErr
	c.Env = append(os.Environ(),
		"XR_REPO_NAME="+repo.Name,
		"XR_REPO_PATH="+dir,
	)

	err := c.Run()
	if opts.Quiet {
		run.Stdout = outBuf.String()
		run.Stderr = errBuf.String()
	}

	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		run.ExitCode = exitErr.ExitCode()
	default:
		// The command could not be started at all (not found, not executable).
		run.ExitCode = -1
		run.Error = err.Error()
	}

	if !opts.Quiet && run.Failed() {
		msg := fmt.Sprintf("exit status %d", run.ExitCode)
		if run.Error != "" {
			msg = run.Error
		}
		_, _ = fmt.Fprintf(stdout, "%s\n", fail(msg))
	}
	return run
}

func printHeader(w io.Writer, name string) {
	_, _ = fmt.Fprint(w, output.RepoHeader(name))
}

func warn(msg string) string {
	return output.Dim("  " + msg)
}

func fail(msg string) string {
	return output.Red("  ✗ " + msg)
}
