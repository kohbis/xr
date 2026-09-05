package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func CurrentBranch(repoPath string) (string, error) {
	if _, err := os.Stat(repoPath); err != nil {
		return "", err
	}
	isWorktree, err := isGitWorktree(repoPath)
	if err != nil {
		return "", err
	}
	if !isWorktree {
		return "", fmt.Errorf("not a git worktree")
	}
	return gitCurrentBranch(repoPath)
}

func RemoteURL(repoPath string) (string, error) {
	out, err := runGitOutput(repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func ShortCommit(repoPath string) (string, error) {
	out, err := runGitOutput(repoPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func CheckIgnore(repoPath string, path string) (bool, error) {
	err := Run(repoPath, "check-ignore", "-q", "--", path)
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// ListFiles returns the paths, relative to repoPath, of the files git knows
// about in the repository: tracked files plus untracked files that are not
// ignored. It is the file set a user would expect a cross-repository scan to
// cover, since ignored build output and vendored trees are excluded.
func ListFiles(repoPath string) ([]string, error) {
	out, err := runGitOutput(repoPath, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	return files, nil
}
