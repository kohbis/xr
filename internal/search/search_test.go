package search

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kohbis/xr/internal/config"
)

func TestParseRipgrepOutput_StandardOutput(t *testing.T) {
	output := "main.go\x1f10\x1ffunc main() {\n" +
		"lib/util.go\x1f25\x1freturn nil\n"

	matches := parseRipgrepOutput("proj", output)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0].File != "main.go" || matches[0].Line != 10 || matches[0].Content != "func main() {" {
		t.Errorf("first match = %+v", matches[0])
	}
	if matches[0].IsContext {
		t.Error("match line should not be marked as context")
	}
	if matches[1].File != "lib/util.go" || matches[1].Line != 25 {
		t.Errorf("second match = %+v", matches[1])
	}
}

func TestParseRipgrepOutput_ContextLines(t *testing.T) {
	output := "main.go\x1e9\x1eimport \"fmt\"\n" +
		"main.go\x1f10\x1ffunc main() {\n"

	matches := parseRipgrepOutput("proj", output)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if !matches[0].IsContext {
		t.Error("line separated by the context separator should be marked as context")
	}
	if matches[1].IsContext {
		t.Error("line separated by the match separator should not be context")
	}
}

func TestParseRipgrepOutput_EmptyOutput(t *testing.T) {
	if matches := parseRipgrepOutput("proj", ""); len(matches) != 0 {
		t.Errorf("got %d matches for empty output, want 0", len(matches))
	}
}

func TestParseRipgrepOutput_SkipsNonResultLines(t *testing.T) {
	// The "--" between context groups and the note ripgrep prints for a binary
	// file carry no line number, so neither becomes a match.
	output := "--\n" +
		"bin.dat: binary file matches (found \"\\0\" byte around offset 5)\n" +
		"main.go\x1f1\x1fhit\n"

	matches := parseRipgrepOutput("proj", output)
	if len(matches) != 1 || matches[0].File != "main.go" {
		t.Errorf("matches = %+v, want only main.go", matches)
	}
}

func TestParseRipgrepOutput_PathWithSeparatorCharacters(t *testing.T) {
	// A path containing ":" and "-" used to confuse the old heuristic parser.
	output := "weird:name-1.go\x1f7\x1fhit\n"

	matches := parseRipgrepOutput("proj", output)
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if matches[0].File != "weird:name-1.go" || matches[0].Line != 7 || matches[0].Content != "hit" {
		t.Errorf("match = %+v", matches[0])
	}
}

func TestBatchFiles(t *testing.T) {
	files := []string{"aaaa", "bbbb", "cccc"}
	// Budget for two paths of five bytes each.
	batches := batchFiles(files, 10)
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2: %v", len(batches), batches)
	}
	if len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Errorf("batches = %v", batches)
	}

	var flat []string
	for _, b := range batchFiles(files, 1) {
		flat = append(flat, b...)
	}
	if strings.Join(flat, ",") != "aaaa,bbbb,cccc" {
		t.Errorf("a tiny budget must still cover every file, got %v", flat)
	}
	if len(batchFiles(nil, 10)) != 0 {
		t.Error("no files should produce no batches")
	}
}

