package runner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kohbis/xr/internal/config"
)

// workspace builds a config with the named repositories, creating a directory
// for each unless it is listed in missing.
func workspace(t *testing.T, names []string, missing ...string) (*config.Config, string) {
	t.Helper()
	wsDir := filepath.Join(t.TempDir(), "repos")

	repos := make([]config.Repository, 0, len(names))
	for _, name := range names {
		repos = append(repos, config.Repository{
			Name: name, Path: name, Type: config.RepoTypeClone, Source: "/x/" + name,
		})
		if !slices.Contains(missing, name) {
			if err := os.MkdirAll(filepath.Join(wsDir, name), 0755); err != nil {
				t.Fatal(err)
			}
		}
	}
	return &config.Config{Workspace: "./repos", Repositories: repos}, wsDir
}

func TestRun_NoCommand(t *testing.T) {
	cfg, wsDir := workspace(t, []string{"a"})
	if _, err := Run(cfg, wsDir, nil, Options{Quiet: true}); err == nil {
		t.Fatal("expected an error when no command is given")
	}
}

func TestRun_CapturesPerRepoOutput(t *testing.T) {
	cfg, wsDir := workspace(t, []string{"a", "b"})

	result, err := Run(cfg, wsDir, []string{"sh", "-c", `printf "%s" "$XR_REPO_NAME"`}, Options{Quiet: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Ran != 2 || result.Failed != 0 || result.Missing != 0 {
		t.Fatalf("result = %+v, want Ran=2", result)
	}
	for i, want := range []string{"a", "b"} {
		if got := result.Runs[i].Stdout; got != want {
			t.Errorf("Runs[%d].Stdout = %q, want %q", i, got, want)
		}
	}
}

func TestRun_ExitStatusAndStderr(t *testing.T) {
	cfg, wsDir := workspace(t, []string{"a"})

	result, err := Run(cfg, wsDir, []string{"sh", "-c", "echo boom >&2; exit 3"}, Options{Quiet: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	run := result.Runs[0]
	if run.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", run.ExitCode)
	}
	if !run.Failed() {
		t.Error("Failed() = false, want true")
	}
	if strings.TrimSpace(run.Stderr) != "boom" {
		t.Errorf("Stderr = %q, want %q", run.Stderr, "boom")
	}
	if result.Failed != 1 {
		t.Errorf("result.Failed = %d, want 1", result.Failed)
	}
}

func TestRun_MissingRepoIsSkippedNotFailed(t *testing.T) {
	cfg, wsDir := workspace(t, []string{"a", "gone"}, "gone")

	result, err := Run(cfg, wsDir, []string{"true"}, Options{Quiet: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Ran != 1 || result.Missing != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want Ran=1 Missing=1 Failed=0", result)
	}
	if !result.Runs[1].Missing || result.Runs[1].Failed() {
		t.Errorf("missing repo should not count as a failure: %+v", result.Runs[1])
	}
}

func TestRun_UnstartableCommand(t *testing.T) {
	cfg, wsDir := workspace(t, []string{"a"})

	result, err := Run(cfg, wsDir, []string{"xr-no-such-binary"}, Options{Quiet: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	run := result.Runs[0]
	if run.Error == "" || !run.Failed() {
		t.Fatalf("run = %+v, want a failure carrying an error", run)
	}
	if run.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", run.ExitCode)
	}
}

func TestRun_RepoFilter(t *testing.T) {
	cfg, wsDir := workspace(t, []string{"a", "b", "c"})

	result, err := Run(cfg, wsDir, []string{"true"}, Options{Quiet: true, RepoFilter: []string{"a", "c"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Runs) != 2 {
		t.Fatalf("ran in %d repos, want 2", len(result.Runs))
	}
	if result.Runs[0].Name != "a" || result.Runs[1].Name != "c" {
		t.Errorf("ran in %q and %q, want a and c", result.Runs[0].Name, result.Runs[1].Name)
	}
}

func TestEffectiveJobs(t *testing.T) {
	tests := []struct{ jobs, n, want int }{
		{jobs: 0, n: 5, want: 1},
		{jobs: -2, n: 5, want: 1},
		{jobs: 3, n: 10, want: 3},
		{jobs: 8, n: 3, want: 3},
	}
	for _, tt := range tests {
		if got := effectiveJobs(tt.jobs, tt.n); got != tt.want {
			t.Errorf("effectiveJobs(%d, %d) = %d, want %d", tt.jobs, tt.n, got, tt.want)
		}
	}
}

// TestRun_ConcurrentOutputMatchesSequential is the core guarantee of --jobs:
// concurrency must not reorder or interleave what the user sees.
func TestRun_ConcurrentOutputMatchesSequential(t *testing.T) {
	names := make([]string, 8)
	for i := range names {
		names[i] = fmt.Sprintf("repo%d", i)
	}
	cfg, wsDir := workspace(t, names, "repo5")

	// Sleep in reverse order so later repositories finish first, which would
	// scramble the output if buffers were flushed on completion.
	script := `n=${XR_REPO_NAME#repo}; sleep 0.$((9 - n)); echo "line1 $XR_REPO_NAME"; echo "line2 $XR_REPO_NAME"; [ "$n" != 3 ] || exit 2`
	args := []string{"sh", "-c", script}

	run := func(jobs int) (string, Result) {
		var result *Result
		out := captureStdout(t, func() {
			var err error
			result, err = Run(cfg, wsDir, args, Options{Jobs: jobs})
			if err != nil {
				t.Errorf("Run() error = %v", err)
			}
		})
		return out, *result
	}

	seqOut, seqResult := run(1)
	parOut, parResult := run(4)

	if parOut != seqOut {
		t.Errorf("concurrent output differs\n--- sequential ---\n%s\n--- concurrent ---\n%s", seqOut, parOut)
	}
	if parResult.Ran != seqResult.Ran || parResult.Failed != seqResult.Failed || parResult.Missing != seqResult.Missing {
		t.Errorf("concurrent result = %+v, sequential = %+v", parResult, seqResult)
	}
	if seqResult.Ran != 6 || seqResult.Failed != 1 || seqResult.Missing != 1 {
		t.Fatalf("fixture did not exercise every path: %+v", seqResult)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	os.Stdout = orig
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return <-done
}
