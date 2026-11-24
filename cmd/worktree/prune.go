package worktree

import (
	"fmt"

	"github.com/kohbis/xr/internal/config"
	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	pruneRepos  []string
	pruneTags   []string
	pruneGone   bool
	pruneMerged bool
	pruneDryRun bool
	pruneJSON   bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Drop stale worktree entries and finished worktrees",
	Long: `Run 'git worktree prune' in each repository, dropping the administrative
entries of worktrees whose directory has been deleted outside of git.

With --merged, worktrees whose branch has been merged into the repository's
default branch on origin are removed as well: origin/<branch> from repos.yaml,
or origin/HEAD when no branch is set. Merge commits and squash or rebase merges
are recognized. A branch with commits that did not land, or with no commits of
its own yet, is kept. A branch fast-forwarded into the default branch, or
squashed with further edits made while merging, is not recognized and is kept.

With --gone, worktrees whose branch tracked a remote branch that no longer
exists are removed. This does not check that the work was merged: a branch
deleted on the remote after closing its pull request counts as gone too.

Both checks reflect the last fetch, so run 'xr repo sync --update --prune'
first. Removal requires confirmation or --yes, stops at worktrees with
uncommitted changes, and keeps the local branch.

Examples:
  xr worktree prune
  xr repo sync --update --prune && xr worktree prune --merged
  xr worktree prune --merged --dry-run`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newManager(cmd)
		if err != nil {
			return err
		}
		repos, err := selectRepos(m, pruneRepos, pruneTags)
		if err != nil {
			return err
		}

		combined := &wt.Result{}

		if pruneMerged || pruneGone {
			candidates, skipped, err := finishedEntries(m, repos)
			if err != nil {
				return err
			}
			combined.Outcomes = append(combined.Outcomes, skipped...)

			if len(candidates) == 0 && !pruneJSON {
				fmt.Println("No finished worktrees to remove.")
			}
			if len(candidates) > 0 {
				if !pruneJSON {
					fmt.Printf("The following worktree(s) are finished:\n")
					for _, c := range candidates {
						fmt.Printf("  - %-20s %-24s %-8s %s\n", c.entry.Repo, c.entry.Branch, c.reason, displayPath(c.entry.Path))
					}
				}
				if !pruneDryRun {
					confirmed, err := confirmRemoval(cmd)
					if err != nil {
						return err
					}
					if !confirmed {
						if pruneJSON {
							return reportResult(cmd, "worktree prune", combined, true)
						}
						fmt.Println("Aborted.")
						return nil
					}
				}
				entries := make([]wt.Entry, 0, len(candidates))
				for _, c := range candidates {
					entries = append(entries, c.entry)
				}
				result, err := m.Remove(entries, false, pruneDryRun)
				if err != nil {
					return err
				}
				combined.Outcomes = append(combined.Outcomes, result.Outcomes...)
			}
		}

		result, err := m.Prune(repos, pruneDryRun)
		if err != nil {
			return err
		}
		combined.Outcomes = append(combined.Outcomes, result.Outcomes...)

		return reportResult(cmd, "worktree prune", combined, pruneJSON)
	},
}

type finished struct {
	entry  wt.Entry
	reason string // "merged" or "gone"
}

// finishedEntries collects the worktrees selected by --merged and --gone, in
// that order, listing a worktree matched by both once as merged.
func finishedEntries(m *wt.Manager, repos []config.Repository) ([]finished, []wt.Outcome, error) {
	var (
		out     []finished
		skipped []wt.Outcome
		seen    = map[string]bool{}
	)
	add := func(entries []wt.Entry, reason string) {
		for _, e := range entries {
			if seen[e.Path] {
				continue
			}
			seen[e.Path] = true
			out = append(out, finished{entry: e, reason: reason})
		}
	}

	if pruneMerged {
		merged, s, err := m.MergedEntries(repos)
		if err != nil {
			return nil, nil, err
		}
		skipped = s
		add(merged, "merged")
	}
	if pruneGone {
		gone, err := m.GoneEntries(repos)
		if err != nil {
			return nil, nil, err
		}
		add(gone, "gone")
	}
	return out, skipped, nil
}

func init() {
	pruneCmd.Flags().StringArrayVarP(&pruneRepos, "repo", "r", nil, "limit to a repository (repeatable)")
	pruneCmd.Flags().StringArrayVarP(&pruneTags, "tag", "t", nil, "limit to repos with any of these tags (repeatable)")
	pruneCmd.Flags().BoolVar(&pruneMerged, "merged", false, "also remove worktrees whose branch is merged into origin's default branch")
	pruneCmd.Flags().BoolVar(&pruneGone, "gone", false, "also remove worktrees whose upstream branch is gone from origin")
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "preview only, remove nothing")
	pruneCmd.Flags().BoolVar(&pruneJSON, "json", false, "output in JSON format")
}
