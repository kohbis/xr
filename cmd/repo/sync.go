package repo

import (
	"fmt"
	"path/filepath"

	"github.com/kohbis/xr/internal/config"
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
  --dry-run        preview only

Always switches to the branch in repos.yaml.
Use --allow-dirty to proceed on dirty repos without prompting (recommended with --non-interactive).

Without arguments, syncs all repositories. Specify repo names to sync only those.

Examples:
  # Switch branches to match repos.yaml
  xr repo sync

  # Preview without changes
  xr repo sync --dry-run

  # Fetch, checkout, and pull
  xr repo sync --update

  # Fetch with prune, checkout, and pull
  xr repo sync --update --prune`,
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

	cfg, cfgPath, err := loadConfigWithPath(cmd)
	if err != nil {
		return err
	}

	if syncDryRun {
		fmt.Printf("Previewing workspace sync (no changes will be made).\n")
	} else {
		fmt.Printf("Syncing workspace...\n")
	}

	ws := workspace.New(filepath.Dir(cfgPath), cfg)
	shouldPrompt, err := interactive.ShouldPrompt(cmd)
	if err != nil {
		return err
	}

	proceedAllDirty := false
	opts := workspace.SyncOptions{
		Pull:   pull,
		Fetch:  fetch,
		Prune:  syncPrune,
		DryRun: syncDryRun,

		AllowDirty:            syncDirty,
		CreateBranchIfMissing: syncCreateBranchIfMissing,
	}
	if shouldPrompt && !opts.AllowDirty {
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
	if shouldPrompt && !syncDryRun {
		opts.ConfirmCheckout = func(repo config.Repository, fromBranch, toBranch string) (bool, error) {
			// If fromBranch is empty, it likely means a detached HEAD; still confirm.
			label := fmt.Sprintf("%s: switch %q → %q", repo.Name, fromBranch, toBranch)
			return promptYesNoSelect(label, true)
		}
	}

	result, err := ws.Sync(args, opts)
	if err != nil {
		return fmt.Errorf("syncing workspace: %w", err)
	}

	if opts.DryRun {
		fmt.Printf("\nPreview done: %d repo(s)\n", result.Skipped)
		fmt.Printf("To execute: rerun without --dry-run\n")
		if result.Failed > 0 {
			fmt.Printf("Preview failures: %d\n", result.Failed)
		}
		return nil
	}
	output.PrintSyncSummary(result.Synced, result.Skipped, result.Failed)
	return nil
}

func init() {
	registerSyncFlags(syncCmd)
}
