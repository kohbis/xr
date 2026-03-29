package diff

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kohbis/xr/internal/config"
)

func TestSearchPattern_MatchesAcrossRepos(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")

	for _, name := range []string{"repo-a", "repo-b"} {
		repoDir := filepath.Join(reposDir, name)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := "version = 1.0\nname = " + name + "\n"
		if err := os.WriteFile(filepath.Join(repoDir, "config.txt"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "repo-a", Path: "repo-a", Type: config.RepoTypeClone},
			{Name: "repo-b", Path: "repo-b", Type: config.RepoTypeClone},
		},
	}

	result, err := SearchPattern(cfg, reposDir, "version")
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got results for %d repos, want 2", len(result))
	}

	for _, name := range []string{"repo-a", "repo-b"} {
		occs, ok := result[name]
		if !ok {
			t.Errorf("missing results for %s", name)
			continue
		}
		if len(occs) != 1 {
			t.Errorf("repo %s: got %d occurrences, want 1", name, len(occs))
		}
	}
}

func TestSearchPattern_InvalidRegex(t *testing.T) {
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{},
	}

	_, err := SearchPattern(cfg, "/tmp", "[invalid")
	if err == nil {
		t.Fatal("SearchPattern() expected error for invalid regex, got nil")
	}
}

func TestSearchPattern_SkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	repoDir := filepath.Join(reposDir, "proj")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".hidden"), []byte("secret\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "visible.txt"), []byte("secret\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "proj", Path: "proj", Type: config.RepoTypeClone},
		},
	}

	result, err := SearchPattern(cfg, reposDir, "secret")
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	occs := result["proj"]
	if len(occs) != 1 {
		t.Errorf("got %d occurrences, want 1 (hidden file should be skipped)", len(occs))
	}
}

func TestSearchPattern_SkipsMissingRepo(t *testing.T) {
	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "ghost", Path: "ghost", Type: config.RepoTypeClone},
		},
	}

	result, err := SearchPattern(cfg, "/tmp/nonexistent-ws", "anything")
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	if len(result["ghost"]) != 0 {
		t.Error("expected no results for missing repo")
	}
}

func TestCompareFile_TwoReposWithSameFile(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")

	for i, name := range []string{"repo-a", "repo-b"} {
		repoDir := filepath.Join(reposDir, name)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := []byte("version = " + string(rune('1'+i)) + "\n")
		if err := os.WriteFile(filepath.Join(repoDir, "Makefile"), content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "repo-a", Path: "repo-a", Type: config.RepoTypeClone},
			{Name: "repo-b", Path: "repo-b", Type: config.RepoTypeClone},
		},
	}

	comparisons, err := CompareFile(cfg, reposDir, "Makefile")
	if err != nil {
		t.Fatalf("CompareFile() error = %v", err)
	}

	if len(comparisons) != 1 {
		t.Fatalf("got %d comparisons, want 1", len(comparisons))
	}
	if len(comparisons[0].Repos) != 2 {
		t.Errorf("got %d repo files, want 2", len(comparisons[0].Repos))
	}
}

func TestCompareFile_SingleRepoNoComparison(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	repoDir := filepath.Join(reposDir, "only-one")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "only-one", Path: "only-one", Type: config.RepoTypeClone},
		},
	}

	comparisons, err := CompareFile(cfg, reposDir, "file.txt")
	if err != nil {
		t.Fatalf("CompareFile() error = %v", err)
	}

	if len(comparisons) != 0 {
		t.Errorf("got %d comparisons, want 0 (need >= 2 repos for comparison)", len(comparisons))
	}
}

func TestCompareFile_NestedFiles(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")

	for _, name := range []string{"repo-a", "repo-b"} {
		subDir := filepath.Join(reposDir, name, "src", "config")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := []byte("setting = " + name + "\n")
		if err := os.WriteFile(filepath.Join(subDir, "app.conf"), content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "repo-a", Path: "repo-a", Type: config.RepoTypeClone},
			{Name: "repo-b", Path: "repo-b", Type: config.RepoTypeClone},
		},
	}

	comparisons, err := CompareFile(cfg, reposDir, "app.conf")
	if err != nil {
		t.Fatalf("CompareFile() error = %v", err)
	}

	if len(comparisons) != 1 {
		t.Fatalf("got %d comparisons, want 1", len(comparisons))
	}
	if len(comparisons[0].Repos) != 2 {
		t.Errorf("got %d repo files, want 2", len(comparisons[0].Repos))
	}
}

func TestCompareFile_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	repoDir := filepath.Join(reposDir, "proj")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "other.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "proj", Path: "proj", Type: config.RepoTypeClone},
		},
	}

	comparisons, err := CompareFile(cfg, reposDir, "nonexistent.txt")
	if err != nil {
		t.Fatalf("CompareFile() error = %v", err)
	}
	if len(comparisons) != 0 {
		t.Errorf("got %d comparisons, want 0", len(comparisons))
	}
}

