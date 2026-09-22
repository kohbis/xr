package pathsafe

import (
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
		// The reason whole elements are compared rather than a string prefix:
		// "..foo" starts with ".." but never leaves dir.
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

// Relative paths are resolved against the working directory, so a relative dir
// and an absolute path under it are still recognised as contained.
func TestInsideRelativeDir(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("repos", "api"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Inside("repos", abs); err != nil {
		t.Errorf("Inside(%q, %q) error = %v, want nil", "repos", abs, err)
	}
}
