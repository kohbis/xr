package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kohbis/xr/internal/config"
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

func TestValidateInsideDir(t *testing.T) {
	if err := validateInsideDir("/ws/worktrees", "/ws/worktrees/api/feat-x"); err != nil {
		t.Errorf("validateInsideDir() error = %v, want nil", err)
	}
	if err := validateInsideDir("/ws/worktrees", "/ws/worktrees"); err == nil {
		t.Error("validateInsideDir() on root error = nil, want error")
	}
	if err := validateInsideDir("/ws/worktrees", "/ws/repos/api"); err == nil {
		t.Error("validateInsideDir() on outside path error = nil, want error")
	}
	// A sibling directory sharing the prefix must not be treated as inside.
	if err := validateInsideDir("/ws/wt", "/ws/wt-other/api"); err == nil {
		t.Error("validateInsideDir() on prefix sibling error = nil, want error")
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
	repoDir := filepath.Join(root, "repos", "api")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	runGit(t, repoDir, "config", "user.email", "xr@test")
	runGit(t, repoDir, "config", "user.name", "xr")
	if err := os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", "f.txt")
	runGit(t, repoDir, "commit", "-m", "init", "--no-gpg-sign")

	cfg := &config.Config{
		Workspace: "./repos",
		Worktrees: "./worktrees",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone},
		},
	}
	return New(root, cfg), cfg.Repositories[0]
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
