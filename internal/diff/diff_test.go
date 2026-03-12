package diff

import (
	"os"
	"path/filepath"
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
