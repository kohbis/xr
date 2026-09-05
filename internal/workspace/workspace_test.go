package workspace

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/output"
)

func TestNormalizeGitignoreLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"repos/", "repos/"},
		{"./repos/", "repos/"},
		{"/repos/", "repos/"},
		{"./repos", "repos"},
		{"node_modules", "node_modules"},
	}

	for _, tt := range tests {
		if got := normalizeGitignoreLine(tt.input); got != tt.want {
			t.Errorf("normalizeGitignoreLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContainsLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    string
		want    bool
	}{
		{"exact match", "repos/\nnode_modules/\n", "repos/", true},
		{"not present", "repos/\n", "other/", false},
		{"normalized ./", "./repos/\n", "repos/", true},
		{"normalized /", "/repos/\n", "repos/", true},
		{"line with ./ matches plain", "repos/\n", "./repos/", true},
		{"empty content", "", "repos/", false},
		{"whitespace trimmed", "  repos/  \n", "repos/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsLine(tt.content, tt.line); got != tt.want {
				t.Errorf("containsLine(%q, %q) = %v, want %v", tt.content, tt.line, got, tt.want)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	result := expandTilde("~/projects/repo")
	if result == "~/projects/repo" {
		t.Error("expandTilde did not expand ~/")
	}
	if len(result) == 0 {
		t.Error("expandTilde returned empty string")
	}

	plain := "/absolute/path"
	if got := expandTilde(plain); got != plain {
		t.Errorf("expandTilde(%q) = %q, want unchanged", plain, got)
	}

	relative := "relative/path"
	if got := expandTilde(relative); got != relative {
		t.Errorf("expandTilde(%q) = %q, want unchanged", relative, got)
	}
}

func TestNew(t *testing.T) {
	cfg := &config.Config{Workspace: "./repos"}
	ws := New("/tmp/ws", cfg)

	if ws.Root != "/tmp/ws" {
		t.Errorf("Root = %q, want %q", ws.Root, "/tmp/ws")
	}
	if ws.Config != cfg {
		t.Error("Config should point to the provided config")
	}
}

func TestCreateGitignore_AddsEntry(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Workspace: "./repos"}
	ws := New(dir, cfg)

	if err := ws.CreateGitignore(true); err != nil {
		t.Fatalf("CreateGitignore() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}

	if !strings.Contains(string(data), "repos/") {
		t.Errorf(".gitignore should contain 'repos/', got %q", string(data))
	}
}

func TestCreateGitignore_DoesNotDuplicate(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("repos/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Workspace: "./repos"}
	ws := New(dir, cfg)

	if err := ws.CreateGitignore(true); err != nil {
		t.Fatalf("CreateGitignore() error = %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}

	count := strings.Count(string(data), "repos/")
	if count != 1 {
		t.Errorf("'repos/' should appear exactly once, appeared %d times", count)
	}
}

func TestCreateGitignore_NoChangeWhenNotIgnoring(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Workspace: "./repos"}
	ws := New(dir, cfg)

	if err := ws.CreateGitignore(false); err != nil {
		t.Fatalf("CreateGitignore() error = %v", err)
	}

	_, err := os.Stat(filepath.Join(dir, ".gitignore"))
	if err == nil {
		t.Error(".gitignore should not be created when ignoreWorkspace is false")
	}
}

func TestCreateGitignore_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Workspace: "./repos"}
	ws := New(dir, cfg)

	if err := ws.CreateGitignore(true); err != nil {
		t.Fatalf("CreateGitignore() error = %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("existing entries should be preserved")
	}
	if !strings.Contains(content, "repos/") {
		t.Error("new entry should be added")
	}
}

func TestDetectRepo_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	linkPath := filepath.Join(dir, "linked-repo")

	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := detectRepo(dir, entries[0])
	if err != nil {
		t.Fatalf("detectRepo() error = %v", err)
	}
	if repo == nil {
		t.Fatal("detectRepo() returned nil for symlink")
	}
	if repo.Type != config.RepoTypeSymlink {
		t.Errorf("Type = %q, want %q", repo.Type, config.RepoTypeSymlink)
	}
	if repo.Source != target {
		t.Errorf("Source = %q, want %q", repo.Source, target)
	}
}

func TestDetectRepo_RegularFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "not-a-repo.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := detectRepo(dir, entries[0])
	if err != nil {
		t.Fatalf("detectRepo() error = %v", err)
	}
	if repo != nil {
		t.Error("detectRepo() should return nil for regular files")
	}
}

func TestDetectRepo_DirWithoutGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "plain-dir"), 0755); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := detectRepo(dir, entries[0])
	if err != nil {
		t.Fatalf("detectRepo() error = %v", err)
	}
	if repo != nil {
		t.Error("detectRepo() should return nil for directories without .git")
	}
}

