package repo

import (
	"fmt"
	"path/filepath"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	syncPull   bool
	syncFetch  bool
	syncPrune  bool
	syncSubmod bool
	syncApply  bool
	syncDirty  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync [repo...]",
	Short: "Sync repositories to match repos.yaml configuration",
	Long: `Synchronize repositories to match the configuration in repos.yaml.

This command performs the following for each repository (or specified repos):
  - Fetch from remote (with --fetch)
  - Switch to the branch specified in repos.yaml
  - Pull latest changes (with --pull)
  - Update submodules recursively (with --submodules)

By default, this command runs in a preview mode and prints what it would do.
Use --apply to perform the actions.

Without arguments, syncs all repositories. Specify repo names to sync only those.

Examples:
  # Sync all repos: fetch, checkout configured branch, and pull
  xr repo sync --fetch --pull

  # Sync specific repos with submodule updates
  xr repo sync project-a project-b --pull --submodules

  # Just switch branches to match repos.yaml (no fetch/pull)
  xr repo sync`,
	ValidArgsFunction: shellcomp.CompleteRepoNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		wsDir, err := filepath.Abs(cfg.Workspace)
		if err != nil {
			return err
		}

		if syncApply {
			fmt.Printf("Syncing workspace...\n")
		} else {
			fmt.Printf("Previewing workspace sync (no changes will be made).\n")
		}

		ws := workspace.New(filepath.Dir(wsDir), cfg)
		isTTY, err := isInteractiveTTY()
		if err != nil {
			return err
		}

		proceedAllDirty := false
		opts := workspace.SyncOptions{
			Pull:   syncPull,
			Fetch:  syncFetch,
			Prune:  syncPrune,
			Submod: syncSubmod,
			DryRun: !syncApply,

			AllowDirty: syncDirty,
		}
		if isTTY && !opts.AllowDirty {
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
		if isTTY && syncApply {
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
			fmt.Printf("To execute: xr repo sync --apply\n")
			if result.Failed > 0 {
				fmt.Printf("Preview failures: %d\n", result.Failed)
			}
			return nil
		}
		output.PrintSyncSummary(result.Synced, result.Skipped, result.Failed)
		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncFetch, "fetch", false, "fetch from remote before switching branch")
	syncCmd.Flags().BoolVar(&syncPull, "pull", false, "pull latest changes after switching branch")
	syncCmd.Flags().BoolVar(&syncPrune, "prune", false, "prune deleted remote branches during fetch (requires --fetch)")
	syncCmd.Flags().BoolVar(&syncSubmod, "submodules", false, "update submodules recursively after sync")
	syncCmd.Flags().BoolVar(&syncApply, "apply", false, "apply changes (default: preview only)")
	syncCmd.Flags().BoolVar(&syncDirty, "allow-dirty", false, "allow syncing repos with uncommitted changes without prompting")
}
