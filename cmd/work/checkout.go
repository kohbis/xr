package work

import (
	"strings"

	"github.com/kohbis/xr/cmd/repo"
	"github.com/spf13/cobra"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout <name>",
	Short: "Alias for repo sync --work",
	Long: `Apply a work plan by syncing repositories listed in .xr/work/<name>.yaml.

Equivalent to: xr repo sync --work <name>
Accepts the same flags as repo sync (--apply, --update, --submodules, etc.).

Examples:
  xr work checkout example
  xr work checkout example --apply --update --submodules`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSuffix(strings.TrimSuffix(args[0], ".yaml"), ".yml")
		return repo.ExecuteSyncWithWork(cmd, name)
	},
}

func init() {
	repo.RegisterSyncFlags(checkoutCmd)
}
