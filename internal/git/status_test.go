package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePorcelainFlags(t *testing.T) {
	t.Parallel()

	porcelain := " M README.md\nM  staged.txt\n?? new.txt\nD  removed.txt\n"
	flags := parsePorcelainFlags(porcelain)

	if !flags.hasModified {
		t.Fatalf("expected hasModified=true")
	}
	if !flags.hasStaged {
		t.Fatalf("expected hasStaged=true")
	}
	if !flags.hasUntracked {
		t.Fatalf("expected hasUntracked=true")
	}
}

func TestComposeStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		flags        worktreeFlags
		hasHead      bool
		hasStash     bool
		upstreamMark string
		want         string
	}{
		{
			name:         "clean with upstream equal",
			flags:        worktreeFlags{},
			hasHead:      true,
			hasStash:     false,
			upstreamMark: "=",
			want:         "=",
		},
		{
			name:         "modified staged untracked ahead",
			flags:        worktreeFlags{hasModified: true, hasStaged: true, hasUntracked: true},
			hasHead:      true,
			hasStash:     false,
			upstreamMark: ">",
			want:         "*+%>",
		},
		{
			name:         "no head and stash",
			flags:        worktreeFlags{},
			hasHead:      false,
			hasStash:     true,
			upstreamMark: "",
			want:         "#$",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := composeStatus(tt.flags, tt.hasHead, tt.hasStash, tt.upstreamMark)
			if got != tt.want {
				t.Fatalf("composeStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitUpstreamMark(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		repoPath := setupTrackedRepo(t)
		got, err := gitUpstreamMark(repoPath)
		if err != nil {
			t.Fatalf("gitUpstreamMark() error = %v", err)
		}
		if got != "=" {
			t.Fatalf("gitUpstreamMark() = %q, want %q", got, "=")
		}
	})

	t.Run("ahead", func(t *testing.T) {
		repoPath := setupTrackedRepo(t)
		writeFile(t, repoPath, "ahead.txt", "ahead\n")
		runGit(t, repoPath, "add", "ahead.txt")
		runGit(t, repoPath, "commit", "-m", "ahead commit")

		got, err := gitUpstreamMark(repoPath)
		if err != nil {
			t.Fatalf("gitUpstreamMark() error = %v", err)
		}
		if got != ">" {
			t.Fatalf("gitUpstreamMark() = %q, want %q", got, ">")
		}
	})

	t.Run("behind", func(t *testing.T) {
		repoPath := setupTrackedRepo(t)
		remotePath := remotePathFromOrigin(t, repoPath)
		pushedPath := setupCloneFromRemote(t, remotePath, "pusher")

		writeFile(t, pushedPath, "behind.txt", "behind\n")
		runGit(t, pushedPath, "add", "behind.txt")
		runGit(t, pushedPath, "commit", "-m", "remote commit")
		runGit(t, pushedPath, "push", "origin", "main")
		runGit(t, repoPath, "fetch", "origin")

		got, err := gitUpstreamMark(repoPath)
		if err != nil {
			t.Fatalf("gitUpstreamMark() error = %v", err)
		}
		if got != "<" {
			t.Fatalf("gitUpstreamMark() = %q, want %q", got, "<")
		}
	})

	t.Run("diverged", func(t *testing.T) {
		repoPath := setupTrackedRepo(t)
		remotePath := remotePathFromOrigin(t, repoPath)
		pushedPath := setupCloneFromRemote(t, remotePath, "pusher")

		writeFile(t, repoPath, "local.txt", "local\n")
		runGit(t, repoPath, "add", "local.txt")
		runGit(t, repoPath, "commit", "-m", "local commit")

		writeFile(t, pushedPath, "remote.txt", "remote\n")
		runGit(t, pushedPath, "add", "remote.txt")
		runGit(t, pushedPath, "commit", "-m", "remote commit")
		runGit(t, pushedPath, "push", "origin", "main")
		runGit(t, repoPath, "fetch", "origin")

		got, err := gitUpstreamMark(repoPath)
		if err != nil {
			t.Fatalf("gitUpstreamMark() error = %v", err)
		}
		if got != "<>" {
			t.Fatalf("gitUpstreamMark() = %q, want %q", got, "<>")
		}
	})
}

func setupTrackedRepo(t *testing.T) string {
	t.Helper()

	base := t.TempDir()
	remotePath := filepath.Join(base, "remote.git")
	runGit(t, base, "init", "--bare", remotePath)

	seedPath := filepath.Join(base, "seed")
	runGit(t, base, "init", seedPath)
	configGitIdentity(t, seedPath)
	writeFile(t, seedPath, "README.md", "seed\n")
	runGit(t, seedPath, "add", "README.md")
	runGit(t, seedPath, "commit", "-m", "initial commit")
	runGit(t, seedPath, "branch", "-M", "main")
	runGit(t, seedPath, "remote", "add", "origin", remotePath)
	runGit(t, seedPath, "push", "-u", "origin", "main")

	return setupCloneFromRemote(t, remotePath, "work")
}

func setupCloneFromRemote(t *testing.T, remotePath, name string) string {
	t.Helper()

	clonePath := filepath.Join(filepath.Dir(remotePath), name)
	runGit(t, filepath.Dir(remotePath), "clone", "-b", "main", "--single-branch", remotePath, clonePath)
	configGitIdentity(t, clonePath)
	return clonePath
}

func configGitIdentity(t *testing.T, repoPath string) {
	t.Helper()
	runGit(t, repoPath, "config", "user.name", "xr-test")
	runGit(t, repoPath, "config", "user.email", "xr-test@example.com")
}

func remotePathFromOrigin(t *testing.T, repoPath string) string {
	t.Helper()
	out := runGit(t, repoPath, "remote", "get-url", "origin")
	return strings.TrimSpace(out)
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (dir=%s): %v\n%s", strings.Join(args, " "), dir, err, string(out))
	}
	return string(out)
}