func TestSearchFile_BasicMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello world\nfoo bar\nhello again\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile("hello")
	matches, err := searchFile("repo", "test.txt", filePath, pattern, 0)
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
	matches, err := searchFile("repo", "test.txt", filePath, pattern, 1)
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
	matches, err := searchFile("repo", "test.txt", filePath, pattern, 0)
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
	matches, err := searchFile("repo", "test.txt", filePath, pattern, 1)
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
	matches, err := searchBuiltinAll(t, "repo", dir, opts)
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
	matches, err := searchBuiltinAll(t, "repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestSearchBuiltin_SkipsGitDirButCoversDotFiles(t *testing.T) {
	dir := t.TempDir()
	// A dot directory a project actually tracks, such as .github, is part of
	// the repository and is searched; only .git itself is off limits.
	tracked := filepath.Join(dir, ".github")
	if err := os.MkdirAll(tracked, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tracked, "ci.yml"), []byte("match\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "description"), []byte("match\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("match\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Pattern: "match"}
	matches, err := searchBuiltinAll(t, "repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	var files []string
	for _, m := range matches {
		files = append(files, m.File)
	}
	sort.Strings(files)
	if strings.Join(files, ",") != ".github/ci.yml,visible.txt" {
		t.Errorf("files = %v, want .github/ci.yml and visible.txt (.git excluded)", files)
	}
}

func TestSearchBuiltin_Regex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("func main() {\nvar x = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Pattern: `func\s+\w+`, UseRegex: true}
	matches, err := searchBuiltinAll(t, "repo", dir, opts)
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

func TestSearch_BuiltinMultiRepo(t *testing.T) {
	dir := t.TempDir()
	reposDir := filepath.Join(dir, "repos")
	for _, name := range []string{"app", "lib"} {
		repoDir := filepath.Join(reposDir, name)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\nimport \"fmt\"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(reposDir, "lib", "extra.go"), []byte("package lib\nimport \"fmt\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "app", Path: "app", Type: config.RepoTypeClone},
			{Name: "lib", Path: "lib", Type: config.RepoTypeClone},
		},
	}

	matches, err := searchBuiltinAll(t, "app", filepath.Join(reposDir, "app"), Options{Pattern: "fmt"})
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("app: got %d matches, want 1", len(matches))
	}

	allMatches, err := Search(cfg, reposDir, Options{Pattern: "fmt"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	appCount, libCount := 0, 0
	for _, m := range allMatches {
		switch m.Repo {
		case "app":
			appCount++
		case "lib":
			libCount++
		}
	}
	if appCount != 1 {
		t.Errorf("app matches = %d, want 1", appCount)
	}
	if libCount != 2 {
		t.Errorf("lib matches = %d, want 2 (main.go + extra.go)", libCount)
	}
}

func TestSearch_SkipsMissingRepo(t *testing.T) {
	cfg := &config.Config{
		Workspace: "./repos",
		Repositories: []config.Repository{
			{Name: "ghost", Path: "ghost", Type: config.RepoTypeClone},
		},
	}

	matches, err := Search(cfg, "/nonexistent/ws", Options{Pattern: "anything"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("got %d matches for missing repo, want 0", len(matches))
	}
}

func TestSearchBuiltin_FixedString(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("func main() {\nvar x = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Pattern: "func main()", UseRegex: false}
	matches, err := searchBuiltinAll(t, "repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestSearchBuiltin_FixedStringDoesNotInterpretRegex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("price is $10.00\nprice is 10000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Pattern: "$10.00", UseRegex: false}
	matches, err := searchBuiltinAll(t, "repo", dir, opts)
	if err != nil {
		t.Fatalf("searchBuiltin() error = %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1 (should not match regex-special chars literally)", len(matches))
	}
}

// searchBuiltinAll runs the builtin engine over the whole repository, the way
// searchRepo does, so tests do not have to enumerate files themselves.
func searchBuiltinAll(t *testing.T, repoName, repoPath string, opts Options) ([]Match, error) {
	t.Helper()
	files, err := listFiles(repoPath, opts.Glob)
	if err != nil {
		return nil, err
	}
	return searchBuiltin(repoName, repoPath, files, opts)
}

// buildParityFixture creates a git repository holding one file of every kind
// the two engines used to disagree about.
func buildParityFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()

	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write(".gitignore", "build/\n")
	write("top.go", "needle here\n")
	write("sub/nested.go", "needle here\n")
	write("notes.txt", "needle here\n")
	write(".github/ci.yml", "needle here\n")     // tracked dotfile: searched
	write("build/generated.go", "needle here\n") // ignored: never searched
	write("bin.dat", "needle\x00here\n")         // binary: never searched

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init", "--no-gpg-sign"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func searchWithEngine(t *testing.T, useRipgrep bool, repoPath string, opts Options) []Match {
	t.Helper()
	original := isRipgrepAvailable
	isRipgrepAvailable = func() bool { return useRipgrep }
	t.Cleanup(func() { isRipgrepAvailable = original })

	if useRipgrep {
		if _, err := exec.LookPath("rg"); err != nil {
			t.Skip("ripgrep not installed")
		}
	}
	matches, err := searchRepo("repo", repoPath, opts)
	if err != nil {
		t.Fatalf("searchRepo() error = %v", err)
	}
	// Deliberately not sorted here: searchRepo must return a deterministic
	// order by itself, whichever engine ran.
	return matches
}

func summarize(matches []Match) string {
	var lines []string
	for _, m := range matches {
		lines = append(lines, fmt.Sprintf("%s:%d:%t:%s", m.File, m.Line, m.IsContext, m.Content))
	}
	return strings.Join(lines, "\n")
}

// TestSearchRepo_EnginesAgree is the point of sharing one file list: the
// results must not depend on whether ripgrep is installed.
func TestSearchRepo_EnginesAgree(t *testing.T) {
	dir := buildParityFixture(t)

	cases := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "whole repository",
			opts: Options{Pattern: "needle"},
			want: ".github/ci.yml:1:false:needle here\n" +
				"notes.txt:1:false:needle here\n" +
				"sub/nested.go:1:false:needle here\n" +
				"top.go:1:false:needle here",
		},
		{
			name: "glob without a separator matches at any depth",
			opts: Options{Pattern: "needle", Glob: "*.go"},
			want: "sub/nested.go:1:false:needle here\n" +
				"top.go:1:false:needle here",
		},
		{
			name: "glob with a separator matches the path",
			opts: Options{Pattern: "needle", Glob: "sub/*.go"},
			want: "sub/nested.go:1:false:needle here",
		},
		{
			name: "repeated ripgrep runs keep the order",
			opts: Options{Pattern: "needle", Glob: "*.go"},
			want: "sub/nested.go:1:false:needle here\n" +
				"top.go:1:false:needle here",
		},
		{
			name: "regex",
			opts: Options{Pattern: `need\w+`, UseRegex: true, Glob: "top.go"},
			want: "top.go:1:false:needle here",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builtin := summarize(searchWithEngine(t, false, dir, tc.opts))
			ripgrep := summarize(searchWithEngine(t, true, dir, tc.opts))

			if builtin != tc.want {
				t.Errorf("builtin engine =\n%s\nwant:\n%s", builtin, tc.want)
			}
			if ripgrep != builtin {
				t.Errorf("engines disagree:\nripgrep:\n%s\nbuiltin:\n%s", ripgrep, builtin)
			}
		})
	}
}

func TestSearchRepo_BatchesLargeFileLists(t *testing.T) {
	dir := buildParityFixture(t)
	// Force many ripgrep invocations to prove batching does not lose or
	// duplicate results.
	files, err := listFiles(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(batchFiles(files, 8)) < 2 {
		t.Fatalf("fixture should split into several batches, got %d", len(batchFiles(files, 8)))
	}

	want := summarize(searchWithEngine(t, false, dir, Options{Pattern: "needle"}))
	got := summarize(searchWithEngine(t, true, dir, Options{Pattern: "needle"}))
	if got != want {
		t.Errorf("batched ripgrep run =\n%s\nwant:\n%s", got, want)
	}
}

// TestSearch_JobsDoesNotChangeResults is the point of running repositories
// concurrently: --jobs is a speed knob, so the matches must come out in
// repos.yaml order whatever order the workers finish in.
func TestSearch_JobsDoesNotChangeResults(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	wsDir := t.TempDir()

	var repos []config.Repository
	for _, name := range []string{"alpha", "bravo", "charlie", "delta"} {
		initRepoWithNeedle(t, filepath.Join(wsDir, name))
		repos = append(repos, config.Repository{Name: name, Path: name})
	}
	// Absent from the workspace: skipped, not searched and not failed.
	repos = append(repos, config.Repository{Name: "missing", Path: "missing"})
	cfg := &config.Config{Repositories: repos}

	want := searchSummary(t, cfg, wsDir, 1)
	if want == "" {
		t.Fatal("fixture produced no matches")
	}
	for _, jobs := range []int{2, 4, 16} {
		if got := searchSummary(t, cfg, wsDir, jobs); got != want {
			t.Errorf("jobs=%d matches =\n%s\nwant:\n%s", jobs, got, want)
		}
	}
}

// TestSearch_JobsKeepsErrorOrder covers the other half of that guarantee: a
// repository that cannot be searched is reported in configuration order too, so
// warnings and JSON failures do not depend on which worker finished first.
func TestSearch_JobsKeepsErrorOrder(t *testing.T) {
	names := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	failing := map[string]bool{"bravo": true, "delta": true}

	original := searchRepoFunc
	t.Cleanup(func() { searchRepoFunc = original })
	searchRepoFunc = func(repoName, _ string, _ Options) ([]Match, error) {
		// Later repositories finish first, so completion order is reversed.
		time.Sleep(time.Duration(len(names)-slices.Index(names, repoName)) * 2 * time.Millisecond)
		if failing[repoName] {
			return nil, fmt.Errorf("cannot search %s", repoName)
		}
		return []Match{{Repo: repoName, File: "a.go", Line: 1, Content: "needle"}}, nil
	}

	wsDir := t.TempDir()
	var repos []config.Repository
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(wsDir, name), 0755); err != nil {
			t.Fatal(err)
		}
		repos = append(repos, config.Repository{Name: name, Path: name})
	}
	cfg := &config.Config{Repositories: repos}

	for _, jobs := range []int{1, 2, 5} {
		var failed []string
		matches, err := Search(cfg, wsDir, Options{
			Pattern:     "needle",
			Jobs:        jobs,
			OnRepoError: func(repo string, _ error) { failed = append(failed, repo) },
		})
		if err != nil {
			t.Fatalf("Search(jobs=%d) error = %v", jobs, err)
		}

		var searched []string
		for _, m := range matches {
			searched = append(searched, m.Repo)
		}
		if want := []string{"alpha", "charlie", "echo"}; !slices.Equal(searched, want) {
			t.Errorf("jobs=%d matched repos = %v, want %v", jobs, searched, want)
		}
		if want := []string{"bravo", "delta"}; !slices.Equal(failed, want) {
			t.Errorf("jobs=%d failed repos = %v, want %v", jobs, failed, want)
		}
	}
}

func searchSummary(t *testing.T, cfg *config.Config, wsDir string, jobs int) string {
	t.Helper()
	matches, err := Search(cfg, wsDir, Options{Pattern: "needle", Jobs: jobs})
	if err != nil {
		t.Fatalf("Search(jobs=%d) error = %v", jobs, err)
	}
	var lines []string
	for _, m := range matches {
		lines = append(lines, fmt.Sprintf("%s/%s:%d:%s", m.Repo, m.File, m.Line, m.Content))
	}
	return strings.Join(lines, "\n")
}

func initRepoWithNeedle(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"a.go", "sub/b.go"} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("needle here\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init"},
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init", "--no-gpg-sign"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
