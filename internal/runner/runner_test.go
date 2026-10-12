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

func TestRun_FailFast(t *testing.T) {
	tests := []struct {
		name       string
		names      []string
		missing    []string
		fail       string
		failFast   bool
		jobs       int
		quiet      bool
		wantRan    int
		wantFailed int
		wantMiss   int
		wantCancel int
		// want is the per-repository outcome in configuration order.
		// Empty means only the aggregate checks below apply.
		want []string
	}{
		{
			name:       "sequential stops after a failure",
			names:      []string{"a", "b", "c"},
			fail:       "b",
			failFast:   true,
			jobs:       1,
			wantRan:    1,
			wantFailed: 1,
			wantCancel: 1,
			want:       []string{"ok", "failed", "cancelled"},
		},
		{
			name:       "without fail-fast later repositories still run",
			names:      []string{"a", "b", "c"},
			fail:       "b",
			jobs:       1,
			quiet:      true,
			wantRan:    2,
			wantFailed: 1,
			want:       []string{"ok", "failed", "ok"},
		},
		{
			name:       "missing before a failure does not cancel",
			names:      []string{"gone", "b", "c"},
			missing:    []string{"gone"},
			fail:       "b",
			failFast:   true,
			jobs:       1,
			quiet:      true,
			wantFailed: 1,
			wantMiss:   1,
			wantCancel: 1,
			want:       []string{"missing", "failed", "cancelled"},
		},
		{
			name:     "jobs above one",
			names:    []string{"a", "b", "c", "d", "e", "f"},
			fail:     "b",
			failFast: true,
			jobs:     4,
			quiet:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, wsDir := workspace(t, tt.names, tt.missing...)
			script := fmt.Sprintf(`if [ "$XR_REPO_NAME" = %q ]; then exit 1; fi`, tt.fail)
			opts := Options{Quiet: tt.quiet, FailFast: tt.failFast, Jobs: tt.jobs}

			var result *Result
			var out string
			run := func() {
				var err error
				result, err = Run(cfg, wsDir, []string{"sh", "-c", script}, opts)
				if err != nil {
					t.Fatalf("Run() error = %v", err)
				}
			}
			if tt.quiet {
				run()
			} else {
				out = captureStdout(t, run)
			}

			if len(tt.want) > 0 {
				if result.Ran != tt.wantRan || result.Failed != tt.wantFailed || result.Missing != tt.wantMiss || result.Cancelled != tt.wantCancel {
					t.Fatalf("result = ran %d failed %d missing %d cancelled %d, want %d %d %d %d",
						result.Ran, result.Failed, result.Missing, result.Cancelled,
						tt.wantRan, tt.wantFailed, tt.wantMiss, tt.wantCancel)
				}
				if len(result.Runs) != len(tt.want) {
					t.Fatalf("len(Runs) = %d, want %d", len(result.Runs), len(tt.want))
				}
				for i, want := range tt.want {
					if got := runState(result.Runs[i]); got != want {
						t.Errorf("Runs[%d] %s = %s, want %s", i, result.Runs[i].Name, got, want)
					}
				}
			} else {
				if result.Failed < 1 {
					t.Fatalf("result.Failed = %d, want at least 1", result.Failed)
				}
				var ran, failed, missing, cancelled int
				for _, r := range result.Runs {
					switch runState(r) {
					case "ok":
						ran++
					case "failed":
						failed++
					case "missing":
						missing++
					case "cancelled":
						cancelled++
					}
				}
				if ran+failed+missing+cancelled != len(tt.names) {
					t.Fatalf("outcomes ran %d failed %d missing %d cancelled %d do not sum to %d",
						ran, failed, missing, cancelled, len(tt.names))
				}
				if ran != result.Ran || failed != result.Failed || missing != result.Missing || cancelled != result.Cancelled {
					t.Fatalf("result counts ran %d failed %d missing %d cancelled %d, outcomes %d %d %d %d",
						result.Ran, result.Failed, result.Missing, result.Cancelled, ran, failed, missing, cancelled)
				}
			}

			for _, r := range result.Runs {
				if r.Cancelled && r.Failed() {
					t.Errorf("cancelled run %s counts as failed", r.Name)
				}
				if r.Cancelled && tt.quiet && (r.Stdout != "" || r.Stderr != "") {
					t.Errorf("cancelled run %s produced output, so the command started", r.Name)
				}
				if r.Name == tt.fail && !r.Failed() {
					t.Errorf("repo %s = %+v, want a failure", tt.fail, r)
				}
			}

			if !tt.quiet && !strings.Contains(out, "skipped: not run after an earlier failure (--fail-fast)") {
				t.Errorf("output = %q, want the fail-fast skip line", out)
			}
		})
	}
}

func runState(r RepoRun) string {
	switch {
	case r.Missing:
		return "missing"
	case r.Cancelled:
		return "cancelled"
	case r.Failed():
		return "failed"
	default:
		return "ok"
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
