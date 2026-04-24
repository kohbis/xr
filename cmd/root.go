package cmd

import (
	"fmt"
	"os"

	"github.com/kohbis/xr/cmd/repo"
	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "xr",
	Short: "Cross-repository search & management CLI",
	Long: `xr is a CLI tool for searching and managing multiple repositories.
Define repositories in repos.yaml and use xr to search, view, and compare across them.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// SetVersion sets rootCmd.Version for Cobra's --version / -v (see spf13/cobra Command.Version).
// Empty v defaults to "dev" (e.g. when main.version is unset at link time).
func SetVersion(v string) {
	if v == "" {
		v = "dev"
	}
	rootCmd.Version = v
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: repos.yaml in current directory)")

	rootCmd.AddGroup(
		&cobra.Group{ID: "workspace", Title: "Workspace"},
		&cobra.Group{ID: "repo", Title: "Repository management"},
		&cobra.Group{ID: "cross", Title: "Cross-repository"},
		&cobra.Group{ID: "meta", Title: "Other"},
	)

	rootCmd.AddCommand(repo.Cmd)

	// Ensure the default completion command is present so it can be grouped.
	rootCmd.InitDefaultCompletionCmd()
	if completionCmd, _, err := rootCmd.Find([]string{"completion"}); err == nil {
		completionCmd.GroupID = "meta"
	}
}
