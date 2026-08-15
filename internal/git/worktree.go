package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Worktree describes a single entry of `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	Head     string
	Branch   string // short branch name; empty when detached or bare
	Detached bool
	Bare     bool
	Locked   bool
	Prunable bool
	// Main reports whether this is the primary worktree of the repository
	// (the first entry git reports), as opposed to a linked worktree.
	Main bool
}

// Worktrees lists the worktrees registered for the repository at repoPath,
// including the main worktree.
func Worktrees(repoPath string) ([]Worktree, error) {
	out, err := RunOutput(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreePorcelain(string(out)), nil
}

// WorktreeAddOptions configures WorktreeAdd.
type WorktreeAddOptions struct {
	// CreateBranch creates the branch as part of adding the worktree
	// (git worktree add -b <branch> <dir> <Base>).
	CreateBranch bool
	// Base is the start point used when CreateBranch is set. Empty means HEAD.
	Base string
	// Track sets up the new branch to track Base as its upstream.
	Track bool
}

// WorktreeAdd creates a worktree for branch at dir.
func WorktreeAdd(repoPath, dir, branch string, opts WorktreeAddOptions) error {
	args := []string{"worktree", "add"}
	if opts.CreateBranch {
		args = append(args, "-b", branch)
		if opts.Track {
			args = append(args, "--track")
		}
		args = append(args, dir)
		if opts.Base != "" {
			args = append(args, opts.Base)
		}
	} else {
		args = append(args, dir, branch)
	}
	return RunQuiet(repoPath, args...)
}

// WorktreeRemove removes the worktree at dir. force discards uncommitted changes.
func WorktreeRemove(repoPath, dir string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, dir)
	return RunQuiet(repoPath, args...)
}

// WorktreePrune drops administrative entries for worktrees whose directory is
// gone, and returns git's verbose report of what was (or, with dryRun, would be)
// removed. An empty report means there was nothing to prune.
func WorktreePrune(repoPath string, dryRun bool) (string, error) {
	args := []string{"worktree", "prune", "--verbose"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	out, err := RunCombinedOutput(repoPath, args...)
	report := strings.TrimSpace(string(out))
	if err != nil {
		if report != "" {
			return "", fmt.Errorf("%s", report)
		}
		return "", err
	}
	return report, nil
}

// RefExists reports whether ref resolves in the repository at repoPath.
func RefExists(repoPath, ref string) (bool, error) {
	err := Run(repoPath, "rev-parse", "--verify", "--quiet", ref)
	if err == nil {
		return true, nil
	}
	// rev-parse exits with 1 when the ref is missing.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// UpstreamGone reports whether branch has a configured upstream that no longer
// exists on the remote. Branches without an upstream return false.
func UpstreamGone(repoPath, branch string) (bool, error) {
	out, err := RunOutput(repoPath, "for-each-ref", "--format=%(upstream:track)", "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "[gone]", nil
}

// parseWorktreePorcelain parses the output of `git worktree list --porcelain`.
// Records are separated by blank lines; the first record is the main worktree.
func parseWorktreePorcelain(out string) []Worktree {
	var (
		worktrees []Worktree
		current   *Worktree
	)

	flush := func() {
		if current != nil {
			worktrees = append(worktrees, *current)
			current = nil
		}
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}

		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			current = &Worktree{Path: value, Main: len(worktrees) == 0}
		case "HEAD":
			if current != nil {
				current.Head = value
			}
		case "branch":
			if current != nil {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "detached":
			if current != nil {
				current.Detached = true
			}
		case "bare":
			if current != nil {
				current.Bare = true
			}
		case "locked":
			if current != nil {
				current.Locked = true
			}
		case "prunable":
			if current != nil {
				current.Prunable = true
			}
		}
	}
	flush()

	return worktrees
}
