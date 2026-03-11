package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/structure"
	"github.com/spf13/cobra"
)

var (
	treeDepth    int
	treeDepsOnly bool
)

var treeCmd = &cobra.Command{
	Use:   "tree [repo]",
	Short: "Show repository structure",
	Long:  `Display the directory structure of repositories in the workspace.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		wsDir, err := resolveWorkspaceDir(cfg)
		if err != nil {
			return err
		}

		for _, repo := range cfg.Repositories {
			if len(args) > 0 && args[0] != repo.Name {
				continue
			}

			repoPath := filepath.Join(wsDir, repo.Path)
			if _, err := os.Stat(repoPath); os.IsNotExist(err) {
				output.PrintWarning(fmt.Sprintf("repo %s not found at %s (run 'xr init' first)", repo.Name, repoPath))
				continue
			}

			info, err := structure.AnalyzeRepo(repo.Name, repoPath, treeDepth)
			if err != nil {
				output.PrintWarning(fmt.Sprintf("analyzing %s: %v", repo.Name, err))
				continue
			}

			fmt.Println()
			structure.PrintTree(info, treeDepsOnly)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(treeCmd)
	treeCmd.Flags().IntVar(&treeDepth, "depth", 3, "maximum depth to display (0 = unlimited)")
	treeCmd.Flags().BoolVar(&treeDepsOnly, "deps", false, "highlight dependency files")
}
