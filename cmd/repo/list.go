package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/git"
	"github.com/spf13/cobra"
)

const (
	statusError = "!"
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
		wsDir, err := filepath.Abs(cfg.Workspace)
		if err != nil {
			return fmt.Errorf("resolving workspace path: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "NAME\tTYPE\tBRANCH\tCURRENT\tSTATUS\tPATH\tSOURCE"); err != nil {
			return err
		}
		for _, r := range cfg.Repositories {
			repoPath := filepath.Join(wsDir, r.Path)
			current, status := repoRuntimeStatus(repoPath)
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Name, r.Type, r.Branch, current, status, r.Path, r.Source); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

func repoRuntimeStatus(repoPath string) (currentBranch string, status string) {
	snapshot, err := git.Inspect(repoPath)
	if err != nil {
		return "-", statusError
	}
	return snapshot.CurrentBranch, snapshot.Status
}

func init() {
	Cmd.AddCommand(listCmd)
}
