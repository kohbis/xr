package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/github"
)

func testConfig() *config.Config {
	return &config.Config{
		Workspace: "./repos",
		Worktrees: "./worktrees",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone, Branch: "main"},
			{Name: "web", Path: "nested/web", Type: config.RepoTypeClone},
		},
	}
}

func TestPathFor(t *testing.T) {
	m := New("/ws", testConfig())

	tests := []struct {
		name   string
		repo   config.Repository
		branch string
		want   string
	}{
		{"simple branch", m.Config.Repositories[0], "feat-x", filepath.Join("/ws", "worktrees", "api", "feat-x")},
		{"nested branch", m.Config.Repositories[0], "feature/foo", filepath.Join("/ws", "worktrees", "api", "feature", "foo")},
		{"nested repo path", m.Config.Repositories[1], "feat-x", filepath.Join("/ws", "worktrees", "nested", "web", "feat-x")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.PathFor(tt.repo, tt.branch); got != tt.want {
				t.Errorf("PathFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectRepos(t *testing.T) {
	m := New("/ws", testConfig())

	all, err := m.SelectRepos(nil)
	if err != nil {
		t.Fatalf("SelectRepos(nil) error = %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len(SelectRepos(nil)) = %d, want 2", len(all))
	}

	one, err := m.SelectRepos([]string{"web"})
	if err != nil {
		t.Fatalf("SelectRepos([web]) error = %v", err)
	}
	if len(one) != 1 || one[0].Name != "web" {
		t.Errorf("SelectRepos([web]) = %+v, want [web]", one)
	}

	if _, err := m.SelectRepos([]string{"nope"}); err == nil {
		t.Error("SelectRepos([nope]) error = nil, want error")
	}

	dup, err := m.SelectRepos([]string{"api", "api"})
	if err != nil {
		t.Fatalf("SelectRepos([api api]) error = %v", err)
	}
	if len(dup) != 1 {
		t.Errorf("len(SelectRepos([api api])) = %d, want 1", len(dup))
	}
}

func TestValidateBranch(t *testing.T) {
	tests := []struct {
		branch  string
		wantErr bool
	}{
		{"feat-x", false},
		{"feature/foo", false},
		{"release/2024/01", false},
		{"", true},
		{"   ", true},
		{"../escape", true},
		{"feature/../../escape", true},
		{"/absolute", true},
		{"~/home", true},
		{"trailing/", true},
		{"-force", true},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			err := validateBranch(tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBranch(%q) error = %v, wantErr = %v", tt.branch, err, tt.wantErr)
			}
		})
	}
}

func TestMatchBranch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		branch  string
		want    bool
		wantErr bool
	}{
		{"empty pattern matches all", "", "feat-x", true, false},
		{"empty pattern matches detached", "", "", true, false},
		{"exact match", "feat-x", "feat-x", true, false},
		{"glob match", "feat-x*", "feat-x-part2", true, false},
		{"no match", "feat-x*", "bugfix", false, false},
		{"pattern skips detached", "feat-x*", "", false, false},
		{"invalid pattern", "[", "feat-x", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchBranch(tt.pattern, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Fatalf("matchBranch() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("matchBranch(%q, %q) = %v, want %v", tt.pattern, tt.branch, got, tt.want)
			}
		})
	}
}

func TestList_InvalidPatternIsReportedWithoutWorktrees(t *testing.T) {
	m := New("/ws", testConfig())

	// No repository exists on disk, so nothing can match — the malformed
	// pattern must still be reported instead of silently returning nothing.
	if _, err := m.List(m.Config.Repositories, "["); err == nil {
		t.Error("List() with malformed pattern error = nil, want error")
	}
}

func TestRemoveEmptyDirs(t *testing.T) {
	root := t.TempDir()
	leaf := filepath.Join(root, "api", "feature", "foo")
	if err := os.MkdirAll(leaf, 0755); err != nil {
		t.Fatal(err)
	}

	removeEmptyDirs(root, leaf)

	if _, err := os.Stat(filepath.Join(root, "api")); !os.IsNotExist(err) {
		t.Errorf("empty parents were not removed: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Errorf("root was removed: %v", err)
	}
}

func TestRemoveEmptyDirs_StopsAtNonEmpty(t *testing.T) {
	root := t.TempDir()
	leaf := filepath.Join(root, "api", "feature", "foo")
	if err := os.MkdirAll(leaf, 0755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(root, "api", "feature", "bar")
	if err := os.MkdirAll(sibling, 0755); err != nil {
		t.Fatal(err)
	}

	removeEmptyDirs(root, leaf)

	if _, err := os.Stat(sibling); err != nil {
		t.Errorf("sibling worktree directory was removed: %v", err)
	}
	if _, err := os.Stat(leaf); !os.IsNotExist(err) {
		t.Errorf("empty leaf was not removed: %v", err)
	}
}

// --- integration tests against a real git repository ---

func setupRepo(t *testing.T) (*Manager, config.Repository) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	root := t.TempDir()
	initGitRepo(t, filepath.Join(root, "repos", "api"))

	cfg := &config.Config{
		Workspace: "./repos",
		Worktrees: "./worktrees",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone},
		},
	}
	return New(root, cfg), cfg.Repositories[0]
}