func TestSearchPattern_RegexMatch(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	repoDir := filepath.Join(reposDir, "proj")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "func main() {\nvar x = 10\nfunc helper() {\n"
	if err := os.WriteFile(filepath.Join(repoDir, "code.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "proj", Path: "proj", Type: config.RepoTypeClone},
		},
	}

	result, err := SearchPattern(cfg, reposDir, `func\s+\w+`)
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	occs := result["proj"]
	if len(occs) != 2 {
		t.Errorf("got %d regex occurrences, want 2", len(occs))
	}
}

func TestSearchPattern_LineNumbers(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	repoDir := filepath.Join(reposDir, "proj")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "first\nsecond\ntarget\nfourth\n"
	if err := os.WriteFile(filepath.Join(repoDir, "lines.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "proj", Path: "proj", Type: config.RepoTypeClone},
		},
	}

	result, err := SearchPattern(cfg, reposDir, "target")
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	occs := result["proj"]
	if len(occs) != 1 {
		t.Fatalf("got %d occurrences, want 1", len(occs))
	}
	if occs[0].Line != 3 {
		t.Errorf("Line = %d, want 3", occs[0].Line)
	}
	if occs[0].Content != "target" {
		t.Errorf("Content = %q, want %q", occs[0].Content, "target")
	}
}

func TestDiffFiles_IdenticalContent(t *testing.T) {
	f1 := RepoFile{Repo: "a", Path: "file.txt", Content: "same content\n"}
	f2 := RepoFile{Repo: "b", Path: "file.txt", Content: "same content\n"}

	result, err := DiffFiles(f1, f2)
	if err != nil {
		t.Fatalf("DiffFiles() error = %v", err)
	}

	if result != "" {
		t.Errorf("DiffFiles() for identical files should be empty, got %q", result)
	}
}

func TestDiffFiles_DifferentContent(t *testing.T) {
	f1 := RepoFile{Repo: "a", Path: "file.txt", Content: "line1\nline2\n"}
	f2 := RepoFile{Repo: "b", Path: "file.txt", Content: "line1\nchanged\n"}

	result, err := DiffFiles(f1, f2)
	if err != nil {
		t.Fatalf("DiffFiles() error = %v", err)
	}

	if result == "" {
		t.Error("DiffFiles() for different files should not be empty")
	}
}

func TestRepoMatchesFilter(t *testing.T) {
	if !repoMatchesFilter(nil, "any") {
		t.Error("empty filter should match any repo")
	}
	if !repoMatchesFilter([]string{"a", "b"}, "b") {
		t.Error("expected b to match filter")
	}
	if repoMatchesFilter([]string{"a"}, "c") {
		t.Error("c should not match filter")
	}
}

func TestGitDiff_RespectsRepoFilter(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	reposDir := filepath.Join(t.TempDir(), "repos")

	makeGitRepo := func(name, commitFile, worktreeExtra string) {
		t.Helper()
		rd := filepath.Join(reposDir, name)
		if err := os.MkdirAll(rd, 0755); err != nil {
			t.Fatal(err)
		}
		gitInitOrSkip(t, rd)
		runGit(t, rd, "config", "user.email", "xr@test")
		runGit(t, rd, "config", "user.name", "xr")
		if err := os.WriteFile(filepath.Join(rd, commitFile), []byte("baseline\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGit(t, rd, "add", commitFile)
		runGit(t, rd, "commit", "-m", "init", "--no-gpg-sign")
		if err := os.WriteFile(filepath.Join(rd, commitFile), []byte("baseline\n"+worktreeExtra), 0644); err != nil {
			t.Fatal(err)
		}
	}

	makeGitRepo("alpha", "f.txt", "delta-alpha\n")
	makeGitRepo("beta", "f.txt", "delta-beta\n")

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "alpha", Path: "alpha", Type: config.RepoTypeClone},
			{Name: "beta", Path: "beta", Type: config.RepoTypeClone},
		},
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	gitErr := GitDiff(cfg, reposDir, []string{"alpha"}, nil)

	_ = w.Close()
	os.Stdout = old
	out, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if gitErr != nil {
		t.Fatalf("GitDiff() error = %v", gitErr)
	}

	s := string(out)
	if s == "" {
		t.Fatal("expected diff output for alpha")
	}
	if !strings.Contains(s, "=== alpha ===") {
		t.Errorf("expected alpha repo header, got:\n%s", s)
	}
	if strings.Contains(s, "=== beta ===") {
		t.Errorf("beta should be excluded by --repo filter, got:\n%s", s)
	}
}

func gitInitOrSkip(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
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