func TestDetectRepo_CloneWithGitDir(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "cloned-repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := detectRepo(dir, entries[0])
	if err != nil {
		t.Fatalf("detectRepo() error = %v", err)
	}
	if repo == nil {
		t.Fatal("detectRepo() returned nil for clone")
	}
	if repo.Type != config.RepoTypeClone {
		t.Errorf("Type = %q, want %q", repo.Type, config.RepoTypeClone)
	}
	if repo.Name != "cloned-repo" {
		t.Errorf("Name = %q, want %q", repo.Name, "cloned-repo")
	}
}

func TestAdd_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	repo := config.Repository{
		Name: "my-link", Path: "my-link",
		Type: config.RepoTypeSymlink, Source: target,
	}
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{repo},
	}
	ws := New(dir, cfg)

	if err := ws.Add(repo); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	linkPath := filepath.Join(dir, "repos", "my-link")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file/dir")
	}
}

func TestAdd_CreatesWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	repo := config.Repository{
		Name: "my-link", Path: "my-link",
		Type: config.RepoTypeSymlink, Source: target,
	}
	cfg := &config.Config{
		Workspace:    "./my-workspace",
		Repositories: []config.Repository{repo},
	}
	ws := New(dir, cfg)

	if err := ws.Add(repo); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	wsDir := filepath.Join(dir, "my-workspace")
	if _, err := os.Stat(wsDir); os.IsNotExist(err) {
		t.Error("Add() should create workspace directory if it doesn't exist")
	}
}

func TestRemoveRejectsEscapingPath(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"parent traversal", "../../etc"},
		{"dot only", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := config.Repository{
				Name: "bad-repo", Path: tt.path,
				Type: config.RepoTypeClone, Source: "https://example.com/repo.git",
			}
			cfg := &config.Config{
				Workspace:    "./repos",
				Repositories: []config.Repository{repo},
			}
			ws := New(dir, cfg)

			err := ws.Remove([]config.Repository{repo})
			if err == nil {
				t.Error("Remove() should reject escaping path")
			}
		})
	}
}

func TestValidateInsideDir(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		dest    string
		wantErr bool
	}{
		{"valid child", "/workspace/repos", "/workspace/repos/my-repo", false},
		{"parent escape", "/workspace/repos", "/workspace/repos/../../etc", true},
		{"same dir", "/workspace/repos", "/workspace/repos", true},
		{"nested child", "/workspace/repos", "/workspace/repos/deep/nested", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInsideDir(tt.dir, tt.dest)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInsideDir(%q, %q) error = %v, wantErr %v", tt.dir, tt.dest, err, tt.wantErr)
			}
		})
	}
}

func TestRemoveSymlink(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	linkPath := filepath.Join(wsDir, "my-link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	repo := config.Repository{
		Name: "my-link", Path: "my-link",
		Type: config.RepoTypeSymlink, Source: target,
	}
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{repo},
	}
	ws := New(dir, cfg)

	if err := ws.Remove([]config.Repository{repo}); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should have been removed")
	}
}

func TestRemoveSymlink_AlreadyRemoved(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	repo := config.Repository{
		Name: "gone-link", Path: "gone-link",
		Type: config.RepoTypeSymlink, Source: "/nonexistent",
	}
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{repo},
	}
	ws := New(dir, cfg)

	if err := ws.Remove([]config.Repository{repo}); err != nil {
		t.Fatalf("Remove() should not error for missing symlink: %v", err)
	}
}

