package worktree

import (
	"testing"

	"github.com/kohbis/xr/internal/github"
	wt "github.com/kohbis/xr/internal/worktree"
)

func TestPRColumn(t *testing.T) {
	tests := []struct {
		name  string
		entry wt.Entry
		want  string
	}{
		{"no pull request", wt.Entry{}, "-"},
		{"lookup failed", wt.Entry{PRError: "origin is not a GitHub repository"}, "?"},
		{"open with checks", wt.Entry{PR: &github.PullRequest{Number: 12, State: github.StateOpen, Checks: github.ChecksFailing}}, "#12 open, failing"},
		{"open without checks", wt.Entry{PR: &github.PullRequest{Number: 12, State: github.StateOpen}}, "#12 open"},
		{"draft", wt.Entry{PR: &github.PullRequest{Number: 3, State: github.StateOpen, Draft: true, Checks: github.ChecksPending}}, "#3 draft, pending"},
		{"merged hides checks", wt.Entry{PR: &github.PullRequest{Number: 9, State: github.StateMerged, Checks: github.ChecksPassing}}, "#9 merged"},
		{"closed", wt.Entry{PR: &github.PullRequest{Number: 5, State: github.StateClosed}}, "#5 closed"},
	}
	for _, tt := range tests {
		if got := prColumn(tt.entry); got != tt.want {
			t.Errorf("%s: prColumn() = %q, want %q", tt.name, got, tt.want)
		}
	}
}
