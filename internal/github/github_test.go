package github

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// stubGH replaces gh with fn for the duration of the test.
func stubGH(t *testing.T, fn func(args ...string) ([]byte, error)) {
	t.Helper()
	original := run
	t.Cleanup(func() { run = original })
	run = fn
}

func TestParseRemote(t *testing.T) {
	tests := []struct {
		remote string
		want   Repo
		ok     bool
	}{
		{"https://github.com/kohbis/xr", Repo{"github.com", "kohbis", "xr"}, true},
		{"https://github.com/kohbis/xr.git", Repo{"github.com", "kohbis", "xr"}, true},
		{"https://github.com/kohbis/xr/", Repo{"github.com", "kohbis", "xr"}, true},
		{"git@github.com:kohbis/xr.git", Repo{"github.com", "kohbis", "xr"}, true},
		{"ssh://git@GitHub.example.com:2222/team/app.git", Repo{"github.example.com", "team", "app"}, true},
		{"https://gitlab.com/group/sub/project.git", Repo{}, false},
		{"/home/me/src/xr", Repo{}, false},
		{"", Repo{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseRemote(tt.remote)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseRemote(%q) = %+v, %v; want %+v, %v", tt.remote, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsGitHubHost(t *testing.T) {
	calls := 0
	stubGH(t, func(args ...string) ([]byte, error) {
		calls++
		if args[len(args)-1] == "ghe.example.com" {
			return []byte("token\n"), nil
		}
		return nil, errors.New("not logged in")
	})
	t.Cleanup(func() { hostCache = map[string]bool{} })

	if !isGitHubHost("github.com") {
		t.Error("github.com must be a GitHub host")
	}
	if calls != 0 {
		t.Errorf("github.com asked gh %d time(s), want 0", calls)
	}
	for range 2 { // the second call must come from the cache
		if !isGitHubHost("ghe.example.com") {
			t.Error("a host gh has credentials for must be a GitHub host")
		}
	}
	if isGitHubHost("gitlab.com") {
		t.Error("a host gh has no credentials for must not be a GitHub host")
	}
	if calls != 2 {
		t.Errorf("gh called %d time(s), want 2 (one per unknown host)", calls)
	}
}

func TestLatestPullRequest(t *testing.T) {
	var gotArgs []string
	stubGH(t, func(args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`[
			{"number": 7, "state": "OPEN", "headRefOid": "fork", "isCrossRepository": true, "createdAt": "2026-03-01T00:00:00Z"},
			{"number": 3, "state": "CLOSED", "headRefOid": "old", "createdAt": "2026-01-01T00:00:00Z"},
			{"number": 5, "state": "MERGED", "isDraft": false, "url": "https://github.com/o/r/pull/5", "headRefOid": "abc",
			 "createdAt": "2026-02-01T00:00:00Z",
			 "statusCheckRollup": [{"status": "COMPLETED", "conclusion": "SUCCESS"}]}
		]`), nil
	})

	pr, err := LatestPullRequest(Repo{"github.com", "o", "r"}, "feat-x")
	if err != nil {
		t.Fatalf("LatestPullRequest() error = %v", err)
	}
	want := &PullRequest{Number: 5, State: StateMerged, URL: "https://github.com/o/r/pull/5", Head: "abc", Checks: ChecksPassing}
	if !reflect.DeepEqual(pr, want) {
		t.Errorf("LatestPullRequest() = %+v, want %+v (newest, fork ignored)", pr, want)
	}
	joined := strings.Join(gotArgs, " ")
	for _, part := range []string{"--repo github.com/o/r", "--head feat-x", "--state all"} {
		if !strings.Contains(joined, part) {
			t.Errorf("gh args %q missing %q", joined, part)
		}
	}
}

func TestLatestPullRequest_None(t *testing.T) {
	stubGH(t, func(args ...string) ([]byte, error) { return []byte("[]"), nil })
	pr, err := LatestPullRequest(Repo{"github.com", "o", "r"}, "feat-x")
	if err != nil || pr != nil {
		t.Errorf("LatestPullRequest() = %+v, %v; want nil, nil", pr, err)
	}
}

func TestSummarizeChecks(t *testing.T) {
	tests := []struct {
		name     string
		statuses []ghStatus
		want     string
	}{
		{"none", nil, ""},
		{"all passed", []ghStatus{{Status: "COMPLETED", Conclusion: "SUCCESS"}, {Status: "COMPLETED", Conclusion: "SKIPPED"}, {State: "SUCCESS"}}, ChecksPassing},
		{"one running", []ghStatus{{Status: "COMPLETED", Conclusion: "SUCCESS"}, {Status: "IN_PROGRESS"}}, ChecksPending},
		{"status context pending", []ghStatus{{State: "PENDING"}}, ChecksPending},
		{"failure wins over pending", []ghStatus{{Status: "QUEUED"}, {Status: "COMPLETED", Conclusion: "FAILURE"}}, ChecksFailing},
		{"status context error", []ghStatus{{State: "ERROR"}}, ChecksFailing},
	}
	for _, tt := range tests {
		if got := summarizeChecks(tt.statuses); got != tt.want {
			t.Errorf("%s: summarizeChecks() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAvailable_Disabled(t *testing.T) {
	stubGH(t, func(args ...string) ([]byte, error) {
		t.Fatalf("gh must not run while disabled, got %v", args)
		return nil, nil
	})
	original := lookPath
	t.Cleanup(func() { lookPath = original })
	lookPath = func(string) (string, error) { return "/usr/bin/gh", nil }

	for _, v := range []string{"", "0", "false", "FALSE"} {
		t.Setenv(DisableEnv, v)
		if err := Available(); err != nil {
			t.Errorf("%s=%q: Available() = %v, want nil", DisableEnv, v, err)
		}
	}
	for _, v := range []string{"1", "true", "yes"} {
		t.Setenv(DisableEnv, v)
		if err := Available(); !errors.Is(err, ErrDisabled) {
			t.Errorf("%s=%q: Available() = %v, want ErrDisabled", DisableEnv, v, err)
		}
	}

	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Setenv(DisableEnv, "")
	if err := Available(); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Available() without gh = %v, want ErrNotInstalled", err)
	}
}

func TestRunGH_Disabled(t *testing.T) {
	t.Setenv(DisableEnv, "1")
	if _, err := runGH("--version"); !errors.Is(err, ErrDisabled) {
		t.Errorf("runGH() while disabled = %v, want ErrDisabled", err)
	}
}
