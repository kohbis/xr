package doctor

import (
	"fmt"
	"os"
	"os/exec"
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

// initGitRepo creates a git repository with one commit on branch main and an
// origin remote pointing at remote.
func initGitRepo(t *testing.T, dir, remote string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	runGit(t, dir, "config", "user.email", "xr@test")
	runGit(t, dir, "config", "user.name", "xr")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "init", "--no-gpg-sign")
	if remote != "" {
		runGit(t, dir, "remote", "add", "origin", remote)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func issueDetails(c Check) []string {
	details := make([]string, 0, len(c.Issues))
	for _, i := range c.Issues {
		details = append(details, i.Repo+": "+i.Detail)
	}
	return details
}

func TestRun_RepositoriesCheck(t *testing.T) {
	const remote = "https://example.com/alpha.git"

	t.Run("matching clone is ok", func(t *testing.T) {
		stubLookPath(t)
		dir := t.TempDir()
		cfgPath := writeConfig(t, dir, "workspace: ./repos\nrepositories:\n  - name: alpha\n    source: "+remote+"\n    branch: main\n")
		initGitRepo(t, filepath.Join(dir, "repos", "alpha"), remote)

		report := Run(cfgPath)
		got := checkByName(t, report, "repositories")
		if got.Status != StatusOK {
			t.Errorf("repositories check = %q %v, want %q", got.Status, issueDetails(got), StatusOK)
		}
		if report.Warnings != 0 {
			t.Errorf("report.Warnings = %d, want 0", report.Warnings)
		}
	})

	t.Run("nothing present is ok and does not consult git", func(t *testing.T) {
		stubLookPath(t)
		dir := t.TempDir()
		cfgPath := writeConfig(t, dir, "workspace: ./repos\nrepositories:\n  - name: alpha\n    source: "+remote+"\n")

		got := checkByName(t, Run(cfgPath), "repositories")
		if got.Status != StatusOK || len(got.Issues) != 0 {
			t.Errorf("repositories check = %+v, want ok without issues", got)
		}
	})

	tests := []struct {
		name       string
		config     string
		setup      func(t *testing.T, dir string)
		wantDetail string
	}{
		{
			name:   "directory that is not a git repository",
			config: "  - name: alpha\n    source: " + remote + "\n",
			setup: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "repos", "alpha"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			wantDetail: "not a git repository",
		},
		{
			name:   "checkout on another branch",
			config: "  - name: alpha\n    source: " + remote + "\n    branch: main\n",
			setup: func(t *testing.T, dir string) {
				repo := filepath.Join(dir, "repos", "alpha")
				initGitRepo(t, repo, remote)
				runGit(t, repo, "switch", "-c", "feature")
			},
			wantDetail: "on branch feature, repos.yaml says main",
		},
		{
			name:   "detached HEAD",
			config: "  - name: alpha\n    source: " + remote + "\n    branch: main\n",
			setup: func(t *testing.T, dir string) {
				repo := filepath.Join(dir, "repos", "alpha")
				initGitRepo(t, repo, remote)
				runGit(t, repo, "switch", "--detach")
			},
			wantDetail: "detached HEAD, repos.yaml says main",
		},
		{
			name:   "origin differs from source",
			config: "  - name: alpha\n    source: " + remote + "\n",
			setup: func(t *testing.T, dir string) {
				initGitRepo(t, filepath.Join(dir, "repos", "alpha"), "https://example.com/other.git")
			},
			wantDetail: "origin is https://example.com/other.git, source is " + remote,
		},
		{
			name:   "clone without origin",
			config: "  - name: alpha\n    source: " + remote + "\n",
			setup: func(t *testing.T, dir string) {
				initGitRepo(t, filepath.Join(dir, "repos", "alpha"), "")
			},
			wantDetail: "no origin remote",
		},
		{
			name:   "symlink pointing elsewhere",
			config: "  - name: alpha\n    source: SRC/alpha\n    type: symlink\n",
			setup: func(t *testing.T, dir string) {
				initGitRepo(t, filepath.Join(dir, "src", "alpha"), remote)
				initGitRepo(t, filepath.Join(dir, "src", "other"), remote)
				if err := os.MkdirAll(filepath.Join(dir, "repos"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "src", "other"), filepath.Join(dir, "repos", "alpha")); err != nil {
					t.Fatal(err)
				}
			},
			wantDetail: "links to ",
		},
		{
			name:   "dangling symlink",
			config: "  - name: alpha\n    source: SRC/alpha\n    type: symlink\n",
			setup: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "repos"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "src", "alpha"), filepath.Join(dir, "repos", "alpha")); err != nil {
					t.Fatal(err)
				}
			},
			wantDetail: "symlink target does not exist",
		},
		{
			name:   "symlink entry materialized as a directory",
			config: "  - name: alpha\n    source: SRC/alpha\n    type: symlink\n",
			setup: func(t *testing.T, dir string) {
				initGitRepo(t, filepath.Join(dir, "repos", "alpha"), remote)
			},
			wantDetail: "is a directory, but type is symlink",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubLookPath(t)
			dir := t.TempDir()
			tt.setup(t, dir)
			cfg := strings.ReplaceAll(tt.config, "SRC", filepath.Join(dir, "src"))
			cfgPath := writeConfig(t, dir, "workspace: ./repos\nrepositories:\n"+cfg)

			report := Run(cfgPath)
			got := checkByName(t, report, "repositories")

			if got.Status != StatusWarning {
				t.Errorf("repositories check = %q, want %q", got.Status, StatusWarning)
			}
			if !strings.Contains(strings.Join(issueDetails(got), "\n"), tt.wantDetail) {
				t.Errorf("issues = %v, want one containing %q", issueDetails(got), tt.wantDetail)
			}
			for _, i := range got.Issues {
				if i.Repo != "alpha" {
					t.Errorf("issue for %q, want alpha", i.Repo)
				}
			}
			// Drift is reported, not failed: commands still work on the repository.
			if report.Failed != 0 {
				t.Errorf("report.Failed = %d, want 0", report.Failed)
			}
		})
	}
}

func TestRun_RepositoriesCheck_SymlinkMatches(t *testing.T) {
	const remote = "https://example.com/alpha.git"
	stubLookPath(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "alpha")
	initGitRepo(t, src, remote)
	if err := os.MkdirAll(filepath.Join(dir, "repos"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, filepath.Join(dir, "repos", "alpha")); err != nil {
		t.Fatal(err)
	}
	cfgPath := writeConfig(t, dir, "workspace: ./repos\nrepositories:\n  - name: alpha\n    source: "+src+"\n    branch: main\n")

	got := checkByName(t, Run(cfgPath), "repositories")
	if got.Status != StatusOK {
		t.Errorf("repositories check = %q %v, want %q", got.Status, issueDetails(got), StatusOK)
	}
}

func TestSameRemote(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://example.com/a.git", "https://example.com/a", true},
		{"https://example.com/a/", "https://example.com/a", true},
		{" https://example.com/a.git\n", "https://example.com/a", true},
		{"git@example.com:o/a.git", "https://example.com/o/a.git", false},
		{"https://example.com/a", "https://example.com/b", false},
	}
	for _, tt := range tests {
		if got := sameRemote(tt.a, tt.b); got != tt.want {
			t.Errorf("sameRemote(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
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
