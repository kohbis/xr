package task

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/output"
	"github.com/spf13/cobra"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		tf, _, err := loadTasksFile(cmd)
		if err != nil {
			return err
		}

		if listJSON {
			return output.PrintJSON(output.CommandResult{
				Command: "task list",
				Summary: map[string]int{"tasks": len(tf.Tasks)},
				Data:    map[string]any{"tasks": tf.Tasks},
			})
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "ID\tSTATUS\tCREATED\tTITLE\tREPOS"); err != nil {
			return err
		}
		for _, t := range tf.Tasks {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", t.ID, t.Status, t.CreatedAt, t.Title, len(t.Repos))
		}
		return w.Flush()
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")
}