// initGitRepo creates a git repository with one commit at dir.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init")
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
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func TestAddListRemove(t *testing.T) {
	m, repo := setupRepo(t)
	repos := []config.Repository{repo}

	result, err := m.Add("feature/foo", repos, AddOptions{Create: true})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := result.Outcomes[0]; got.Status != StatusCreated {
		t.Fatalf("Add() outcome = %+v, want status %q", got, StatusCreated)
	}

	wantPath := m.PathFor(repo, "feature/foo")
	if _, err := os.Stat(filepath.Join(wantPath, "f.txt")); err != nil {
		t.Fatalf("worktree was not checked out at %s: %v", wantPath, err)
	}

	entries, err := m.List(repos, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(List()) = %d, want 1 (main worktree must be excluded)", len(entries))
	}
	if entries[0].Branch != "feature/foo" || entries[0].Repo != "api" {
		t.Errorf("List() = %+v, want api/feature/foo", entries[0])
	}

	filtered, err := m.List(repos, "feature/*")
	if err != nil {
		t.Fatalf("List(pattern) error = %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("len(List(\"feature/*\")) = %d, want 1", len(filtered))
	}
	none, err := m.List(repos, "bugfix*")
	if err != nil {
		t.Fatalf("List(pattern) error = %v", err)
	}
	if len(none) != 0 {
		t.Errorf("len(List(\"bugfix*\")) = %d, want 0", len(none))
	}

	removeResult, err := m.Remove(entries, false, false)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if got := removeResult.Outcomes[0]; got.Status != StatusRemoved {
		t.Fatalf("Remove() outcome = %+v, want status %q", got, StatusRemoved)
	}
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory still present: %v", err)
	}
	// The intermediate "feature" directory of the nested branch must be gone too.
	if _, err := os.Stat(filepath.Join(m.WorktreesDir(), "api")); !os.IsNotExist(err) {
		t.Errorf("empty parent directories were not cleaned up: %v", err)
	}
}

func TestAdd_MissingBranchWithoutCreate(t *testing.T) {
	m, repo := setupRepo(t)

	result, err := m.Add("feat-x", []config.Repository{repo}, AddOptions{})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	outcome := result.Outcomes[0]
	if outcome.Status != StatusFailed {
		t.Fatalf("Add() outcome = %+v, want status %q", outcome, StatusFailed)
	}
	if _, err := os.Stat(m.WorktreesDir()); !os.IsNotExist(err) {
		t.Errorf("worktrees directory was created for a failed add: %v", err)
	}
}

func TestAdd_ExistingLocalBranch(t *testing.T) {
	m, repo := setupRepo(t)
	dir, err := m.RepoDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "branch", "feat-x")

	result, err := m.Add("feat-x", []config.Repository{repo}, AddOptions{})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := result.Outcomes[0]; got.Status != StatusCreated {
		t.Fatalf("Add() outcome = %+v, want status %q", got, StatusCreated)
	}
}

