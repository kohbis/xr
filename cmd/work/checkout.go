package work

import (
	"fmt"
	"strings"

	"github.com/kohbis/xr/cmd/repo"
	"github.com/spf13/cobra"
)

var (
	checkoutApply                bool
	checkoutFetch                bool
	checkoutPull                 bool
	checkoutPrune                bool
	checkoutSubmodules           bool
	checkoutAllowDirty           bool
	checkoutCreateBranchIfMissing bool
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout <name>",
	Short: "Alias for repo sync --work",
	Long: `Alias for applying a work plan by syncing repositories in the plan.

This is equivalent to running:
  xr repo sync --work <name>
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSuffix(strings.TrimSuffix(args[0], ".yaml"), ".yml")
		syncCmd, _, err := repo.Cmd.Find([]string{"sync"})
		if err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("work", name); err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("apply", fmt.Sprintf("%t", checkoutApply)); err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("fetch", fmt.Sprintf("%t", checkoutFetch)); err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("pull", fmt.Sprintf("%t", checkoutPull)); err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("prune", fmt.Sprintf("%t", checkoutPrune)); err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("submodules", fmt.Sprintf("%t", checkoutSubmodules)); err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("allow-dirty", fmt.Sprintf("%t", checkoutAllowDirty)); err != nil {
			return err
		}
		if err := syncCmd.Flags().Set("create-branch-if-missing", fmt.Sprintf("%t", checkoutCreateBranchIfMissing)); err != nil {
			return err
		}
		return syncCmd.RunE(syncCmd, []string{})
	},
}

func init() {
	checkoutCmd.Flags().BoolVar(&checkoutApply, "apply", false, "apply changes (default: preview only)")
	checkoutCmd.Flags().BoolVar(&checkoutFetch, "fetch", false, "fetch from remote before switching branch")
	checkoutCmd.Flags().BoolVar(&checkoutPull, "pull", false, "pull latest changes after switching branch")
	checkoutCmd.Flags().BoolVar(&checkoutPrune, "prune", false, "prune deleted remote branches during fetch (requires --fetch)")
	checkoutCmd.Flags().BoolVar(&checkoutSubmodules, "submodules", false, "update submodules recursively after sync")
	checkoutCmd.Flags().BoolVar(&checkoutAllowDirty, "allow-dirty", false, "allow syncing repos with uncommitted changes without prompting")
	checkoutCmd.Flags().BoolVar(&checkoutCreateBranchIfMissing, "create-branch-if-missing", false, "create local branch when missing (from current HEAD)")
}

