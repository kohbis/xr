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
	"sync"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/output"
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
	if jobs := effectiveJobs(opts.Jobs, len(targets)); jobs > 1 {
		runConcurrent(targets, wsDir, args, opts, jobs, runs)
	} else {
		for i, repo := range targets {
			runs[i] = runRepo(repo, wsDir, args, opts, os.Stdout, os.Stderr)
		}
	}

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

// effectiveJobs resolves the worker count for n repositories.
func effectiveJobs(jobs, n int) int {
	if jobs < 1 {
		jobs = 1
	}
	if jobs > n {
		jobs = n
	}
	return jobs
}

// runConcurrent fills runs using the given number of workers. Each repository
// buffers its own output, and buffers are flushed in configuration order so the
// combined output matches a sequential run.
func runConcurrent(targets []config.Repository, wsDir string, args []string, opts Options, jobs int, runs []RepoRun) {
	type slot struct {
		out  bytes.Buffer
		err  bytes.Buffer
		done chan struct{}
	}

	slots := make([]*slot, len(targets))
	for i := range slots {
		slots[i] = &slot{done: make(chan struct{})}
	}

	queue := make(chan int)
	go func() {
		for i := range targets {
			queue <- i
		}
		close(queue)
	}()

	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				s := slots[i]
				runs[i] = runRepo(targets[i], wsDir, args, opts, &s.out, &s.err)
				close(s.done)
			}
		}()
	}

	for _, s := range slots {
		<-s.done
		// Streams stay separate so redirection keeps working; ordering between
		// them is lost within a repository, which is the cost of buffering.
		_, _ = io.Copy(os.Stdout, &s.out)
		_, _ = io.Copy(os.Stderr, &s.err)
	}
	wg.Wait()
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