func TestAdd_DryRunAndDuplicate(t *testing.T) {
	m, repo := setupRepo(t)
	repos := []config.Repository{repo}

	result, err := m.Add("feat-x", repos, AddOptions{Create: true, DryRun: true})
	if err != nil {
		t.Fatalf("Add(dry-run) error = %v", err)
	}
	if got := result.Outcomes[0]; got.Status != StatusPreview {
		t.Fatalf("Add(dry-run) outcome = %+v, want status %q", got, StatusPreview)
	}
	if _, err := os.Stat(m.PathFor(repo, "feat-x")); !os.IsNotExist(err) {
		t.Fatalf("dry run created a worktree: %v", err)
	}

	if _, err := m.Add("feat-x", repos, AddOptions{Create: true}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	again, err := m.Add("feat-x", repos, AddOptions{Create: true})
	if err != nil {
		t.Fatalf("Add() second call error = %v", err)
	}
	if got := again.Outcomes[0]; got.Status != StatusSkipped {
		t.Errorf("Add() on existing worktree outcome = %+v, want status %q", got, StatusSkipped)
	}
}

func TestAdd_InvalidBranch(t *testing.T) {
	m, repo := setupRepo(t)

	if _, err := m.Add("../escape", []config.Repository{repo}, AddOptions{Create: true}); err == nil {
		t.Error("Add(\"../escape\") error = nil, want error")
	}
}

func TestPrune(t *testing.T) {
	m, repo := setupRepo(t)
	repos := []config.Repository{repo}

	if _, err := m.Add("feat-x", repos, AddOptions{Create: true}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	// Delete the checkout behind git's back, leaving a stale admin entry.
	if err := os.RemoveAll(m.PathFor(repo, "feat-x")); err != nil {
		t.Fatal(err)
	}

	entries, err := m.List(repos, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(List()) = %d, want 1 stale entry", len(entries))
	}

	// A dry run reports the stale entry without dropping it.
	preview, err := m.Prune(repos, true)
	if err != nil {
		t.Fatalf("Prune(dry-run) error = %v", err)
	}
	if got := preview.Outcomes[0]; got.Status != StatusPreview {
		t.Fatalf("Prune(dry-run) outcome = %+v, want status %q", got, StatusPreview)
	}
	if entries, err := m.List(repos, ""); err != nil || len(entries) != 1 {
		t.Fatalf("dry run pruned the entry: entries = %v, err = %v", entries, err)
	}

	result, err := m.Prune(repos, false)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if got := result.Outcomes[0]; got.Status != StatusPruned {
		t.Fatalf("Prune() outcome = %+v, want status %q", got, StatusPruned)
	}

	entries, err = m.List(repos, "")
	if err != nil {
		t.Fatalf("List() after prune error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(List()) after prune = %d, want 0", len(entries))
	}

	// Nothing left to prune must not be reported as a change.
	again, err := m.Prune(repos, false)
	if err != nil {
		t.Fatalf("Prune() second call error = %v", err)
	}
	if got := again.Outcomes[0]; got.Status != StatusSkipped {
		t.Errorf("Prune() with nothing stale outcome = %+v, want status %q", got, StatusSkipped)
	}
}

// The branch name is valid, so validateBranch lets it through and the
// containment check is what has to stop this before git creates anything.
func TestAdd_RepoPathEscapesWorktreesDir(t *testing.T) {
	m, _ := setupRepo(t)

	// repos/../escape is a real repository, so RepoDir resolves and the add
	// gets as far as the check; worktrees/../escape is outside the root.
	initGitRepo(t, filepath.Join(m.Root, "escape"))
	repo := config.Repository{Name: "escape", Path: "../escape", Type: config.RepoTypeClone}

	target := m.PathFor(repo, "feat-x")
	result, err := m.Add("feat-x", []config.Repository{repo}, AddOptions{Create: true})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := result.Outcomes[0]; got.Status != StatusFailed {
		t.Fatalf("Add() outcome = %+v, want status %q", got, StatusFailed)
	}
	if _, err := os.Stat(target); err == nil {
		t.Errorf("worktree created outside the worktrees directory at %s", target)
	}
}

// Walking upwards must stop at the root, not continue into an empty sibling.
func TestRemoveEmptyDirs_StopsOutsideRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "worktrees")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	removeEmptyDirs(root, outside)

	if _, err := os.Stat(outside); err != nil {
		t.Errorf("removeEmptyDirs removed %s, which is outside the root: %v", outside, err)
	}
}

// A symlinked parent must not let a worktree land outside the root.
func TestAdd_SymlinkedParentEscapesWorktreesDir(t *testing.T) {
	m, repo := setupRepo(t)
	outside := t.TempDir()
	if err := os.MkdirAll(m.WorktreesDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(m.WorktreesDir(), repo.Path)); err != nil {
		t.Fatal(err)
	}

	result, err := m.Add("feat-x", []config.Repository{repo}, AddOptions{Create: true})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := result.Outcomes[0]; got.Status != StatusFailed {
		t.Fatalf("Add() outcome = %+v, want status %q", got, StatusFailed)
	}
	if _, err := os.Stat(filepath.Join(outside, "feat-x")); err == nil {
		t.Error("worktree created outside the worktrees directory")
	}
}

