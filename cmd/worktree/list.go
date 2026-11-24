package worktree

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/github"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	listRepos  []string
	listTags   []string
	listBranch string
	listPR     bool
	listJSON   bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees across repositories",
	Long: `List the worktrees of the repositories in repos.yaml.

The main checkout of each repository is not listed — that one is managed by
'xr repo sync'. Worktrees created outside xr are listed too, since the listing
comes from 'git worktree list' rather than from repos.yaml.

Use --branch to reconstruct the view of a single task from a branch naming
convention.

STATUS uses the same markers as 'xr repo list' (* modified, + staged, % untracked,
$ stash, < behind, > ahead), and ! for a worktree whose directory is missing.

--pr adds the latest pull request of each branch on GitHub: its number, state
(open, draft, merged, closed) and the result of its checks (passing, failing,
pending). Pull requests from forks are ignored. It reads GitHub through the gh
CLI, so it needs gh installed and logged in, and fails when XR_NO_GH is set. A
repository whose origin is not on GitHub shows ? and a warning.

Examples:
  xr worktree list
  xr worktree list -b 'feat-x*'
  xr worktree list -t backend
  xr worktree list -r api --json
  xr worktree list -b 'feat-x*' --pr`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newManager(cmd)
		if err != nil {
			return err
		}
		repos, err := selectRepos(m, listRepos, listTags)
		if err != nil {
			return err
		}

		entries, err := m.List(repos, listBranch)
		if err != nil {
			return err
		}

		if listPR {
			if err := m.AttachPullRequests(entries); err != nil {
				return err
			}
		}

		if listJSON {
			// Marshal an empty listing as [] rather than null.
			if entries == nil {
				entries = []wt.Entry{}
			}
			return output.PrintJSON(output.CommandResult{
				Command: "worktree list",
				Summary: map[string]int{"worktrees": len(entries)},
				Data:    map[string]any{"worktrees": entries},
			})
		}

		if len(entries) == 0 {
			fmt.Println("No worktrees found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		header := "REPO\tBRANCH\tSTATUS\tPATH"
		if listPR {
			header = "REPO\tBRANCH\tSTATUS\tPR\tPATH"
		}
		if _, err := fmt.Fprintln(w, header); err != nil {
			return err
		}
		for _, e := range entries {
			branch := e.Branch
			if branch == "" {
				branch = "(detached)"
			}
			cols := []any{e.Repo, branch, e.Status}
			format := "%s\t%s\t%s\t%s\n"
			if listPR {
				cols = append(cols, prColumn(e))
				format = "%s\t%s\t%s\t%s\t%s\n"
			}
			cols = append(cols, displayPath(e.Path))
			if _, err := fmt.Fprintf(w, format, cols...); err != nil {
				return err
			}
		}
		if err := w.Flush(); err != nil {
			return err
		}
		printPRWarnings(entries)
		return nil
	},
}

// prColumn renders the PR column: "#12 open, passing", "-" without a pull
// request, "?" when the lookup failed.
func prColumn(e wt.Entry) string {
	switch {
	case e.PRError != "":
		return "?"
	case e.PR == nil:
		return "-"
	}
	state := e.PR.State
	if state == github.StateOpen && e.PR.Draft {
		state = "draft"
	}
	s := fmt.Sprintf("#%d %s", e.PR.Number, state)
	if e.PR.Checks != "" && e.PR.State == github.StateOpen {
		s += ", " + e.PR.Checks
	}
	return s
}

// printPRWarnings reports each failed lookup once per repository and message,
// on stderr, so a repository that is not on GitHub does not repeat the same
// warning for every worktree.
func printPRWarnings(entries []wt.Entry) {
	seen := map[string]bool{}
	for _, e := range entries {
		if e.PRError == "" {
			continue
		}
		key := e.Repo + "\x00" + e.PRError
		if seen[key] {
			continue
		}
		seen[key] = true
		output.PrintWarning(fmt.Sprintf("%s: pull request lookup failed: %s", e.Repo, e.PRError))
	}
}

func init() {
	listCmd.Flags().StringArrayVarP(&listRepos, "repo", "r", nil, "limit to a repository (repeatable)")
	listCmd.Flags().StringArrayVarP(&listTags, "tag", "t", nil, "limit to repos with any of these tags (repeatable)")
	listCmd.Flags().StringVarP(&listBranch, "branch", "b", "", "filter branches by glob pattern (e.g. 'feat-x*')")
	listCmd.Flags().BoolVar(&listPR, "pr", false, "show the latest pull request of each branch (uses gh)")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")
	cobra.CheckErr(listCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}
