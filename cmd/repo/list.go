package repo

import (
	"fmt"
	"text/tabwriter"
	"os"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List repositories in the workspace",
	Long:  `List all repositories defined in repos.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tBRANCH\tPATH\tSOURCE")
		for _, r := range cfg.Repositories {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.Type, r.Branch, r.Path, r.Source)
		}
		return w.Flush()
	},
}

func init() {
	Cmd.AddCommand(listCmd)
}
