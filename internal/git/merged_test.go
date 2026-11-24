package git

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// setupMergeRepo returns a repository with one commit on main.
func setupMergeRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := filepath.Join(t.TempDir(), "repo")
	runGit(t, filepath.Dir(dir), "init", "-b", "main", dir)
	configGitIdentity(t, dir)
	runGit(t, dir, "config", "commit.gpgsign", "false")
	writeFile(t, dir, "base.txt", "base\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	return dir
}

// commitOn checks out branch (creating it from main when create is set) and
// commits one file per name.
func commitOn(t *testing.T, dir, branch string, create bool, files ...string) {
	t.Helper()
	if create {
		runGit(t, dir, "checkout", "-q", "-b", branch, "main")
	} else {
		runGit(t, dir, "checkout", "-q", branch)
	}
	for _, f := range files {
		writeFile(t, dir, f, f+"\n")
		runGit(t, dir, "add", f)
		runGit(t, dir, "commit", "-q", "-m", f)
	}
}

func TestBranchMerged(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name: "no commits of its own",
			setup: func(t *testing.T, dir string) {
				runGit(t, dir, "branch", "feat", "main")
				commitOn(t, dir, "main", false, "later.txt")
			},
			want: false,
		},
		{
			name: "unmerged commits",
			setup: func(t *testing.T, dir string) {
				commitOn(t, dir, "feat", true, "a.txt")
			},
			want: false,
		},
		{
			name: "merge commit",
			setup: func(t *testing.T, dir string) {
				commitOn(t, dir, "feat", true, "a.txt", "b.txt")
				commitOn(t, dir, "main", false, "other.txt")
				runGit(t, dir, "merge", "-q", "--no-ff", "-m", "merge feat", "feat")
			},
			want: true,
		},
		{
			name: "squash merge",
			setup: func(t *testing.T, dir string) {
				commitOn(t, dir, "feat", true, "a.txt", "b.txt")
				commitOn(t, dir, "main", false, "other.txt")
				runGit(t, dir, "merge", "-q", "--squash", "feat")
				runGit(t, dir, "commit", "-q", "-m", "feat (#1)")
			},
			want: true,
		},
		{
			name: "rebase merge",
			setup: func(t *testing.T, dir string) {
				commitOn(t, dir, "feat", true, "a.txt", "b.txt")
				commitOn(t, dir, "main", false, "other.txt")
				runGit(t, dir, "cherry-pick", "main..feat")
			},
			want: true,
		},
		{
			name: "squash merged, then more work",
			setup: func(t *testing.T, dir string) {
				commitOn(t, dir, "feat", true, "a.txt")
				runGit(t, dir, "checkout", "-q", "main")
				runGit(t, dir, "merge", "-q", "--squash", "feat")
				runGit(t, dir, "commit", "-q", "-m", "feat (#1)")
				commitOn(t, dir, "feat", false, "more.txt")
			},
			want: false,
		},
		{
			name: "merge commit, then more work",
			setup: func(t *testing.T, dir string) {
				commitOn(t, dir, "feat", true, "a.txt")
				commitOn(t, dir, "main", false, "other.txt")
				runGit(t, dir, "merge", "-q", "--no-ff", "-m", "merge feat", "feat")
				commitOn(t, dir, "feat", false, "more.txt")
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupMergeRepo(t)
			tt.setup(t, dir)
			runGit(t, dir, "checkout", "-q", "main")

			got, err := BranchMerged(dir, "feat", "refs/heads/main")
			if err != nil {
				t.Fatalf("BranchMerged() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("BranchMerged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoteDefaultRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := setupTrackedRepo(t)
	runGit(t, dir, "remote", "set-head", "origin", "main")

	tests := []struct {
		branch string
		want   string
	}{
		{"main", "refs/remotes/origin/main"},
		{"", "refs/remotes/origin/main"},
		{"missing", "refs/remotes/origin/main"},
	}
	for _, tt := range tests {
		got, err := RemoteDefaultRef(dir, tt.branch)
		if err != nil {
			t.Fatalf("RemoteDefaultRef(%q) error = %v", tt.branch, err)
		}
		if got != tt.want {
			t.Errorf("RemoteDefaultRef(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}

	runGit(t, dir, "remote", "set-head", "origin", "--delete")
	got, err := RemoteDefaultRef(dir, "")
	if err != nil {
		t.Fatalf("RemoteDefaultRef without origin/HEAD error = %v", err)
	}
	if got != "" {
		t.Errorf("RemoteDefaultRef without origin/HEAD = %q, want empty", got)
	}
}
