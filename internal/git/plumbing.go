package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func gitHasStash(repoPath string) (bool, error) {
	return RefExists(repoPath, "refs/stash")
}

func gitHasHead(repoPath string) (bool, error) {
	return RefExists(repoPath, "HEAD")
}

func gitCurrentBranch(repoPath string) (string, error) {
	out, err := RunOutput(repoPath, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "(detached)", nil
	}
	return branch, nil
}

func gitUpstreamMark(repoPath string) (string, error) {
	upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	upstreamCmd.Dir = repoPath
	if err := upstreamCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return "", nil
		}
		return "", err
	}

	aheadBehindCmd := exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	aheadBehindCmd.Dir = repoPath
	out, err := aheadBehindCmd.Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 2 {
		return "", fmt.Errorf("unexpected rev-list output: %q", strings.TrimSpace(string(out)))
	}
	ahead := fields[0] != "0"
	behind := fields[1] != "0"
	switch {
	case ahead && behind:
		return "<>", nil
	case ahead:
		return ">", nil
	case behind:
		return "<", nil
	default:
		return "=", nil
	}
}

func isGitWorktree(repoPath string) (bool, error) {
	out, err := RunOutput(repoPath, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}
