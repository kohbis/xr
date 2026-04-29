package work

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/output"
	"github.com/spf13/cobra"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List work plans",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := workspaceRoot(cmd)
		if err != nil {
			return err
		}
		dir := workDir(root)
		relDir := filepath.Join(".xr", "work")

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				if listJSON {
					return output.PrintJSON(output.CommandResult{
						Command: "work list",
						Summary: map[string]int{"plans": 0},
						Data: map[string]any{
							"dir":   relDir,
							"plans": []string{},
						},
					})
				}
				fmt.Println("No work plans found.")
				return nil
			}
			return err
		}

		var plans []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				plans = append(plans, strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml"))
			}
		}
		sort.Strings(plans)

		if listJSON {
			return output.PrintJSON(output.CommandResult{
				Command: "work list",
				Summary: map[string]int{"plans": len(plans)},
				Data: map[string]any{
					"dir":   relDir,
					"plans": plans,
				},
			})
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME")
		for _, p := range plans {
			if _, err := fmt.Fprintf(w, "%s\n", p); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")
}
