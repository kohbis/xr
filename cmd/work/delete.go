package work

import (
	"fmt"
	"os"

	"github.com/kohbis/xr/internal/work"
	"github.com/spf13/cobra"
)

var deleteYes bool

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a work plan file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !deleteYes {
			return fmt.Errorf("refusing to delete without --yes")
		}

		root, err := workspaceRoot(cmd)
		if err != nil {
			return err
		}

		path, err := work.SafeFilePath(root, args[0])
		if err != nil {
			return err
		}

		if err := os.Remove(path); err != nil {
			return fmt.Errorf("deleting work plan: %w", err)
		}

		fmt.Println(path)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteYes, "yes", false, "delete the work plan (required)")
}
