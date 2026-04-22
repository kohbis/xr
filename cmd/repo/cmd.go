package repo

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories in the workspace",
	Long:  `Commands for managing repositories defined in repos.yaml.`,
}

func init() {
	Cmd.AddCommand(importCmd)
	Cmd.AddCommand(removeCmd)
	Cmd.AddCommand(addCmd)
	Cmd.AddCommand(syncCmd)
	Cmd.AddCommand(GitignoreCmd)
}
