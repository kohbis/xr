package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

const (
	statusError     = "!"
	statusClean     = " "
	statusModified  = "*"
	statusStaged    = "+"
	statusNoHead    = "#"
	statusStash     = "$"
	statusUntracked = "%"
)

type worktreeFlags struct {
	hasStaged    bool
	hasUntracked bool
	hasModified  bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List repositories in the workspace",
	Long:  `List all repositories defined in repos.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}
		wsDir, err := filepath.Abs(cfg.Workspace)
		if err != nil {
			return fmt.Errorf("resolving workspace path: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "NAME\tTYPE\tBRANCH\tCURRENT\tSTATUS\tPATH\tSOURCE"); err != nil {
			return err
		}
		for _, r := range cfg.Repositories {
			repoPath := filepath.Join(wsDir, r.Path)
			current, status := repoRuntimeStatus(repoPath)
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Name, r.Type, r.Branch, current, status, r.Path, r.Source); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

func repoRuntimeStatus(repoPath string) (currentBranch string, status string) {
	if _, err := os.Stat(repoPath); err != nil {
		return "-", statusError
	}

	// Non-git directories are treated as unavailable/error state.
	isWorktree, err := isGitWorktree(repoPath)
	if err != nil || !isWorktree {
		return "-", statusError
	}

	currentBranch, err = gitCurrentBranch(repoPath)
	if err != nil {
		return "-", statusError
	}

	status, err = buildStatus(repoPath)
	if err != nil {
		return currentBranch, statusError
	}
	if status == "" {
		status = statusClean
	}
	return currentBranch, status
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
	// Keep leading spaces per line. In porcelain format, a leading space in X
	// means "no staged change", so trimming whole output would corrupt status.
	lines := strings.Split(porcelain, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if len(line) < 2 {
			continue
		}

		// Porcelain format: XY <path>
		x := line[0]
		y := line[1]

		// Index changes (staged but not committed yet).
		if x != ' ' && x != '?' {
			flags.hasStaged = true
		}

		if x == '?' && y == '?' {
			flags.hasUntracked = true
		}

		// Tracked worktree changes (including delete/rename/typechange).
		if y != ' ' && y != '?' {
			flags.hasModified = true
		}
	}
	return flags
}

func gitHasStash(repoPath string) (bool, error) {
	return gitRefExists(repoPath, "refs/stash")
}

func gitHasHead(repoPath string) (bool, error) {
	return gitRefExists(repoPath, "HEAD")
}

func gitRefExists(repoPath, ref string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func gitCurrentBranch(repoPath string) (string, error) {
	out, err := runGitOutput(repoPath, "branch", "--show-current")
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
	out, err := runGitOutput(repoPath, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func runGitOutput(repoPath string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	return cmd.Output()
}

func init() {
	Cmd.AddCommand(listCmd)
}
