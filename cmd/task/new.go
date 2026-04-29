package task

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <id>",
	Short: "Print a new task template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(id) {
			return fmt.Errorf("id must be kebab-case (got %q)", id)
		}

		createdAt := time.Now().Format("2006-01-02")
		title := strings.ReplaceAll(id, "-", " ")
		fmt.Printf(`- id: %s
  createdAt: %s
  title: %s
  description: ""
  status: planned
  repos: []
  goals: []
  acceptanceCriteria: []
  subtasks: []
  steps:
    - id: run-setup
      type: run
      run: xr repo list --json
    - id: implement
      type: agent
      instruction: ""
      acceptanceCriteria: []
`, id, createdAt, title)
		return nil
	},
}
