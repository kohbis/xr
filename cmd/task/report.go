package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kohbis/xr/internal/output"
	"github.com/spf13/cobra"
)

var (
	reportLatest bool
	reportList   bool
	reportJSON   bool
)

var reportCmd = &cobra.Command{
	Use:   "report <id>",
	Short: "Inspect task run reports",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceRoot, err := resolveWorkspaceRoot(cmd)
		if err != nil {
			return err
		}
		taskID := args[0]
		dir := filepath.Join(workspaceRoot, ".xr", "reports", taskID)

		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".json") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)

		if reportJSON {
			return output.PrintJSON(output.CommandResult{
				Command: "task report",
				Summary: map[string]int{"reports": len(names)},
				Data:    map[string]any{"dir": dir, "files": names},
			})
		}

		if reportLatest || (!reportList && !reportLatest) {
			if len(names) == 0 {
				fmt.Println("No reports found.")
				return nil
			}
			fmt.Println(filepath.Join(dir, names[len(names)-1]))
			return nil
		}

		for _, n := range names {
			fmt.Println(filepath.Join(dir, n))
		}
		return nil
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportLatest, "latest", false, "show only the latest report path")
	reportCmd.Flags().BoolVar(&reportList, "list", false, "list report paths")
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "output in JSON format")
}
