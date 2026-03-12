package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var gitignoreCmd = &cobra.Command{
	Use:   "gitignore",
	Short: "Update .gitignore",
	Long:  `Add workspace directory entries to .gitignore in the current workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		absDir, err := filepath.Abs(".")
		if err != nil {
			return fmt.Errorf("resolving directory: %w", err)
		}

		fmt.Printf("Add repos directory (%s) to .gitignore? [y/N]: ", cfg.Workspace)
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		ignoreWorkspace := answer == "y" || answer == "yes"

		ws := workspace.New(absDir, cfg)
		if err := ws.CreateGitignore(ignoreWorkspace); err != nil {
			return fmt.Errorf("creating .gitignore: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(gitignoreCmd)
}
