package pathsafe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInside(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		path    string
		wantErr bool
	}{
		{"direct child", "/workspace/repos", "/workspace/repos/my-repo", false},
		{"nested child", "/workspace/repos", "/workspace/repos/deep/nested", false},
		{"parent escape", "/workspace/repos", "/workspace/repos/../../etc", true},
		{"dir itself", "/workspace/repos", "/workspace/repos", true},
		{"parent", "/workspace/repos", "/workspace", true},
		{"outside path", "/workspace/worktrees", "/workspace/repos/api", true},
		{"prefix sibling directory", "/ws/wt", "/ws/wt-other/api", true},
		// Starts with "..", never leaves dir.
		{"child starting with dots", "/workspace/repos", "/workspace/repos/..foo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Inside(tt.dir, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Inside(%q, %q) error = %v, wantErr %v", tt.dir, tt.path, err, tt.wantErr)
			}
		})
	}
}

// A relative dir and an absolute path under it are both resolved first.
func TestInsideRelativeDir(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("repos", "api"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Inside("repos", abs); err != nil {
		t.Errorf("Inside(%q, %q) error = %v, want nil", "repos", abs, err)
	}
}

// A path that leaves through a symlinked parent is outside, however it reads.
func TestInside_SymlinkedParentEscapes(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(base, "repos")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	if err := Inside(dir, filepath.Join(dir, "link", "victim")); err == nil {
		t.Error("Inside() through a symlinked parent = nil, want error")
	}
}

// A symlink repository must stay removable.
func TestInside_SymlinkAsFinalComponentIsInside(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(base, "repos")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "my-repo")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	if err := Inside(dir, link); err != nil {
		t.Errorf("Inside() on a symlink repository = %v, want nil", err)
	}
}

// A worktree path is checked before it exists.
func TestInside_ResolvesWhatExistsOfAPathThatDoesNot(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(base, "worktrees")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "api")); err != nil {
		t.Fatal(err)
	}

	if err := Inside(dir, filepath.Join(dir, "api", "feature", "new-branch")); err == nil {
		t.Error("Inside() through a symlinked parent of a missing path = nil, want error")
	}
}
