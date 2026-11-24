package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// RemoteDefaultRef returns the remote-tracking ref that branches of the
// repository at repoPath are merged into: refs/remotes/origin/<branch> when
// branch is set and exists, otherwise the ref origin/HEAD points to. It returns
// "" when neither resolves.
func RemoteDefaultRef(repoPath, branch string) (string, error) {
	if branch != "" {
		ref := "refs/remotes/origin/" + branch
		ok, err := RefExists(repoPath, ref)
		if err != nil {
			return "", err
		}
		if ok {
			return ref, nil
		}
	}
	out, err := RunOutput(repoPath, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err != nil {
		// symbolic-ref --quiet exits with 1 when origin/HEAD is not set.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// BranchMerged reports whether the work on branch has landed in target, by any
// of the usual ways a pull request is merged:
//
//   - a merge commit: the branch tip is an ancestor of target, but not on its
//     first-parent line (that is where a branch that never got its own commits
//     sits, and it must not count as merged);
//   - a rebase merge: every commit on the branch has a commit with the same
//     patch on target;
//   - a squash merge: the branch's combined change since its merge base with
//     target matches the patch of a commit on target.
//
// A branch with commits beyond what landed is not merged, so work added after
// the merge is never reported as done. Errs on the side of "not merged": a
// branch fast-forwarded into target, or squashed with further edits made while
// merging, is not detected.
func BranchMerged(repoPath, branch, target string) (bool, error) {
	branchRef := "refs/heads/" + branch

	ancestor, err := isAncestor(repoPath, branchRef, target)
	if err != nil {
		return false, err
	}
	if ancestor {
		onLine, err := onFirstParentLine(repoPath, branchRef, target)
		if err != nil {
			return false, err
		}
		return !onLine, nil
	}

	rebased, err := allCommitsPicked(repoPath, branchRef, target)
	if err != nil || rebased {
		return rebased, err
	}
	return squashMerged(repoPath, branchRef, target)
}

// allCommitsPicked reports whether `git cherry` finds every commit of branch
// that is not in target already applied to target under another hash.
func allCommitsPicked(repoPath, branchRef, target string) (bool, error) {
	out, err := RunOutput(repoPath, "cherry", target, branchRef)
	if err != nil {
		return false, fmt.Errorf("cherry: %w", err)
	}
	report := strings.TrimSpace(string(out))
	if report == "" {
		return false, nil
	}
	for _, line := range strings.Split(report, "\n") {
		if !strings.HasPrefix(line, "- ") {
			return false, nil
		}
	}
	return true, nil
}

func isAncestor(repoPath, commit, target string) (bool, error) {
	err := Run(repoPath, "merge-base", "--is-ancestor", commit, target)
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("merge-base --is-ancestor: %w", err)
}

// onFirstParentLine reports whether commit appears when walking target's
// first parents. The walk stops at commit's parents, so it covers only the
// history after commit rather than all of target.
func onFirstParentLine(repoPath, commit, target string) (bool, error) {
	tip, err := revParse(repoPath, commit)
	if err != nil {
		return false, err
	}
	out, err := RunOutput(repoPath, "rev-list", "--first-parent", target, "--not", commit+"^@")
	if err != nil {
		return false, fmt.Errorf("rev-list: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == tip {
			return true, nil
		}
	}
	return false, nil
}

// squashMerged builds a throwaway commit holding the branch's whole change on
// top of its merge base with target, and asks `git cherry` whether a commit
// with the same patch exists on target.
func squashMerged(repoPath, branchRef, target string) (bool, error) {
	out, err := RunOutput(repoPath, "merge-base", target, branchRef)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil // unrelated histories
		}
		return false, fmt.Errorf("merge-base: %w", err)
	}
	base := strings.TrimSpace(string(out))

	tree, err := revParse(repoPath, branchRef+"^{tree}")
	if err != nil {
		return false, err
	}
	baseTree, err := revParse(repoPath, base+"^{tree}")
	if err != nil {
		return false, err
	}
	if tree == baseTree {
		return false, nil // no net change: nothing that could have been merged
	}

	// The identity only labels an unreferenced object; set it so the check
	// works without user.name / user.email configured.
	out, err = RunOutput(repoPath,
		"-c", "user.name=xr", "-c", "user.email=xr@localhost",
		"commit-tree", "--no-gpg-sign", tree, "-p", base, "-m", "xr merged check")
	if err != nil {
		return false, fmt.Errorf("commit-tree: %w", err)
	}
	squashed := strings.TrimSpace(string(out))

	out, err = RunOutput(repoPath, "cherry", target, squashed)
	if err != nil {
		return false, fmt.Errorf("cherry: %w", err)
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "-"), nil
}

func revParse(repoPath, rev string) (string, error) {
	out, err := RunOutput(repoPath, "rev-parse", "--verify", "--quiet", rev)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}
