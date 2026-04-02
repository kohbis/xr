package repo

import (
	"fmt"
	"path/filepath"

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

		fmt.Printf("Syncing workspace...\n")

		ws := workspace.New(filepath.Dir(wsDir), cfg)
		opts := workspace.SyncOptions{
			Pull:   syncPull,
			Fetch:  syncFetch,
			Prune:  syncPrune,
			Submod: syncSubmod,
		}
		result, err := ws.Sync(args, opts)
		if err != nil {
			return fmt.Errorf("syncing workspace: %w", err)
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
}
