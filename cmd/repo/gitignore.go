package repo

import (
	"fmt"

	"github.com/kohbis/xr/internal/interactive"
	"github.com/spf13/cobra"
)

// GitignoreCmd adds the workspace directory to .gitignore in the current workspace.
var GitignoreCmd = &cobra.Command{
	Use:   "gitignore",
	Short: "Update .gitignore",
	Long: `Add the workspace directory to the .gitignore next to repos.yaml.

Prompts for confirmation. Pass --yes to add the entry without prompting; with
--non-interactive (or no terminal on stdin) and no --yes the command fails
rather than leaving .gitignore untouched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		ignoreWorkspace, err := confirmIgnoreWorkspace(cmd, cfg.Workspace)
		if err != nil {
			return err
		}

		if err := newWorkspace(cfg).CreateGitignore(ignoreWorkspace); err != nil {
			return fmt.Errorf("creating .gitignore: %w", err)
		}

		return nil
	},
}

// confirmIgnoreWorkspace reports whether the workspace directory should be
// added to .gitignore.
//
// Reading stdin directly used to make a non-interactive run answer "no" by
// hitting EOF, so a CI invocation silently did nothing. Without a prompt
// available the command now says what it needs instead.
func confirmIgnoreWorkspace(cmd *cobra.Command, workspace string) (bool, error) {
	if interactive.Yes(cmd) {
		return true, nil
	}
	shouldPrompt, err := interactive.ShouldPrompt(cmd)
	if err != nil {
		return false, err
	}
	if !shouldPrompt {
		return false, fmt.Errorf("non-interactive gitignore requires --yes")
	}
	return interactive.YesNo(fmt.Sprintf("Add %s to .gitignore", workspace), true)
}