// setupClonedRepo clones a bare remote with one commit on main into
// <root>/repos/api, so the workspace repository has origin/main.
func setupClonedRepo(t *testing.T) (*Manager, config.Repository, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	initGitRepo(t, seed)
	runGit(t, seed, "branch", "-M", "main")
	remote := filepath.Join(root, "remote.git")
	runGit(t, root, "clone", "-q", "--bare", seed, remote)

	clone := filepath.Join(root, "repos", "api")
	runGit(t, root, "clone", "-q", remote, clone)
	runGit(t, clone, "config", "user.email", "xr@test")
	runGit(t, clone, "config", "user.name", "xr")
	runGit(t, clone, "config", "commit.gpgsign", "false")

	cfg := &config.Config{
		Workspace: "./repos",
		Worktrees: "./worktrees",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone, Branch: "main"},
		},
	}
	return New(root, cfg), cfg.Repositories[0], clone
}

func commitFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-q", "-m", name)
}

func TestMergedEntries(t *testing.T) {
	m, repo, clone := setupClonedRepo(t)
	repos := []config.Repository{repo}

	for _, branch := range []string{"feat-merged", "feat-wip", "feat-fresh"} {
		if _, err := m.Add(branch, repos, AddOptions{Create: true}); err != nil {
			t.Fatalf("Add(%s) error = %v", branch, err)
		}
	}
	commitFile(t, m.PathFor(repo, "feat-merged"), "merged.txt")
	commitFile(t, m.PathFor(repo, "feat-wip"), "wip.txt")

	// Squash-merge feat-merged on the remote side, then fetch it.
	runGit(t, clone, "merge", "-q", "--squash", "feat-merged")
	runGit(t, clone, "commit", "-q", "-m", "feat-merged (#1)")
	runGit(t, clone, "push", "-q", "origin", "main")
	runGit(t, clone, "fetch", "-q", "origin")

	merged, skipped, err := m.MergedEntries(repos)
	if err != nil {
		t.Fatalf("MergedEntries() error = %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %+v, want none", skipped)
	}
	if len(merged) != 1 || merged[0].Branch != "feat-merged" {
		t.Errorf("MergedEntries() = %+v, want only feat-merged", merged)
	}
}