func TestRemoveClone(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	cloneDir := filepath.Join(wsDir, "my-clone")
	if err := os.MkdirAll(filepath.Join(cloneDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	// Add a file to verify the entire directory is removed
	if err := os.WriteFile(filepath.Join(cloneDir, "file.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	repo := config.Repository{
		Name: "my-clone", Path: "my-clone",
		Type: config.RepoTypeClone, Source: "https://example.com/repo.git",
	}
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{repo},
	}
	ws := New(dir, cfg)

	if err := ws.Remove([]config.Repository{repo}); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if _, err := os.Stat(cloneDir); !os.IsNotExist(err) {
		t.Error("clone directory should have been removed")
	}
}

func TestRemoveClone_AlreadyRemoved(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	repo := config.Repository{
		Name: "gone-clone", Path: "gone-clone",
		Type: config.RepoTypeClone, Source: "https://example.com/repo.git",
	}
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{repo},
	}
	ws := New(dir, cfg)

	if err := ws.Remove([]config.Repository{repo}); err != nil {
		t.Fatalf("Remove() should not error for missing clone: %v", err)
	}
}

func TestDetectRepo_GitFileAsClone(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "linked-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".git"), []byte("gitdir: ../.git/modules/linked-repo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := detectRepo(dir, entries[0])
	if err != nil {
		t.Fatalf("detectRepo() error = %v", err)
	}
	if repo == nil {
		t.Fatal("detectRepo() returned nil")
	}
	if repo.Type != config.RepoTypeClone {
		t.Errorf("Type = %q, want %q", repo.Type, config.RepoTypeClone)
	}
}

func TestSyncOptions(t *testing.T) {
	opts := SyncOptions{
		Pull:  true,
		Fetch: true,
		Prune: true,
	}
	if !opts.Pull || !opts.Fetch || !opts.Prune {
		t.Error("SyncOptions fields should be set correctly")
	}
}

func TestSync_SkipsReposNotInFilter(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "repo-a", Path: "repo-a", Type: config.RepoTypeClone, Source: "https://example.com/a.git"},
			{Name: "repo-b", Path: "repo-b", Type: config.RepoTypeClone, Source: "https://example.com/b.git"},
		},
	}
	ws := New(dir, cfg)

	// This should not error even though repos don't exist
	// because we're filtering to only sync "repo-a" and it's ok to print warnings
	result, err := ws.Sync([]string{"repo-a"}, SyncOptions{})
	if err != nil {
		t.Errorf("Sync() error = %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
}

func TestSyncSymlink_Missing(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "my-link", Path: "my-link", Type: config.RepoTypeSymlink, Source: "/nonexistent"},
		},
	}
	ws := New(dir, cfg)

	// Should not error, just print a message
	result, err := ws.Sync(nil, SyncOptions{})
	if err != nil {
		t.Errorf("Sync() error = %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
}

func TestSyncSymlink_Exists(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	linkPath := filepath.Join(wsDir, "my-link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "my-link", Path: "my-link", Type: config.RepoTypeSymlink, Source: target},
		},
	}
	ws := New(dir, cfg)

	result, err := ws.Sync(nil, SyncOptions{})
	if err != nil {
		t.Errorf("Sync() error = %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
}

func TestSyncSymlink_WithGitRepo(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a target directory that looks like a git repo
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(wsDir, "my-link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "my-link", Path: "my-link", Type: config.RepoTypeSymlink, Source: target, Branch: "main"},
		},
	}
	ws := New(dir, cfg)

	// syncSymlink will try git operations which will fail since it's not a real git repo,
	// but Sync should not return an error (it prints warnings)
	result, err := ws.Sync(nil, SyncOptions{})
	if err != nil {
		t.Errorf("Sync() error = %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
}

func TestSyncSymlink_NoBranchNonGitDir(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "repos")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	linkPath := filepath.Join(wsDir, "my-link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "my-link", Path: "my-link", Type: config.RepoTypeSymlink, Source: target},
		},
	}
	ws := New(dir, cfg)

	// No branch configured and target is not a git repo — should just say "ok"
	result, err := ws.Sync(nil, SyncOptions{})
	if err != nil {
		t.Errorf("Sync() error = %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
}

// seedGitRepo creates a git repository with one commit on branch "main".
func seedGitRepo(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
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
	runGit(t, dir, "branch", "-M", "main")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func TestSyncClone_MissingSkippedWithoutCloneMissing(t *testing.T) {
	root := t.TempDir()
	source := seedGitRepo(t, filepath.Join(t.TempDir(), "origin"))

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone, Source: source, Branch: "main"},
		},
	}
	ws := New(root, cfg)

	result, err := ws.Sync(nil, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Skipped != 1 || result.Synced != 0 {
		t.Errorf("result = %+v, want Skipped=1 Synced=0", result)
	}
	if _, err := os.Stat(filepath.Join(root, "repos", "api")); !os.IsNotExist(err) {
		t.Error("repository was materialized without --clone-missing")
	}
}

func TestSyncClone_CloneMissingDryRunDoesNotClone(t *testing.T) {
	root := t.TempDir()
	source := seedGitRepo(t, filepath.Join(t.TempDir(), "origin"))

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone, Source: source, Branch: "main"},
		},
	}
	ws := New(root, cfg)

	result, err := ws.Sync(nil, SyncOptions{CloneMissing: true, DryRun: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("result = %+v, want Skipped=1", result)
	}
	if _, err := os.Stat(filepath.Join(root, "repos", "api")); !os.IsNotExist(err) {
		t.Error("--dry-run cloned the repository")
	}
}

