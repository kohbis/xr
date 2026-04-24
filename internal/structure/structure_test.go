package structure

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		// Some sandboxes disallow writes outside the workspace directory, which can
		// make git init fail even though the code works in real repos.
		t.Skipf("git init unavailable in this environment: %v (%s)", err, string(out))
	}
}

func TestAnalyzeRepo_BasicStructure(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "my-repo")

	if err := os.MkdirAll(filepath.Join(repoDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "src", "lib.go"), []byte("package src\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := AnalyzeRepo("my-repo", repoDir, 0)
	if err != nil {
		t.Fatalf("AnalyzeRepo() error = %v", err)
	}

	if info.Name != "my-repo" {
		t.Errorf("Name = %q, want %q", info.Name, "my-repo")
	}
	if info.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", info.FileCount)
	}
}

func TestAnalyzeRepo_DetectsLanguageFromDepFile(t *testing.T) {
	tests := []struct {
		depFile  string
		wantLang string
	}{
		{"go.mod", "Go"},
		{"package.json", "Node.js"},
		{"Cargo.toml", "Rust"},
		{"requirements.txt", "Python"},
		{"pom.xml", "Java"},
		{"Gemfile", "Ruby"},
		{"composer.json", "PHP"},
	}

	for _, tt := range tests {
		t.Run(tt.depFile, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.depFile), []byte("content\n"), 0644); err != nil {
				t.Fatal(err)
			}

			info, err := AnalyzeRepo("repo", dir, 0)
			if err != nil {
				t.Fatalf("AnalyzeRepo() error = %v", err)
			}

			if info.Language != tt.wantLang {
				t.Errorf("Language = %q, want %q", info.Language, tt.wantLang)
			}
		})
	}
}

func TestAnalyzeRepo_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hidden, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "config"), []byte("data\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("data\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := AnalyzeRepo("repo", dir, 0)
	if err != nil {
		t.Fatalf("AnalyzeRepo() error = %v", err)
	}

	if info.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (hidden dir contents should be skipped)", info.FileCount)
	}
}

func TestAnalyzeRepo_SkipsIgnoredByGitignore(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\nignored-dir/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "ignored-dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored-dir", "ignored.js"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("data\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := AnalyzeRepo("repo", dir, 0)
	if err != nil {
		t.Fatalf("AnalyzeRepo() error = %v", err)
	}

	if info.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (gitignored contents should be skipped)", info.FileCount)
	}
	for _, child := range info.Children {
		if child.Name == "ignored.txt" || child.Name == "ignored-dir" {
			t.Errorf("gitignored entry %q should be skipped from the tree", child.Name)
		}
	}
}

func TestAnalyzeRepo_MaxDepth(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "level1.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "level2.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "level3.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := AnalyzeRepo("repo", dir, 2)
	if err != nil {
		t.Fatalf("AnalyzeRepo() error = %v", err)
	}

	// depth 0: root.txt, a/  |  depth 1: level1.txt, b/  |  depth 2 is cut off
	if info.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2 (depth limit should stop at level 2)", info.FileCount)
	}
}

func TestAnalyzeRepo_DepFileMarkedIsDep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := AnalyzeRepo("repo", dir, 0)
	if err != nil {
		t.Fatalf("AnalyzeRepo() error = %v", err)
	}

	foundDep := false
	for _, child := range info.Children {
		if child.Name == "go.mod" && child.IsDep {
			foundDep = true
		}
		if child.Name == "main.go" && child.IsDep {
			t.Error("main.go should not be marked as dependency file")
		}
	}
	if !foundDep {
		t.Error("go.mod should be marked as dependency file")
	}
}

func capturePrintTree(t *testing.T, info *RepoInfo) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	PrintTree(info)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestPrintTree_Output(t *testing.T) {
	info := &RepoInfo{
		Name:      "test-repo",
		Language:  "Go",
		FileCount: 3,
		Branch:    "main",
		Commit:    "abc123",
		Children: []*Node{
			{Name: "src", IsDir: true, Children: []*Node{
				{Name: "main.go", IsDir: false},
			}},
			{Name: "go.mod", IsDir: false, IsDep: true},
			{Name: "README.md", IsDir: false},
		},
	}

	output := capturePrintTree(t, info)

	expected := "test-repo [Go] (main abc123)"
	if !bytes.Contains([]byte(output), []byte(expected)) {
		t.Errorf("PrintTree output missing header. got:\n%s", output)
	}
}

func TestNode_SortingDirsFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "z-dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a-file.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := AnalyzeRepo("repo", dir, 0)
	if err != nil {
		t.Fatalf("AnalyzeRepo() error = %v", err)
	}

	if len(info.Children) < 2 {
		t.Fatalf("expected at least 2 children, got %d", len(info.Children))
	}

	first := info.Children[0]
	if !first.IsDir {
		t.Errorf("first child should be dir (dirs sorted first), got file %q", first.Name)
	}
}

func TestDepFiles_AllMapped(t *testing.T) {
	expected := map[string]string{
		"go.mod":           "Go",
		"go.sum":           "Go",
		"package.json":     "Node.js",
		"Cargo.toml":       "Rust",
		"requirements.txt": "Python",
		"pyproject.toml":   "Python",
		"pom.xml":          "Java",
		"build.gradle":     "Java",
		"Gemfile":          "Ruby",
		"composer.json":    "PHP",
	}

	for file, lang := range expected {
		got, ok := depFiles[file]
		if !ok {
			t.Errorf("depFiles missing %q", file)
			continue
		}
		if got != lang {
			t.Errorf("depFiles[%q] = %q, want %q", file, got, lang)
		}
	}

	if len(depFiles) != len(expected) {
		// Print the diff
		for k := range depFiles {
			if _, ok := expected[k]; !ok {
				fmt.Printf("unexpected depFile: %q\n", k)
			}
		}
		t.Errorf("depFiles has %d entries, want %d", len(depFiles), len(expected))
	}
}
