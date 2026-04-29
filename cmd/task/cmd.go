package task

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:     "task",
	Short:   "Task harness for multi-repo work",
	Long:    "Define development tasks in xr-tasks.yaml and run reproducible run-steps with reports.",
	GroupID: "task",
}

func init() {
	Cmd.PersistentFlags().String("tasks", "", "tasks file (default: xr-tasks.yaml)")
	Cmd.AddCommand(initCmd)
	Cmd.AddCommand(newCmd)
	Cmd.AddCommand(validateCmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(runCmd)
	Cmd.AddCommand(reportCmd)
}