func TestMergedEntries_NoOriginIsSkipped(t *testing.T) {
	m, repo := setupRepo(t)
	repos := []config.Repository{repo}
	if _, err := m.Add("feat-x", repos, AddOptions{Create: true}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	merged, skipped, err := m.MergedEntries(repos)
	if err != nil {
		t.Fatalf("MergedEntries() error = %v", err)
	}
	if len(merged) != 0 {
		t.Errorf("merged = %+v, want none", merged)
	}
	if len(skipped) != 1 || skipped[0].Status != StatusSkipped || skipped[0].Repo != "api" {
		t.Errorf("skipped = %+v, want one skipped outcome for api", skipped)
	}
}

func TestAttachPullRequests(t *testing.T) {
	m, repo := setupRepo(t)
	repos := []config.Repository{repo}
	for _, b := range []string{"feat-pr", "feat-none", "feat-err"} {
		if _, err := m.Add(b, repos, AddOptions{Create: true}); err != nil {
			t.Fatalf("Add(%s) error = %v", b, err)
		}
	}

	m.FindPullRequest = func(repoDir, branch string) (*github.PullRequest, error) {
		switch branch {
		case "feat-pr":
			return &github.PullRequest{Number: 4, State: github.StateOpen}, nil
		case "feat-err":
			return nil, github.ErrNotGitHub
		}
		return nil, nil
	}

	entries, err := m.List(repos, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if err := m.AttachPullRequests(entries); err != nil {
		t.Fatalf("AttachPullRequests() error = %v", err)
	}

	byBranch := map[string]Entry{}
	for _, e := range entries {
		byBranch[e.Branch] = e
	}
	if e := byBranch["feat-pr"]; e.PR == nil || e.PR.Number != 4 || e.PRError != "" {
		t.Errorf("feat-pr = %+v, want PR #4", e)
	}
	if e := byBranch["feat-none"]; e.PR != nil || e.PRError != "" {
		t.Errorf("feat-none = %+v, want no PR and no error", e)
	}
	if e := byBranch["feat-err"]; e.PR != nil || e.PRError == "" {
		t.Errorf("feat-err = %+v, want PRError", e)
	}
}

func TestAttachPullRequests_Disabled(t *testing.T) {
	m, _ := setupRepo(t)
	t.Setenv(github.DisableEnv, "1")
	if err := m.AttachPullRequests(nil); !errors.Is(err, github.ErrDisabled) {
		t.Errorf("AttachPullRequests() with %s set = %v, want ErrDisabled", github.DisableEnv, err)
	}
}

func TestClosedEntries(t *testing.T) {
	m, repo := setupRepo(t)
	repos := []config.Repository{repo}
	branches := []string{"feat-closed", "feat-closed-moved", "feat-open", "feat-merged", "feat-none", "feat-err"}
	for _, b := range branches {
		if _, err := m.Add(b, repos, AddOptions{Create: true}); err != nil {
			t.Fatalf("Add(%s) error = %v", b, err)
		}
	}

	entries, err := m.List(repos, "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	head := map[string]string{}
	for _, e := range entries {
		head[e.Branch] = e.Head
	}

	m.FindPullRequest = func(_, branch string) (*github.PullRequest, error) {
		switch branch {
		case "feat-closed":
			return &github.PullRequest{Number: 1, State: github.StateClosed, Head: head[branch]}, nil
		case "feat-closed-moved":
			return &github.PullRequest{Number: 2, State: github.StateClosed, Head: "0000000"}, nil
		case "feat-open":
			return &github.PullRequest{Number: 3, State: github.StateOpen, Head: head[branch]}, nil
		case "feat-merged":
			return &github.PullRequest{Number: 4, State: github.StateMerged, Head: head[branch]}, nil
		case "feat-err":
			return nil, github.ErrNotGitHub
		}
		return nil, nil
	}

	closed, skipped, err := m.ClosedEntries(repos)
	if err != nil {
		t.Fatalf("ClosedEntries() error = %v", err)
	}
	if len(closed) != 1 || closed[0].Branch != "feat-closed" {
		t.Errorf("ClosedEntries() = %+v, want only feat-closed", closed)
	}
	if len(skipped) != 1 || skipped[0].Status != StatusSkipped {
		t.Errorf("skipped = %+v, want one skipped outcome for the failed lookup", skipped)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return strings.TrimSpace(string(out))
}

func TestAdd_CreateStartsFromOrigin(t *testing.T) {
	m, repo, clone := setupClonedRepo(t)
	repos := []config.Repository{repo}

	// origin/main moves ahead of the local main, as after a fetch.
	runGit(t, clone, "checkout", "-q", "-b", "tmp")
	commitFile(t, clone, "upstream.txt")
	runGit(t, clone, "update-ref", "refs/remotes/origin/main", "HEAD")
	runGit(t, clone, "checkout", "-q", "main")
	runGit(t, clone, "branch", "-q", "-D", "tmp")
	originMain := gitOutput(t, clone, "rev-parse", "origin/main")

	result, err := m.Add("feat-x", repos, AddOptions{Create: true})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := result.Outcomes[0]; got.Status != StatusCreated || got.Detail != "create branch from origin/main" {
		t.Fatalf("Add() outcome = %+v, want created from origin/main", got)
	}
	if got := gitOutput(t, clone, "rev-parse", "feat-x"); got != originMain {
		t.Errorf("feat-x starts at %s, want origin/main %s", got, originMain)
	}
	if up := gitOutput(t, clone, "for-each-ref", "--format=%(upstream)", "refs/heads/feat-x"); up != "" {
		t.Errorf("feat-x upstream = %q, want none until pushed", up)
	}
}

func TestDefaultBase(t *testing.T) {
	_, _, clone := setupClonedRepo(t)
	runGit(t, clone, "branch", "-q", "local-only")

	tests := []struct {
		name   string
		branch string
		setup  func(t *testing.T)
		want   string
	}{
		{name: "configured branch on origin", branch: "main", want: "origin/main"},
		{name: "configured branch only local", branch: "local-only", want: "local-only"},
		{name: "no branch, origin/HEAD", setup: func(t *testing.T) { runGit(t, clone, "remote", "set-head", "origin", "main") }, want: "origin/main"},
		{name: "no branch, no origin/HEAD", setup: func(t *testing.T) { runGit(t, clone, "remote", "set-head", "origin", "--delete") }, want: "HEAD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}
			got, err := defaultBase(clone, config.Repository{Name: "api", Branch: tt.branch})
			if err != nil {
				t.Fatalf("defaultBase() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("defaultBase() = %q, want %q", got, tt.want)
			}
		})
	}
}
