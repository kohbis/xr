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
