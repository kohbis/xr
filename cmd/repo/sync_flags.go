package repo

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	syncPrune                 bool
	syncDryRun                bool
	syncDirty                 bool
	syncCreateBranchIfMissing bool
	syncUpdate                bool
)

func effectiveSyncNetwork() (fetch, pull bool) {
	if syncUpdate {
		fetch = true
		pull = true
	}
	return fetch, pull
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

func registerSyncFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&syncUpdate, "update", false, "fetch and pull from remote before switching branch")
	cmd.Flags().BoolVar(&syncPrune, "prune", false, "prune deleted remote branches during fetch (requires --update)")
	cmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "preview only, perform no git operations")
	cmd.Flags().BoolVar(&syncDirty, "allow-dirty", false, "allow syncing repos with uncommitted changes without prompting")
	cmd.Flags().BoolVar(&syncCreateBranchIfMissing, "create-branch-if-missing", false, "create local branch when missing (from current HEAD)")
}
