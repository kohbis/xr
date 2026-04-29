package work

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:     "work",
	Short:   "Work plan harness (repo selection)",
	GroupID: "workspace",
	Long:    "Manage work plans stored under .xr/work/<name>.yaml to scope repo-oriented commands.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(checkoutCmd)
	Cmd.AddCommand(deleteCmd)
}