func TestSyncClone_CloneMissingClonesIntoNestedPath(t *testing.T) {
	root := t.TempDir()
	source := seedGitRepo(t, filepath.Join(t.TempDir(), "origin"))

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "api", Path: "nested/api", Type: config.RepoTypeClone, Source: source, Branch: "main"},
		},
	}
	ws := New(root, cfg)

	result, err := ws.Sync(nil, SyncOptions{CloneMissing: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Synced != 1 || result.Failed != 0 {
		t.Errorf("result = %+v, want Synced=1 Failed=0", result)
	}

	dest := filepath.Join(root, "repos", "nested", "api")
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatalf("clone missing at %s: %v", dest, err)
	}
	if branch := gitCurrentBranch(dest); branch != "main" {
		t.Errorf("branch = %q, want %q", branch, "main")
	}
}

func TestSyncClone_CloneMissingFailsWithoutSource(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "api", Path: "api", Type: config.RepoTypeClone},
		},
	}
	ws := New(root, cfg)

	result, err := ws.Sync(nil, SyncOptions{CloneMissing: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("result = %+v, want Failed=1", result)
	}
}

func TestSyncSymlink_MissingSkippedWithoutCloneMissing(t *testing.T) {
	root := t.TempDir()
	target := seedGitRepo(t, filepath.Join(t.TempDir(), "lib"))

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "lib", Path: "lib", Type: config.RepoTypeSymlink, Source: target, Branch: "main"},
		},
	}
	ws := New(root, cfg)

	result, err := ws.Sync(nil, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("result = %+v, want Skipped=1", result)
	}
	if _, err := os.Lstat(filepath.Join(root, "repos", "lib")); !os.IsNotExist(err) {
		t.Error("symlink was created without --clone-missing")
	}
}

func TestSyncSymlink_CloneMissingRecreatesLink(t *testing.T) {
	root := t.TempDir()
	target := seedGitRepo(t, filepath.Join(t.TempDir(), "lib"))

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "lib", Path: "lib", Type: config.RepoTypeSymlink, Source: target, Branch: "main"},
		},
	}
	ws := New(root, cfg)

	result, err := ws.Sync(nil, SyncOptions{CloneMissing: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Synced != 1 || result.Failed != 0 {
		t.Errorf("result = %+v, want Synced=1 Failed=0", result)
	}

	link := filepath.Join(root, "repos", "lib")
	got, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink(%s): %v", link, err)
	}
	if got != target {
		t.Errorf("symlink target = %q, want %q", got, target)
	}
}

