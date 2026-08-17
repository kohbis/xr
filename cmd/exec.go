package cmd

import (
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/runner"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/spf13/cobra"
)

var (
	execRepo []string
	execJobs int
	execJSON bool
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

Exits non-zero when the command fails in any repository.

--jobs runs several repositories at once. Output stays grouped and ordered by
repository, but each block appears only once that repository finishes.

Examples:
  xr exec -- go test ./...
  xr exec -r api -r web -- make lint
  xr exec -j 8 -- git fetch --prune
  xr exec --json -- git status --porcelain`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		wsDir, err := resolveWorkspaceDir(cfg)
		if err != nil {
			return err
		}

		result, err := runner.Run(cfg, wsDir, args, runner.Options{
			RepoFilter: execRepo,
			Jobs:       execJobs,
			Quiet:      execJSON,
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
		}

		if result.Failed > 0 {
			// Failures are already reported per repository.
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return exitcode.Silent(1)
		}
		return nil
	},
}

func execResult(args []string, result *runner.Result) output.CommandResult {
	repos := make([]output.RepoResult, 0, len(result.Runs))
	for _, r := range result.Runs {
		status := "ok"
		switch {
		case r.Missing:
			status = "skipped"
		case r.Failed():
			status = "failed"
		}
		// A skipped repository never ran, so it has no exit status to report.
		var metrics map[string]int
		if !r.Missing {
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
			"ok":      result.Ran,
			"failed":  result.Failed,
			"skipped": result.Missing,
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
	execCmd.Flags().IntVarP(&execJobs, "jobs", "j", 1, "number of repositories to run concurrently")
	execCmd.Flags().BoolVar(&execJSON, "json", false, "output in JSON format")
	cobra.CheckErr(execCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}
