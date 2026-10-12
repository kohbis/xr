package cmd

import (
	"testing"

	"github.com/kohbis/xr/internal/runner"
)

func TestExecResultCancelled(t *testing.T) {
	result := &runner.Result{
		Runs: []runner.RepoRun{
			{Name: "ok", ExitCode: 0},
			{Name: "bad", ExitCode: 2},
			{Name: "gone", Missing: true, Error: "missing in workspace"},
			{Name: "later", Cancelled: true},
		},
		Ran: 1, Failed: 1, Missing: 1, Cancelled: 1,
	}

	got := execResult([]string{"go", "test", "./..."}, result)
	if got.Summary["cancelled"] != 1 {
		t.Errorf("summary cancelled = %d, want 1", got.Summary["cancelled"])
	}
	if got.Summary["ok"] != 1 || got.Summary["failed"] != 1 || got.Summary["skipped"] != 1 {
		t.Errorf("summary = %+v, want ok 1, failed 1, skipped 1", got.Summary)
	}

	tests := []struct {
		name       string
		wantStatus string
		wantExit   bool
		exitCode   int
	}{
		{name: "ok", wantStatus: "ok", wantExit: true, exitCode: 0},
		{name: "bad", wantStatus: "failed", wantExit: true, exitCode: 2},
		{name: "gone", wantStatus: "skipped"},
		{name: "later", wantStatus: "cancelled"},
	}
	if len(got.Repos) != len(tests) {
		t.Fatalf("len(Repos) = %d, want %d", len(got.Repos), len(tests))
	}
	for i, tt := range tests {
		repo := got.Repos[i]
		if repo.Name != tt.name {
			t.Errorf("Repos[%d].Name = %q, want %q", i, repo.Name, tt.name)
		}
		if repo.Status != tt.wantStatus {
			t.Errorf("Repos[%d].Status = %q, want %q", i, repo.Status, tt.wantStatus)
		}
		_, hasExit := repo.Metrics["exit_code"]
		if hasExit != tt.wantExit {
			t.Errorf("Repos[%d] exit_code present = %v, want %v", i, hasExit, tt.wantExit)
		}
		if tt.wantExit && repo.Metrics["exit_code"] != tt.exitCode {
			t.Errorf("Repos[%d] exit_code = %d, want %d", i, repo.Metrics["exit_code"], tt.exitCode)
		}
	}
}