func TestSyncJobs(t *testing.T) {
	ws := New(t.TempDir(), &config.Config{Workspace: "./repos"})
	confirm := func(_ config.Repository, _ string) (bool, error) { return true, nil }

	tests := []struct {
		name string
		opts SyncOptions
		n    int
		want int
	}{
		{name: "unset means sequential", opts: SyncOptions{}, n: 5, want: 1},
		{name: "zero means sequential", opts: SyncOptions{Jobs: 0}, n: 5, want: 1},
		{name: "negative means sequential", opts: SyncOptions{Jobs: -3}, n: 5, want: 1},
		{name: "capped at repo count", opts: SyncOptions{Jobs: 8}, n: 3, want: 3},
		{name: "used as given", opts: SyncOptions{Jobs: 4}, n: 10, want: 4},
		{name: "prompts force sequential", opts: SyncOptions{Jobs: 8, ConfirmDirty: confirm}, n: 5, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ws.syncJobs(tt.opts, tt.n); got != tt.want {
				t.Fatalf("syncJobs() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestSyncConcurrent_OutputMatchesSequential is the core guarantee of --jobs:
// concurrency must not change what the user sees.
func TestSyncConcurrent_OutputMatchesSequential(t *testing.T) {
	sources := make([]string, 6)
	for i := range sources {
		sources[i] = seedGitRepo(t, filepath.Join(t.TempDir(), fmt.Sprintf("origin%d", i)))
	}

	run := func(jobs int) (string, SyncResult) {
		repos := make([]config.Repository, len(sources))
		for i, src := range sources {
			repos[i] = config.Repository{
				Name:   fmt.Sprintf("repo%d", i),
				Path:   fmt.Sprintf("repo%d", i),
				Type:   config.RepoTypeClone,
				Source: src,
				Branch: "main",
			}
		}
		// One repository has no source, so a failure is part of the comparison.
		repos = append(repos, config.Repository{Name: "broken", Path: "broken", Type: config.RepoTypeClone})

		ws := New(t.TempDir(), &config.Config{Workspace: "./repos", Repositories: repos})

		var result *SyncResult
		out := captureStdout(t, func() {
			var err error
			result, err = ws.Sync(nil, SyncOptions{CloneMissing: true, Jobs: jobs})
			if err != nil {
				t.Errorf("Sync() error = %v", err)
			}
		})
		return out, *result
	}

	seqOut, seqResult := run(1)
	parOut, parResult := run(4)

	// git clone writes absolute paths, so compare the xr-rendered lines only.
	if got, want := syncLines(parOut), syncLines(seqOut); got != want {
		t.Errorf("concurrent output differs from sequential\n--- sequential ---\n%s\n--- concurrent ---\n%s", want, got)
	}
	if !reflect.DeepEqual(parResult, seqResult) {
		t.Errorf("concurrent result = %+v, sequential = %+v", parResult, seqResult)
	}
	if seqResult.Synced != len(sources) || seqResult.Failed != 1 {
		t.Fatalf("fixture did not exercise both paths: %+v", seqResult)
	}
}

// syncLines keeps the lines xr itself renders, dropping git's clone chatter and
// the temp-dir paths that differ between runs.
func syncLines(out string) string {
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "["), strings.HasPrefix(line, "  ✓"), strings.HasPrefix(line, "  ⊘"):
			kept = append(kept, line)
		case strings.HasPrefix(line, "  →"):
			// "cloning from <tmpdir>" differs per run; keep only the verb.
			if i := strings.Index(line, " from "); i >= 0 {
				line = line[:i+len(" from ")]
			}
			kept = append(kept, line)
		case strings.HasPrefix(line, "  ✗"):
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
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

// TestSyncClone_CloneProgressGoesToStderr pins the stream git's clone output
// belongs to. Buffering per repository once moved it onto stdout, which broke
// `2>/dev/null` for callers.
func TestSyncClone_CloneProgressGoesToStderr(t *testing.T) {
	source := seedGitRepo(t, filepath.Join(t.TempDir(), "origin"))

	for _, jobs := range []int{1, 4} {
		t.Run(fmt.Sprintf("jobs=%d", jobs), func(t *testing.T) {
			root := t.TempDir()
			cfg := &config.Config{
				Workspace: "./repos",
				Repositories: []config.Repository{
					{Name: "api", Path: "api", Type: config.RepoTypeClone, Source: source, Branch: "main"},
				},
			}
			ws := New(root, cfg)

			stdout, stderr := captureStreams(t, func() {
				if _, err := ws.Sync(nil, SyncOptions{CloneMissing: true, Jobs: jobs}); err != nil {
					t.Errorf("Sync() error = %v", err)
				}
			})

			if strings.Contains(stdout, "Cloning into") {
				t.Errorf("git clone progress leaked onto stdout:\n%s", stdout)
			}
			if !strings.Contains(stderr, "Cloning into") {
				t.Errorf("git clone progress missing from stderr:\n%s", stderr)
			}
			// xr's own progress lines stay on stdout.
			if !strings.Contains(stdout, "cloned") {
				t.Errorf("sync progress missing from stdout:\n%s", stdout)
			}
		})
	}
}

func captureStreams(t *testing.T, fn func()) (string, string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW

	read := func(r *os.File) chan string {
		ch := make(chan string)
		go func() {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			ch <- buf.String()
		}()
		return ch
	}
	outCh, errCh := read(outR), read(errR)

	fn()

	os.Stdout, os.Stderr = origOut, origErr
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}
	return <-outCh, <-errCh
}

func TestSync_RecordsOutcomes(t *testing.T) {
	root := t.TempDir()
	reposDir := filepath.Join(root, "repos")
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		t.Fatal(err)
	}
	// A symlink repo pointing at a plain directory is skipped as "not a git
	// repository"; a missing clone without CloneMissing is skipped too; a
	// symlink repo whose path is occupied by a real directory fails.
	target := filepath.Join(root, "plain")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(reposDir, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(reposDir, "broken"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "linked", Path: "linked", Type: config.RepoTypeSymlink, Source: target},
			{Name: "absent", Path: "absent", Type: config.RepoTypeClone, Source: "https://example.invalid/absent.git"},
			{Name: "broken", Path: "broken", Type: config.RepoTypeSymlink, Source: target},
		},
	}
	ws := New(root, cfg)

	result, err := ws.Sync(nil, SyncOptions{Quiet: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(result.Repos) != 3 {
		t.Fatalf("got %d outcomes, want 3: %+v", len(result.Repos), result.Repos)
	}
	if got := result.Repos[0]; got.Name != "linked" || got.Status != SyncStatusSkipped || got.Detail != "not a git repository" {
		t.Errorf("linked outcome = %+v", got)
	}
	if got := result.Repos[1]; got.Name != "absent" || got.Status != SyncStatusSkipped || !strings.Contains(got.Detail, "missing") {
		t.Errorf("absent outcome = %+v", got)
	}
	if got := result.Repos[2]; got.Name != "broken" || got.Status != SyncStatusFailed || !strings.Contains(got.Detail, "not a symlink") {
		t.Errorf("broken outcome = %+v", got)
	}
	for _, o := range result.Repos {
		if o.Steps == nil {
			t.Errorf("%s: Steps is nil, want empty slice for JSON", o.Name)
		}
	}
	if result.Skipped+result.Failed+result.Synced != 3 {
		t.Errorf("counts do not add up: %+v", result)
	}
}

func TestScanRepos_ReturnsWarningsInsteadOfPrinting(t *testing.T) {
	root := t.TempDir()
	reposDir := filepath.Join(root, "repos")
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		t.Fatal(err)
	}
	// A git repository without an origin remote is detected, with a warning.
	noRemote := filepath.Join(reposDir, "local-only")
	if err := os.MkdirAll(noRemote, 0755); err != nil {
		t.Fatal(err)
	}
	initCmd := exec.Command("git", "init")
	initCmd.Dir = noRemote
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}

	ws := New(root, &config.Config{Workspace: "./repos"})
	out := captureStdout(t, func() {
		scan, err := ws.ScanRepos()
		if err != nil {
			t.Fatalf("ScanRepos() error = %v", err)
		}
		if len(scan.Repos) != 1 || scan.Repos[0].Name != "local-only" {
			t.Errorf("Repos = %+v, want local-only", scan.Repos)
		}
		if len(scan.Warnings) != 1 || !strings.Contains(scan.Warnings[0], "no origin remote") {
			t.Errorf("Warnings = %v, want one about the missing origin", scan.Warnings)
		}
	})
	if out != "" {
		t.Errorf("ScanRepos printed to stdout: %q", out)
	}
}

func TestAdd_ProgressGoesToInjectedPrinter(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	ws := New(root, &config.Config{Workspace: "./repos"})
	ws.Printer = output.NewSyncPrinter(&buf, &buf)

	repo := config.Repository{Name: "lib", Path: "lib", Type: config.RepoTypeSymlink, Source: target}
	stdout := captureStdout(t, func() {
		if err := ws.Add(repo); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	})
	if stdout != "" {
		t.Errorf("Add printed to stdout despite injected printer: %q", stdout)
	}
	if !strings.Contains(buf.String(), "symlink created") {
		t.Errorf("injected printer did not receive progress, got %q", buf.String())
	}
}
