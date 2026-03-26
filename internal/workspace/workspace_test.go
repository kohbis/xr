package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kohbis/xr/internal/config"
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

func TestDetectRepo_SubmoduleWithGitFile(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "sub-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".git"), []byte("gitdir: ../.git/modules/sub-repo\n"), 0644); err != nil {
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
		t.Fatal("detectRepo() returned nil for submodule")
	}
	if repo.Type != config.RepoTypeGit {
		t.Errorf("Type = %q, want %q (submodule has .git file, not dir)", repo.Type, config.RepoTypeGit)
	}
}
