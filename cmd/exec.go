package cmd

import (
	"fmt"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/parallel"
	"github.com/kohbis/xr/internal/runner"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/spf13/cobra"
)

var (
	execRepo     []string
	execTags     []string
	execJobs     int
	execJSON     bool
	execFailFast bool
	execWorktree string
)

var execCmd = &cobra.Command{
	Use:     "exec [flags] -- <command> [args...]",
	Short:   "Run a command in every repository",
	GroupID: "cross",
	Long: `Run one command in each repository of the workspace.

The command runs directly, without a shell, so its arguments are passed through
unchanged. Use an explicit shell for pipelines or globbing:

  xr exec -- bash -c 'go build ./... 2>&1 | tail -5'

Each repository runs with XR_REPO_NAME and XR_REPO_PATH set in the environment.
Repositories missing from the workspace are skipped, not failed.

--worktree runs the command in worktrees instead of the workspace checkouts:
those of the selected repositories whose branch matches the glob, as listed by
'xr worktree list -b'. Each worktree is reported as <repo>@<branch>, which is
also what XR_REPO_NAME carries; XR_REPO_PATH is the worktree directory. -r and
-t still choose the repositories.

Exits non-zero when the command fails in any repository.

--jobs runs several repositories at once. Output stays grouped and ordered by
repository, but each block appears only once that repository finishes.

--fail-fast does not start the command in repositories that have not started
yet once it has failed in one. Repositories already running are left to finish,
so with --jobs which ones are cancelled depends on timing.

Examples:
  xr exec -- go test ./...
  xr exec --fail-fast -- go test ./...
  xr exec -r api -r web -- make lint
  xr exec -t go -r api -- go test ./...
  xr exec -j 8 -- git fetch --prune
  xr exec --worktree 'feat-x*' -- go test ./...
  xr exec --json -- git status --porcelain`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := parallel.ValidateJobs(execJobs); err != nil {
			return err
		}

		cfg, err := config.LoadCommand(rootCmd)
		if err != nil {
			return err
		}
		cfg = cfg.WithTags(execTags)
		cfg, wsDir, repoFilter, err := resolveTargets(cfg, execRepo, execWorktree)
		if err != nil {
			return err
		}

		result, err := runner.Run(cfg, wsDir, args, runner.Options{
			RepoFilter: repoFilter,
			Jobs:       execJobs,
			Quiet:      execJSON,
			FailFast:   execFailFast,
		})
		if err != nil {
			return err
		}

		if execJSON {
			if err := output.PrintJSON(execResult(args, result)); err != nil {
				return err
			}
		} else {
			output.PrintActionSummary("ok", result.Ran, result.Missing, result.Failed)
			if result.Cancelled > 0 {
				_, _ = fmt.Println(output.Dim(fmt.Sprintf("Cancelled: %d (--fail-fast)", result.Cancelled)))
			}
		}

		// Failures are already reported per repository.
		return exitcode.FailedIf(cmd, result.Failed)
	},
}

func execResult(args []string, result *runner.Result) output.CommandResult {
	repos := make([]output.RepoResult, 0, len(result.Runs))
	for _, r := range result.Runs {
		status := "ok"
		switch {
		case r.Missing:
			status = "skipped"
		case r.Cancelled:
			status = "cancelled"
		case r.Failed():
			status = "failed"
		}
		// A repository that never ran has no exit status to report.
		var metrics map[string]int
		if !r.Missing && !r.Cancelled {
			metrics = map[string]int{"exit_code": r.ExitCode}
		}
		repos = append(repos, output.RepoResult{
			Name:    r.Name,
			Status:  status,
			Error:   r.Error,
			Metrics: metrics,
		})
	}

	return output.CommandResult{
		Command: "exec",
		Summary: map[string]int{
			"ok":        result.Ran,
			"failed":    result.Failed,
			"skipped":   result.Missing,
			"cancelled": result.Cancelled,
		},
		Repos: repos,
		Data: map[string]any{
			"command": args,
			"runs":    result.Runs,
		},
	}
}

func init() {
	rootCmd.AddCommand(execCmd)
	// Stop parsing flags at the first argument so the command's own flags are
	// passed through rather than claimed by xr.
	execCmd.Flags().SetInterspersed(false)
	execCmd.Flags().StringArrayVarP(&execRepo, "repo", "r", nil, "limit execution to specific repos")
	execCmd.Flags().StringArrayVarP(&execTags, "tag", "t", nil, "limit execution to repos with any of these tags (repeatable)")
	execCmd.Flags().IntVarP(&execJobs, "jobs", "j", 1, "number of repositories to run concurrently")
	execCmd.Flags().BoolVar(&execJSON, "json", false, "output in JSON format")
	execCmd.Flags().BoolVar(&execFailFast, "fail-fast", false, "do not start the command in further repositories after one fails")
	registerWorktreeFlag(execCmd, &execWorktree, "run")
	cobra.CheckErr(execCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
	cobra.CheckErr(execCmd.RegisterFlagCompletionFunc("tag", shellcomp.CompleteTags))
}
