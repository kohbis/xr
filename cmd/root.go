package cmd

import (
	"fmt"
	"os"

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

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: repos.yaml in current directory)")
}
