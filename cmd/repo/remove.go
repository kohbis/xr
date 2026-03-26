package repo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	removeForce      bool
	removeConfigOnly bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <repo...>",
	Short: "Remove repositories from the workspace",
	Long: `Remove repositories from the workspace and repos.yaml.
The removal method depends on the repository type:
  - symlink: removes the symbolic link
  - clone:   removes the cloned directory
  - git:     deinitializes and removes the git submodule

Use --config-only to remove only from repos.yaml without touching the filesystem.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cmd.Root().PersistentFlags().Lookup("config").Value.String()
		if cfgPath == "" {
			cfgPath = "repos.yaml"
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		reposByName := make(map[string]config.Repository, len(cfg.Repositories))
		for _, r := range cfg.Repositories {
			reposByName[r.Name] = r
		}

		var targets []config.Repository
		for _, name := range args {
			r, ok := reposByName[name]
			if !ok {
				return fmt.Errorf("repository %q not found in config", name)
			}
			targets = append(targets, r)
		}

		fmt.Printf("The following repo(s) will be removed:\n")
		for _, r := range targets {
			fmt.Printf("  - %-20s %-8s %s\n", r.Name, string(r.Type), r.Path)
		}

		if !removeForce {
			fmt.Print("\nProceed? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if !removeConfigOnly {
			wsDir, err := filepath.Abs(cfg.Workspace)
			if err != nil {
				return err
			}

			ws := workspace.New(filepath.Dir(wsDir), cfg)
			if err := ws.Remove(targets); err != nil {
				return fmt.Errorf("removing repos: %w", err)
			}
		}

		// Remove from config
		remaining := make([]config.Repository, 0, len(cfg.Repositories))
		removeSet := make(map[string]struct{}, len(targets))
		for _, r := range targets {
			removeSet[r.Name] = struct{}{}
		}
		for _, r := range cfg.Repositories {
			if _, ok := removeSet[r.Name]; !ok {
				remaining = append(remaining, r)
			}
		}
		cfg.Repositories = remaining

		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Removed %d repo(s) from %s.\n", len(targets), cfgPath)
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "skip confirmation prompt")
	removeCmd.Flags().BoolVar(&removeConfigOnly, "config-only", false, "remove only from config, keep filesystem intact")
}
