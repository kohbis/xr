package cmd

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
	Use:               "sync [repo...]",
	Short:             "Sync repositories to match repos.yaml configuration",
	Long:              "Shortcut for `xr repo sync`.",
	ValidArgsFunction: shellcomp.CompleteRepoNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		wsDir, err := resolveWorkspaceDir(cfg)
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
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&syncFetch, "fetch", false, "fetch from remote before switching branch")
	syncCmd.Flags().BoolVar(&syncPull, "pull", false, "pull latest changes after switching branch")
	syncCmd.Flags().BoolVar(&syncPrune, "prune", false, "prune deleted remote branches during fetch (requires --fetch)")
	syncCmd.Flags().BoolVar(&syncSubmod, "submodules", false, "update submodules recursively after sync")
}
