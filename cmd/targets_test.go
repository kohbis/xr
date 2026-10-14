package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kohbis/xr/internal/config"
)

// setupWorktreeWorkspace builds a workspace of two repositories, api and web,
// with worktrees api/feat-x, api/feat-x-followup and web/feat-x, and returns
// its config.
func setupWorktreeWorkspace(t *testing.T) *config.Config {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	for _, name := range []string{"api", "web"} {
		dir := filepath.Join(root, "repos", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "init", "-q", "-b", "main")
		runGit(t, dir, "-c", "user.email=xr@test", "-c", "user.name=xr", "commit", "-q", "--allow-empty", "-m", "init", "--no-gpg-sign")
	}
	addWorktree := func(repo, branch string) {
		runGit(t, filepath.Join(root, "repos", repo), "worktree", "add", "-q", "-b", branch, filepath.Join(root, "worktrees", repo, branch))
	}
	addWorktree("api", "feat-x")
	addWorktree("api", "feat-x-followup")
	addWorktree("web", "feat-x")

	return &config.Config{
		Path:      filepath.Join(root, "repos.yaml"),
		Workspace: "./repos",
		Worktrees: "./worktrees",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone, Tags: []string{"go"}},
			{Name: "web", Path: "web", Type: config.RepoTypeClone},
		},
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

func targetNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Repositories))
	for _, r := range cfg.Repositories {
		names = append(names, r.Name)
	}
	return names
}

func TestResolveTargets_Workspace(t *testing.T) {
	cfg := setupWorktreeWorkspace(t)

	got, wsDir, filter, err := resolveTargets(cfg, []string{"api"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Error("without --worktree the config must be passed through unchanged")
	}
	if want, _ := cfg.WorkspaceDir(); wsDir != want {
		t.Errorf("wsDir = %q, want %q", wsDir, want)
	}
	if len(filter) != 1 || filter[0] != "api" {
		t.Errorf("repo filter = %v, want [api]: the internal package applies it", filter)
	}
}

func TestResolveTargets_Worktrees(t *testing.T) {
	cfg := setupWorktreeWorkspace(t)

	tests := []struct {
		name    string
		repos   []string
		pattern string
		want    []string
	}{
		{name: "glob selects a task across repositories", pattern: "feat-x*", want: []string{"api@feat-x", "api@feat-x-followup", "web@feat-x"}},
		{name: "exact branch", pattern: "feat-x", want: []string{"api@feat-x", "web@feat-x"}},
		{name: "--repo narrows before matching", repos: []string{"web"}, pattern: "feat-x*", want: []string{"web@feat-x"}},
		{name: "no match selects nothing", pattern: "release-*", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view, wsDir, filter, err := resolveTargets(cfg, tt.repos, tt.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if wsDir != "" || filter != nil {
				t.Errorf("wsDir = %q, filter = %v, want both empty: the view already resolves them", wsDir, filter)
			}
			if got := targetNames(view); strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("targets = %v, want %v", got, tt.want)
			}
			for _, r := range view.Repositories {
				// The internal packages join wsDir and Path; with wsDir empty
				// the Path must already be the directory to run in.
				if !filepath.IsAbs(r.Path) {
					t.Errorf("%s: Path = %q, want absolute", r.Name, r.Path)
				}
				if _, err := os.Stat(filepath.Join(wsDir, r.Path, ".git")); err != nil {
					t.Errorf("%s: %s is not a worktree: %v", r.Name, r.Path, err)
				}
				if r.Branch == "" || !strings.HasSuffix(r.Name, "@"+r.Branch) {
					t.Errorf("%s: Branch = %q, want the worktree's branch", r.Name, r.Branch)
				}
			}
		})
	}

	// Tags of the parent repository travel with the view, so output that
	// reports them stays meaningful.
	view, _, _, err := resolveTargets(cfg, nil, "feat-x")
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Repositories[0].Tags; len(got) != 1 || got[0] != "go" {
		t.Errorf("api@feat-x tags = %v, want [go]", got)
	}
}

func TestResolveTargets_WorktreeErrors(t *testing.T) {
	cfg := setupWorktreeWorkspace(t)

	if _, _, _, err := resolveTargets(cfg, nil, " "); err == nil {
		t.Error("a blank pattern must be rejected rather than matching nothing silently")
	}
	if _, _, _, err := resolveTargets(cfg, nil, "feat-["); err == nil {
		t.Error("a malformed glob must be rejected")
	}
}
