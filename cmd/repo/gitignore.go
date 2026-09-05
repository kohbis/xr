package repo

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// GitignoreCmd adds the workspace directory to .gitignore in the current workspace.
var GitignoreCmd = &cobra.Command{
	Use:   "gitignore",
	Short: "Update .gitignore",
	Long:  `Add the workspace directory to the .gitignore next to repos.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		fmt.Printf("Add repos directory (%s) to .gitignore? [y/N]: ", cfg.Workspace)
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		ignoreWorkspace := answer == "y" || answer == "yes"

		if err := newWorkspace(cfg).CreateGitignore(ignoreWorkspace); err != nil {
			return fmt.Errorf("creating .gitignore: %w", err)
		}

		return nil
	},
}
