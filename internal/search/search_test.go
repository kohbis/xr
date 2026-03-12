package search

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kohbis/xr/internal/config"
)

func TestParseRipgrepOutput_StandardOutput(t *testing.T) {
	output := "/workspace/repos/proj/main.go:10:func main() {\n/workspace/repos/proj/main.go:15:}\n"

	matches, err := parseRipgrepOutput("proj", "/workspace/repos/proj", output, false)
	if err != nil {
		t.Fatalf("parseRipgrepOutput() error = %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}

	if matches[0].File != "main.go" {
		t.Errorf("matches[0].File = %q, want %q", matches[0].File, "main.go")
	}
	if matches[0].Line != 10 {
		t.Errorf("matches[0].Line = %d, want 10", matches[0].Line)
	}
	if matches[0].Content != "func main() {" {
		t.Errorf("matches[0].Content = %q, want %q", matches[0].Content, "func main() {")
	}
	if matches[0].Repo != "proj" {
		t.Errorf("matches[0].Repo = %q, want %q", matches[0].Repo, "proj")
	}
}

func TestParseRipgrepOutput_EmptyOutput(t *testing.T) {
	matches, err := parseRipgrepOutput("proj", "/workspace/repos/proj", "", false)
	if err != nil {
		t.Fatalf("parseRipgrepOutput() error = %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0", len(matches))
	}
}

func TestParseRipgrepOutput_SkipsSeparatorLines(t *testing.T) {
	output := "/workspace/repos/proj/a.go:1:line1\n--\n/workspace/repos/proj/b.go:2:line2\n"

	matches, err := parseRipgrepOutput("proj", "/workspace/repos/proj", output, false)
	if err != nil {
		t.Fatalf("parseRipgrepOutput() error = %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
}

func TestSearchFile_BasicMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello world\nfoo bar\nhello again\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile("hello")
	matches, err := searchFile("repo", dir, filePath, pattern, 0)
	if err != nil {
		t.Fatalf("searchFile() error = %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0].Line != 1 {
		t.Errorf("matches[0].Line = %d, want 1", matches[0].Line)
	}
	if matches[1].Line != 3 {
		t.Errorf("matches[1].Line = %d, want 3", matches[1].Line)
	}
}

func TestSearchFile_WithContext(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "line1\nline2\ntarget\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile("target")
	matches, err := searchFile("repo", dir, filePath, pattern, 1)
	if err != nil {
		t.Fatalf("searchFile() error = %v", err)
	}

	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3 (1 context before + 1 match + 1 context after)", len(matches))
	}

	if !matches[0].IsContext {
		t.Error("matches[0] should be context")
	}
	if matches[1].IsContext {
		t.Error("matches[1] should not be context")
	}
	if !matches[2].IsContext {
		t.Error("matches[2] should be context")
	}
}

func TestSearchFile_NoMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("nothing here\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile("missing")
	matches, err := searchFile("repo", dir, filePath, pattern, 0)
	if err != nil {
		t.Fatalf("searchFile() error = %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("got %d matches, want 0", len(matches))
	}
}

func TestSearchFile_ContextDoesNotDuplicate(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "match1\nmatch2\nother\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile("match")
	matches, err := searchFile("repo", dir, filePath, pattern, 1)
	if err != nil {
		t.Fatalf("searchFile() error = %v", err)
	}

	// match1 (line 1) + match2 (line 2, also context of match1) + other (line 3, context of match2)
	if len(matches) != 3 {
		t.Errorf("got %d matches, want 3 (no duplicate context lines)", len(matches))
	}
}

func TestSearchBuiltin_GlobFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Pattern: "package",
		Glob:    "*.go",
	}
	matches, err := searchBuiltin("repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1 (only .go file)", len(matches))
	}
	if matches[0].File != "hello.go" {
		t.Errorf("File = %q, want %q", matches[0].File, "hello.go")
	}
}

func TestSearchBuiltin_IgnoreCase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("Hello World\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Pattern: "hello", IgnoreCase: true}
	matches, err := searchBuiltin("repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestSearchBuiltin_SkipsDotDirs(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hidden, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "secret.txt"), []byte("match\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("match\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Pattern: "match"}
	matches, err := searchBuiltin("repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1 (hidden dir should be skipped)", len(matches))
	}
}

func TestSearchBuiltin_Regex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("func main() {\nvar x = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Pattern: `func\s+\w+`, UseRegex: true}
	matches, err := searchBuiltin("repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestSearch_RepoFilter(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	for _, name := range []string{"alpha", "beta"} {
		repoDir := filepath.Join(reposDir, name)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("target\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "alpha", Path: "alpha", Type: config.RepoTypeClone},
			{Name: "beta", Path: "beta", Type: config.RepoTypeClone},
		},
	}

	matches, err := Search(cfg, reposDir, Options{
		Pattern:    "target",
		RepoFilter: []string{"alpha"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	for _, m := range matches {
		if m.Repo != "alpha" {
			t.Errorf("got match from repo %q, only expected alpha", m.Repo)
		}
	}
}

func TestContains(t *testing.T) {
	if !contains([]string{"a", "b", "c"}, "b") {
		t.Error("contains(a,b,c, b) = false, want true")
	}
	if contains([]string{"a", "b"}, "z") {
		t.Error("contains(a,b, z) = true, want false")
	}
	if contains(nil, "a") {
		t.Error("contains(nil, a) = true, want false")
	}
}

func TestIsFilePath(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/path/to/file", true},
		{"file.go", true},
		{"nopath", false},
		{"src/main", true},
	}

	for _, tt := range tests {
		if got := isFilePath(tt.input); got != tt.want {
			t.Errorf("isFilePath(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestMinMax(t *testing.T) {
	if got := max(3, 5); got != 5 {
		t.Errorf("max(3,5) = %d, want 5", got)
	}
	if got := max(7, 2); got != 7 {
		t.Errorf("max(7,2) = %d, want 7", got)
	}
	if got := min(3, 5); got != 3 {
		t.Errorf("min(3,5) = %d, want 3", got)
	}
	if got := min(7, 2); got != 2 {
		t.Errorf("min(7,2) = %d, want 2", got)
	}
}
