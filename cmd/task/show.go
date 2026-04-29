package task

import (
	"fmt"

	"github.com/kohbis/xr/internal/output"
	"github.com/spf13/cobra"
)

var showJSON bool

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tf, _, err := loadTasksFile(cmd)
		if err != nil {
			return err
		}
		t, err := findTaskByID(tf, args[0])
		if err != nil {
			return err
		}

		if showJSON {
			return output.PrintJSON(output.CommandResult{
				Command: "task show",
				Data:    map[string]any{"task": t},
			})
		}

		fmt.Printf("ID: %s\n", t.ID)
		if t.CreatedAt != "" {
			fmt.Printf("Created: %s\n", t.CreatedAt)
		}
		if t.Status != "" {
			fmt.Printf("Status: %s\n", t.Status)
		}
		if t.Title != "" {
			fmt.Printf("Title: %s\n", t.Title)
		}
		if t.Description != "" {
			fmt.Printf("\n%s\n", t.Description)
		}
		if len(t.Repos) > 0 {
			fmt.Printf("\nRepos:\n")
			for _, r := range t.Repos {
				fmt.Printf("- %s\n", r)
			}
		}
		if len(t.AcceptanceCriteria) > 0 {
			fmt.Printf("\nAcceptance criteria:\n")
			for _, ac := range t.AcceptanceCriteria {
				fmt.Printf("- %s\n", ac)
			}
		}
		if len(t.Subtasks) > 0 {
			fmt.Printf("\nSubtasks:\n")
			for _, st := range t.Subtasks {
				label := st.ID
				if st.Title != "" {
					label = fmt.Sprintf("%s (%s)", st.ID, st.Title)
				}
				if st.Status != "" {
					label = fmt.Sprintf("%s [%s]", label, st.Status)
				}
				fmt.Printf("- %s\n", label)
			}
		}
		if len(t.Steps) > 0 {
			fmt.Printf("\nSteps:\n")
			for _, s := range t.Steps {
				fmt.Printf("- %s (%s)\n", s.ID, s.Type)
				switch s.Type {
				case "run":
					if s.Repo != "" {
						fmt.Printf("    repo: %s\n", s.Repo)
					} else if len(s.Repos) > 0 {
						fmt.Printf("    repos: %v\n", s.Repos)
					}
					fmt.Printf("    run: %s\n", s.Run)
				default:
					fmt.Printf("    instruction: %s\n", s.Instruction)
				}
			}
		}
		return nil
	},
}

func init() {
	showCmd.Flags().BoolVar(&showJSON, "json", false, "output in JSON format")
}
