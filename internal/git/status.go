package git

import (
	"fmt"
	"os"
	"strings"
)

const (
	StatusClean = " "
)

const (
	statusModified  = "*"
	statusStaged    = "+"
	statusNoHead    = "#"
	statusStash     = "$"
	statusUntracked = "%"
)

type Snapshot struct {
	CurrentBranch string
	Status        string
}

func IsDirty(repoPath string) (bool, error) {
	flags, err := gitWorktreeState(repoPath)
	if err != nil {
		return false, err
	}
	return flags.hasModified || flags.hasStaged || flags.hasUntracked, nil
}

type worktreeFlags struct {
	hasStaged    bool
	hasUntracked bool
	hasModified  bool
}

func Inspect(repoPath string) (*Snapshot, error) {
	if _, err := os.Stat(repoPath); err != nil {
		return nil, err
	}

	isWorktree, err := isGitWorktree(repoPath)
	if err != nil {
		return nil, err
	}
	if !isWorktree {
		return nil, fmt.Errorf("not a git worktree")
	}

	currentBranch, err := gitCurrentBranch(repoPath)
	if err != nil {
		return nil, err
	}

	status, err := buildStatus(repoPath)
	if err != nil {
		return nil, err
	}
	if status == "" {
		status = StatusClean
	}

	return &Snapshot{
		CurrentBranch: currentBranch,
		Status:        status,
	}, nil
}

func buildStatus(repoPath string) (string, error) {
	flags, err := gitWorktreeState(repoPath)
	if err != nil {
		return "", err
	}

	hasHead, err := gitHasHead(repoPath)
	if err != nil {
		return "", err
	}
	hasStash, err := gitHasStash(repoPath)
	if err != nil {
		return "", err
	}
	upstreamMark, err := gitUpstreamMark(repoPath)
	if err != nil {
		return "", err
	}

	return composeStatus(flags, hasHead, hasStash, upstreamMark), nil
}

func composeStatus(flags worktreeFlags, hasHead bool, hasStash bool, upstreamMark string) string {
	status := ""
	if flags.hasModified {
		status += statusModified
	}
	if flags.hasStaged {
		status += statusStaged
	} else if !hasHead {
		status += statusNoHead
	}
	if hasStash {
		status += statusStash
	}
	if flags.hasUntracked {
		status += statusUntracked
	}
	status += upstreamMark
	return status
}

func gitWorktreeState(repoPath string) (worktreeFlags, error) {
	out, err := runGitOutput(repoPath, "status", "--porcelain")
	if err != nil {
		return worktreeFlags{}, err
	}
	return parsePorcelainFlags(string(out)), nil
}

func parsePorcelainFlags(porcelain string) (flags worktreeFlags) {
	lines := strings.Split(porcelain, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if len(line) < 2 {
			continue
		}

		x := line[0]
		y := line[1]

		if x != ' ' && x != '?' {
			flags.hasStaged = true
		}
		if x == '?' && y == '?' {
			flags.hasUntracked = true
		}
		if y != ' ' && y != '?' {
			flags.hasModified = true
		}
	}
	return flags
}

