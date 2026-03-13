package repo

import (
	"fmt"
	"path/filepath"

	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var updatePull bool

var updateCmd = &cobra.Command{
	Use:   "update [repo...]",
	Short: "Update workspace repositories",
	Long: `Update repositories in the workspace. Without arguments, updates all repos.
Specify repo names to update only those repos.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		wsDir, err := filepath.Abs(cfg.Workspace)
		if err != nil {
			return err
		}

		fmt.Printf("Updating workspace...\n")

		ws := workspace.New(filepath.Dir(wsDir), cfg)
		if err := ws.Update(args, updatePull); err != nil {
			return fmt.Errorf("updating workspace: %w", err)
		}

		fmt.Printf("\nUpdate complete.\n")
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updatePull, "pull", false, "pull latest changes from remote")
}
