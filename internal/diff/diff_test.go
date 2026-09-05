package diff

import (
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

	result, err := SearchPattern(cfg, reposDir, "version", nil)
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got results for %d repos, want 2", len(result))
	}

	for i, name := range []string{"repo-a", "repo-b"} {
		if result[i].Repo != name {
			t.Errorf("result[%d].Repo = %q, want %q (configuration order)", i, result[i].Repo, name)
			continue
		}
		if len(result[i].Matches) != 1 {
			t.Errorf("repo %s: got %d occurrences, want 1", name, len(result[i].Matches))
		}
	}
}

func TestSearchPattern_KeepsConfigurationOrder(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	names := []string{"zeta", "alpha", "mid", "beta", "omega"}
	var repos []config.Repository
	for _, name := range names {
		repoDir := filepath.Join(reposDir, name)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("needle\n"), 0644); err != nil {
			t.Fatal(err)
		}
		repos = append(repos, config.Repository{Name: name, Path: name, Type: config.RepoTypeClone})
	}
	cfg := &config.Config{Workspace: "./repos", Repositories: repos}

	for range 5 {
		result, err := SearchPattern(cfg, reposDir, "needle", nil)
		if err != nil {
			t.Fatalf("SearchPattern() error = %v", err)
		}
		var got []string
		for _, r := range result {
			got = append(got, r.Repo)
		}
		if strings.Join(got, ",") != strings.Join(names, ",") {
			t.Fatalf("order = %v, want %v", got, names)
		}
	}
}

func TestSearchPattern_SkipsGitDirAndIgnoredFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	repoDir := filepath.Join(reposDir, "proj")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	gitInitOrSkip(t, repoDir)
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("build/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("needle\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "build"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "build", "out.txt"), []byte("needle\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Something inside .git that would match if the directory were scanned.
	if err := os.WriteFile(filepath.Join(repoDir, ".git", "description"), []byte("needle\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{{Name: "proj", Path: "proj", Type: config.RepoTypeClone}},
	}

	result, err := SearchPattern(cfg, reposDir, "needle", nil)
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}
	if len(result[0].Matches) != 1 || result[0].Matches[0].File != "main.go" {
		t.Errorf("matches = %+v, want only main.go", result[0].Matches)
	}

	comparisons, err := CompareFile(cfg, reposDir, "out.txt", nil)
	if err != nil {
		t.Fatalf("CompareFile() error = %v", err)
	}
	if len(comparisons) != 0 {
		t.Errorf("CompareFile found ignored file: %+v", comparisons)
	}
}

func TestSearchPattern_InvalidRegex(t *testing.T) {
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{},
	}

	_, err := SearchPattern(cfg, "/tmp", "[invalid", nil)
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

	result, err := SearchPattern(cfg, reposDir, "secret", nil)
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	if len(result) != 1 || len(result[0].Matches) != 1 {
		t.Errorf("got %+v, want exactly 1 occurrence (hidden file should be skipped)", result)
	}
}

func TestSearchPattern_SkipsMissingRepo(t *testing.T) {
	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "ghost", Path: "ghost", Type: config.RepoTypeClone},
		},
	}

	result, err := SearchPattern(cfg, "/tmp/nonexistent-ws", "anything", nil)
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected no results for missing repo, got %+v", result)
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

	comparisons, err := CompareFile(cfg, reposDir, "Makefile", nil)
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

	comparisons, err := CompareFile(cfg, reposDir, "file.txt", nil)
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

	comparisons, err := CompareFile(cfg, reposDir, "app.conf", nil)
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

	comparisons, err := CompareFile(cfg, reposDir, "nonexistent.txt", nil)
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

	result, err := SearchPattern(cfg, reposDir, `func\s+\w+`, nil)
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	occs := result[0].Matches
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

	result, err := SearchPattern(cfg, reposDir, "target", nil)
	if err != nil {
		t.Fatalf("SearchPattern() error = %v", err)
	}

	occs := result[0].Matches
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

	results := GitDiff(cfg, reposDir, []string{"alpha"}, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (beta excluded by filter): %+v", len(results), results)
	}
	if results[0].Repo != "alpha" || results[0].Error != "" {
		t.Fatalf("result = %+v", results[0])
	}
	if !strings.Contains(results[0].Output, "delta-alpha") {
		t.Errorf("expected diff output for alpha, got:\n%s", results[0].Output)
	}
}

func TestGitDiff_ReportsErrorForNonGitDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	reposDir := filepath.Join(t.TempDir(), "repos")
	if err := os.MkdirAll(filepath.Join(reposDir, "plain"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{{Name: "plain", Path: "plain", Type: config.RepoTypeClone}},
	}
	results := GitDiff(cfg, reposDir, nil, nil)
	if len(results) != 1 || results[0].Error == "" {
		t.Fatalf("expected an error result for a non-git directory, got %+v", results)
	}
}

func TestSearchHistoryResults_ReportsErrorForNonGitDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	reposDir := filepath.Join(t.TempDir(), "repos")
	if err := os.MkdirAll(filepath.Join(reposDir, "plain"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Workspace:    "./repos",
		Repositories: []config.Repository{{Name: "plain", Path: "plain", Type: config.RepoTypeClone}},
	}
	results, err := SearchHistoryResults(cfg, reposDir, "fix", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Error == "" || len(results[0].Lines) != 0 {
		t.Fatalf("expected an error result with no lines, got %+v", results)
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
