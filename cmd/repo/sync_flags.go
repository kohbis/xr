package repo

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	syncPrune                 bool
	syncApply                 bool
	syncDirty                 bool
	syncWork                  string
	syncCreateBranchIfMissing bool
	syncUpdate                bool
	syncSubmod                bool
)

func effectiveSyncNetwork() (fetch, pull, submod bool) {
	if syncUpdate {
		fetch = true
		pull = true
	}
	if syncSubmod {
		submod = true
	}
	return fetch, pull, submod
}

func validateSyncFlags(fetch bool) error {
	if syncPrune && !fetch {
		return fmt.Errorf("--prune requires --update")
	}
	if syncCreateBranchIfMissing && !fetch {
		return fmt.Errorf("--create-branch-if-missing requires --update")
	}
	return nil
}

// RegisterSyncFlags adds repo sync flags to cmd. Used by xr repo sync and xr work checkout.
func RegisterSyncFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&syncUpdate, "update", false, "fetch and pull from remote before switching branch")
	cmd.Flags().BoolVar(&syncSubmod, "submodules", false, "update submodules recursively after sync")
	cmd.Flags().BoolVar(&syncPrune, "prune", false, "prune deleted remote branches during fetch (requires --update)")
	cmd.Flags().BoolVar(&syncApply, "apply", false, "apply changes (default: preview only)")
	cmd.Flags().BoolVar(&syncDirty, "allow-dirty", false, "allow syncing repos with uncommitted changes without prompting")
	cmd.Flags().StringVar(&syncWork, "work", "", "scope sync to work plan name (from .xr/work/<name>.yaml)")
	cmd.Flags().BoolVar(&syncCreateBranchIfMissing, "create-branch-if-missing", false, "create local branch when missing (from current HEAD)")
}

// ExecuteSyncWithWork runs sync scoped to a work plan name (used by xr work checkout).
func ExecuteSyncWithWork(cmd *cobra.Command, workName string) error {
	prev := syncWork
	syncWork = workName
	defer func() { syncWork = prev }()
	return runSync(cmd, nil)
}
