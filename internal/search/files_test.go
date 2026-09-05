package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		glob string
		rel  string
		want bool
	}{
		{glob: "", rel: "any/file.txt", want: true},
		// Without a separator the pattern applies to the file name at any depth.
		{glob: "*.go", rel: "main.go", want: true},
		{glob: "*.go", rel: "cmd/deep/main.go", want: true},
		{glob: "*.go", rel: "main.rs", want: false},
		// With a separator it applies to the whole relative path.
		{glob: "cmd/*.go", rel: "cmd/main.go", want: true},
		{glob: "cmd/*.go", rel: "main.go", want: false},
		{glob: "cmd/*.go", rel: "internal/cmd/main.go", want: false},
		// A leading **/ is redundant under the rule above.
		{glob: "**/*.go", rel: "cmd/main.go", want: true},
		{glob: "**/*.go", rel: "main.go", want: true},
		{glob: "go.mod", rel: "go.mod", want: true},
		{glob: "go.mod", rel: "sub/go.mod", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.glob+"|"+tt.rel, func(t *testing.T) {
			got, err := matchGlob(tt.glob, tt.rel)
			if err != nil {
				t.Fatalf("matchGlob() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %t, want %t", tt.glob, tt.rel, got, tt.want)
			}
		})
	}
}

func TestMatchGlob_InvalidPattern(t *testing.T) {
	if _, err := matchGlob("[", "main.go"); err == nil {
		t.Fatal("expected an error for a malformed pattern")
	}
}

func TestListFiles_NonGitDirectorySkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := listFiles(dir, "")
	if err != nil {
		t.Fatalf("listFiles() error = %v", err)
	}
	if strings.Join(files, ",") != "a.txt,sub/b.txt" {
		t.Errorf("listFiles() = %v, want a.txt and sub/b.txt", files)
	}
}

func TestIsBinary(t *testing.T) {
	if !isBinary([]byte("text\x00more")) {
		t.Error("content with a NUL byte should be binary")
	}
	if isBinary([]byte("plain text\n")) {
		t.Error("plain text should not be binary")
	}
	// A NUL beyond the inspected prefix is not detected, as with ripgrep.
	long := append(make([]byte, binarySniffLen+10), 0)
	for i := range binarySniffLen + 10 {
		long[i] = 'a'
	}
	if isBinary(long) {
		t.Error("a NUL past the sniff prefix should not count")
	}
}
