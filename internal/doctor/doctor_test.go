package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubLookPath makes every tool resolvable except the named ones, so the
// missing-tool paths can be exercised on a machine that has them all.
func stubLookPath(t *testing.T, missing ...string) {
	t.Helper()
	absent := make(map[string]struct{}, len(missing))
	for _, m := range missing {
		absent[m] = struct{}{}
	}
	original := lookPath
	t.Cleanup(func() { lookPath = original })
	lookPath = func(name string) (string, error) {
		if _, ok := absent[name]; ok {
			return "", fmt.Errorf("exec: %q: executable file not found in $PATH", name)
		}
		return "/usr/bin/" + name, nil
	}
}

func checkByName(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, c := range report.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q check in report %+v", name, report.Checks)
	return Check{}
}

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun_ToolStatuses(t *testing.T) {
	tests := []struct {
		name       string
		missing    []string
		tool       string
		wantStatus string
		wantFailed bool
	}{
		{name: "git present", tool: "git", wantStatus: StatusOK},
		{name: "git missing fails", missing: []string{"git"}, tool: "git", wantStatus: StatusFailed, wantFailed: true},
		{name: "diff missing fails", missing: []string{"diff"}, tool: "diff", wantStatus: StatusFailed, wantFailed: true},
		// ripgrep is optional: search falls back to the built-in engine.
		{name: "ripgrep missing only warns", missing: []string{"rg"}, tool: "rg", wantStatus: StatusWarning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubLookPath(t, tt.missing...)
			dir := t.TempDir()
			cfgPath := writeConfig(t, dir, "workspace: ./repos\n")

			report := Run(cfgPath)

			got := checkByName(t, report, tt.tool)
			if got.Status != tt.wantStatus {
				t.Errorf("%s check = %q, want %q", tt.tool, got.Status, tt.wantStatus)
			}
			if tt.wantFailed && report.Failed == 0 {
				t.Error("a missing required tool must be counted as failed")
			}
			if !tt.wantFailed && report.Failed != 0 {
				t.Errorf("report.Failed = %d, want 0", report.Failed)
			}
		})
	}
}

func TestRun_ConfigStatuses(t *testing.T) {
	t.Run("missing config warns rather than fails", func(t *testing.T) {
		stubLookPath(t)
		report := Run(filepath.Join(t.TempDir(), "repos.yaml"))

		got := checkByName(t, report, "config")
		if got.Status != StatusWarning {
			t.Errorf("config check = %q, want %q", got.Status, StatusWarning)
		}
		if report.Failed != 0 {
			t.Errorf("report.Failed = %d, want 0: no config is the state before xr init", report.Failed)
		}
		// Without a config there is nothing to say about a workspace.
		for _, c := range report.Checks {
			if c.Name == "workspace" {
				t.Error("workspace should not be checked without a config")
			}
		}
	})

	t.Run("unreadable config fails", func(t *testing.T) {
		stubLookPath(t)
		dir := t.TempDir()
		cfgPath := writeConfig(t, dir, "workspace: [unclosed\n")

		report := Run(cfgPath)

		got := checkByName(t, report, "config")
		if got.Status != StatusFailed {
			t.Errorf("config check = %q, want %q", got.Status, StatusFailed)
		}
		if report.Failed == 0 {
			t.Error("a config that cannot be parsed must be counted as failed")
		}
	})
}

func TestRun_WorkspaceStatuses(t *testing.T) {
	cfgContent := `workspace: ./repos
repositories:
  - name: alpha
    source: https://example.com/alpha.git
  - name: bravo
    source: https://example.com/bravo.git
`

	t.Run("materialized workspace is ok", func(t *testing.T) {
		stubLookPath(t)
		dir := t.TempDir()
		cfgPath := writeConfig(t, dir, cfgContent)
		for _, name := range []string{"alpha", "bravo"} {
			if err := os.MkdirAll(filepath.Join(dir, "repos", name), 0755); err != nil {
				t.Fatal(err)
			}
		}

		got := checkByName(t, Run(cfgPath), "workspace")
		if got.Status != StatusOK {
			t.Errorf("workspace check = %q (%s), want %q", got.Status, got.Detail, StatusOK)
		}
	})

	t.Run("missing repositories warn and name themselves", func(t *testing.T) {
		stubLookPath(t)
		dir := t.TempDir()
		cfgPath := writeConfig(t, dir, cfgContent)
		if err := os.MkdirAll(filepath.Join(dir, "repos", "alpha"), 0755); err != nil {
			t.Fatal(err)
		}

		report := Run(cfgPath)
		got := checkByName(t, report, "workspace")

		if got.Status != StatusWarning {
			t.Errorf("workspace check = %q, want %q", got.Status, StatusWarning)
		}
		if !strings.Contains(got.Detail, "bravo") {
			t.Errorf("detail should name the missing repository, got %q", got.Detail)
		}
		if strings.Contains(got.Detail, "alpha") {
			t.Errorf("detail should not name a present repository, got %q", got.Detail)
		}
		// A workspace that has not been created yet must not fail the run.
		if report.Failed != 0 {
			t.Errorf("report.Failed = %d, want 0", report.Failed)
		}
	})
}

func TestJoinLimit(t *testing.T) {
	tests := []struct {
		names []string
		limit int
		want  string
	}{
		{names: []string{"a"}, limit: 5, want: "a"},
		{names: []string{"a", "b"}, limit: 5, want: "a, b"},
		{names: []string{"a", "b", "c"}, limit: 2, want: "a, b and 1 more"},
	}
	for _, tt := range tests {
		if got := joinLimit(tt.names, tt.limit); got != tt.want {
			t.Errorf("joinLimit(%v, %d) = %q, want %q", tt.names, tt.limit, got, tt.want)
		}
	}
}
