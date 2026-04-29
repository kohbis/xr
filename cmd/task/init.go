package task

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	initForce bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize xr-tasks.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tasksFilePath(cmd)
		if !initForce {
			if _, err := os.Stat(p); err == nil {
				return fmt.Errorf("%s already exists (use --force to overwrite)", p)
			}
		}

		template := `version: 1

tasks:
  - id: example-task
    createdAt: 2026-04-29
    title: Example task
    description: Replace this with your own task description
    status: planned
    repos: []
    acceptanceCriteria: []
    steps:
      - id: run-example
        type: run
        run: xr repo list --json
      - id: agent-example
        type: agent
        instruction: Implement the feature and open a PR
        acceptanceCriteria:
          - the feature works
`

		if err := os.WriteFile(p, []byte(template), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", p, err)
		}
		fmt.Printf("Created %s\n", p)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing file")
}
