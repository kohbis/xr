package repo

import (
	"fmt"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/interactive"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [repo...]",
	Short: "Sync repositories to match repos.yaml configuration",
	Long: `Synchronize repositories to match the configuration in repos.yaml.

By default this command runs git operations. Use --dry-run to preview without changes.

Sync options:
  (none)           switch branches only
  --update         fetch and pull from remote
  --prune          prune deleted remote branches during fetch (requires --update)
  --clone-missing  clone repositories missing from the workspace
  --jobs N / -j N  sync N repositories concurrently
  --dry-run        preview only
  --json           print a JSON result instead of progress output
  --report PATH    additionally write the JSON result to a file

Always switches to the branch in repos.yaml.
Use --allow-dirty to proceed on dirty repos without prompting (recommended with --non-interactive).

Missing repositories are skipped by default. With --clone-missing they are
materialized instead — clone repos are cloned, symlink repos are linked — which
makes an unattended bootstrap from repos.yaml possible without interactive xr init.

Exits non-zero when any repository fails to sync. Skipped repositories are not
failures, so a run that only skips still exits 0.

--jobs syncs several repositories at once, which mainly helps --update and
--clone-missing on large workspaces. Output stays grouped and ordered by
repository, but each block appears only once that repository finishes. Because
concurrent workers cannot share stdin, --jobs above 1 disables prompts: dirty
repositories are skipped unless --allow-dirty or --yes is given.

Without arguments, syncs all repositories. Specify repo names to sync only those.

Examples:
  # Switch branches to match repos.yaml
  xr repo sync

  # Preview without changes
  xr repo sync --dry-run

  # Fetch, checkout, and pull
  xr repo sync --update

  # Fetch with prune, checkout, and pull
  xr repo sync --update --prune

  # Bootstrap a workspace unattended (CI / agents)
  xr repo sync --clone-missing --update --non-interactive --allow-dirty

  # Bootstrap a large workspace with 8 repositories in flight
  xr repo sync --clone-missing --update -j 8 --allow-dirty

  # Machine-readable result for a pipeline
  xr repo sync --update --allow-dirty --json
  xr repo sync --update --allow-dirty --report sync-report.json`,
	ValidArgsFunction: shellcomp.CompleteRepoNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd, args)
	},
}

func runSync(cmd *cobra.Command, args []string) error {
	fetch, pull := effectiveSyncNetwork()
	if err := validateSyncFlags(fetch); err != nil {
		return err
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	if !syncJSON {
		if syncDryRun {
			fmt.Printf("Previewing workspace sync (no changes will be made).\n")
		} else {
			fmt.Printf("Syncing workspace...\n")
		}
	}

	ws := newWorkspace(cfg)
	shouldPrompt, err := interactive.ShouldPrompt(cmd)
	if err != nil {
		return err
	}
	// Concurrent workers cannot share stdin, so --jobs opts out of prompting the
	// same way --non-interactive does.
	if syncJobs > 1 {
		shouldPrompt = false
	}
	// JSON goes to stdout, where a prompt would corrupt it.
	if syncJSON {
		shouldPrompt = false
	}

	proceedAllDirty := false
	opts := workspace.SyncOptions{
		Pull:   pull,
		Fetch:  fetch,
		Prune:  syncPrune,
		DryRun: syncDryRun,

		AllowDirty:            syncDirty,
		CreateBranchIfMissing: syncCreateBranchIfMissing,
		CloneMissing:          syncCloneMissing,
		Jobs:                  syncJobs,
		Quiet:                 syncJSON,
	}
	yesFlag := interactive.Yes(cmd)
	// --yes proceeds on every dirty repository, which is what --allow-dirty does.
	// Expressing it that way rather than as an always-true callback keeps the
	// options free of prompts, so --jobs can still run workers concurrently.
	if yesFlag {
		opts.AllowDirty = true
	}
	if !opts.AllowDirty && shouldPrompt {
		opts.ConfirmDirty = func(repo config.Repository, reason string) (bool, error) {
			if proceedAllDirty {
				return true, nil
			}
			choice, err := promptSelect(nil, fmt.Sprintf("%s: %s", repo.Name, reason), []string{"Skip", "Proceed", "Proceed all"}, 10, false)
			if err != nil {
				return false, err
			}
			switch choice {
			case 0:
				return false, nil
			case 1:
				return true, nil
			case 2:
				proceedAllDirty = true
				return true, nil
			default:
				return false, nil
			}
		}
	}
	if shouldPrompt && !yesFlag && !syncDryRun {
		opts.ConfirmCheckout = func(repo config.Repository, fromBranch, toBranch string) (bool, error) {
			label := fmt.Sprintf("%s: switch %q → %q", repo.Name, fromBranch, toBranch)
			return promptYesNoSelect(label, true)
		}
	}

	result, err := ws.Sync(args, opts)
	if err != nil {
		return fmt.Errorf("syncing workspace: %w", err)
	}

	if syncReport != "" {
		if err := output.WriteJSONFile(syncReport, syncResultJSON(result, opts.DryRun)); err != nil {
			return fmt.Errorf("writing report: %w", err)
		}
	}
	if syncJSON {
		if err := output.PrintJSON(syncResultJSON(result, opts.DryRun)); err != nil {
			return err
		}
		return syncExitCode(cmd, result)
	}

	if opts.DryRun {
		fmt.Printf("\nPreview done: %d repo(s)\n", result.Skipped)
		fmt.Printf("To execute: rerun without --dry-run\n")
		if result.Failed > 0 {
			fmt.Printf("Preview failures: %d\n", result.Failed)
		}
		return syncExitCode(cmd, result)
	}
	output.PrintSyncSummary(result.Synced, result.Skipped, result.Failed)
	return syncExitCode(cmd, result)
}

// syncResultJSON renders a sync result in the shared machine-readable shape.
// Per-repository steps are the same lines the terminal output shows.
func syncResultJSON(result *workspace.SyncResult, dryRun bool) output.CommandResult {
	repos := make([]output.RepoResult, 0, len(result.Repos))
	for _, r := range result.Repos {
		rr := output.RepoResult{Name: r.Name, Status: r.Status}
		if r.Status == workspace.SyncStatusFailed {
			rr.Error = r.Detail
		}
		repos = append(repos, rr)
	}
	outcomes := result.Repos
	if outcomes == nil {
		outcomes = []workspace.SyncOutcome{}
	}
	return output.CommandResult{
		Command: "repo sync",
		Summary: map[string]int{
			"synced":  result.Synced,
			"skipped": result.Skipped,
			"failed":  result.Failed,
		},
		Repos: repos,
		Data: map[string]any{
			"dry_run": dryRun,
			"repos":   outcomes,
		},
	}
}

// syncExitCode makes the process exit non-zero when repositories failed to
// sync. Per-repository failures are already reported in the summary above, so
// the error carries no message and cobra's error/usage output is suppressed —
// only the exit status changes.
func syncExitCode(cmd *cobra.Command, result *workspace.SyncResult) error {
	if result.Failed == 0 {
		return nil
	}
	return exitcode.Failed(cmd)
}

func init() {
	registerSyncFlags(syncCmd)
}
