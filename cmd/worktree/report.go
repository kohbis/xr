package worktree

import (
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/output"
	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

func printOutcomes(result *wt.Result) {
	for _, o := range result.Outcomes {
		output.PrintSyncHeader(o.Repo, "worktree")
		switch o.Status {
		case wt.StatusCreated:
			if o.Detail != "" {
				output.PrintSyncAction(o.Detail)
			}
			output.PrintSyncOK(displayPath(o.Path))
		case wt.StatusRemoved:
			output.PrintSyncOK("removed " + displayPath(o.Path))
		case wt.StatusPruned:
			msg := "pruned stale worktree entries"
			if o.Detail != "" {
				msg += ": " + o.Detail
			}
			output.PrintSyncOK(msg)
		case wt.StatusPreview:
			msg := "preview"
			if o.Detail != "" {
				msg += ": " + o.Detail
			}
			if o.Path != "" {
				msg += " → " + displayPath(o.Path)
			}
			output.PrintSyncSkip(msg)
		case wt.StatusSkipped:
			output.PrintSyncSkip(o.Detail)
		case wt.StatusFailed:
			output.PrintSyncFail(o.Detail)
		}
	}
}

func jsonResult(command string, result *wt.Result) output.CommandResult {
	changed, skipped, failed := result.Counts()
	// Marshal an empty result as [] rather than null, so consumers can iterate
	// without a nil check.
	outcomes := result.Outcomes
	if outcomes == nil {
		outcomes = []wt.Outcome{}
	}
	repos := make([]output.RepoResult, 0, len(result.Outcomes))
	for _, o := range result.Outcomes {
		repoResult := output.RepoResult{Name: o.Repo, Status: o.Status}
		if o.Status == wt.StatusFailed {
			repoResult.Error = o.Detail
		}
		repos = append(repos, repoResult)
	}
	return output.CommandResult{
		Command: command,
		Summary: map[string]int{
			"changed": changed,
			"skipped": skipped,
			"failed":  failed,
		},
		Repos: repos,
		Data:  map[string]any{"outcomes": outcomes},
	}
}

// reportResult prints a result in the requested format. Per-repository
// failures are reported in the output rather than as a command error, matching
// 'xr repo sync', and like it the command then exits non-zero so the exit
// status alone can gate a pipeline.
func reportResult(cmd *cobra.Command, command string, result *wt.Result, asJSON bool) error {
	changed, skipped, failed := result.Counts()
	if asJSON {
		if err := output.PrintJSON(jsonResult(command, result)); err != nil {
			return err
		}
	} else {
		printOutcomes(result)
		output.PrintActionSummary(changedLabel(command), changed, skipped, failed)
	}
	if failed > 0 {
		return exitcode.Failed(cmd)
	}
	return nil
}

func changedLabel(command string) string {
	switch command {
	case "worktree add":
		return "created"
	case "worktree remove":
		return "removed"
	case "worktree prune":
		return "pruned"
	default:
		return "done"
	}
}
