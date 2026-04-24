package repo

import (
	"fmt"
	"path/filepath"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/kohbis/xr/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	removeForce      bool
	removeConfigOnly bool
)

var removeCmd = &cobra.Command{
	Use:   "remove [repo...]",
	Short: "Remove repositories from the workspace",
	Long: `Remove repositories from the workspace and repos.yaml.
The removal method depends on the repository type:
  - symlink: removes the symbolic link
  - clone:   removes the cloned directory
  - git:     deinitializes and removes the git submodule

Use --config-only to remove only from repos.yaml without touching the filesystem.`,
	Args:              cobra.MinimumNArgs(0),
	ValidArgsFunction: shellcomp.CompleteRepoNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		isTTY, err := isInteractiveTTY()
		if err != nil {
			return err
		}

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

		if len(args) == 0 {
			if !isTTY {
				return fmt.Errorf("missing required value(s): repo name(s) (non-interactive)")
			}
			selected, err := promptRemoveTargetsInteractive(reposByName)
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				fmt.Println("Aborted.")
				return nil
			}
			args = selected
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
			if !isTTY {
				return fmt.Errorf("non-interactive remove requires --force")
			}
			ok, err := promptConfirmRemove()
			if err != nil {
				return err
			}
			if !ok {
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

func promptRemoveTargetsInteractive(reposByName map[string]config.Repository) ([]string, error) {
	if len(reposByName) == 0 {
		return nil, fmt.Errorf("no repositories found in config")
	}

	names := make([]string, 0, len(reposByName))
	for name := range reposByName {
		names = append(names, name)
	}
	return promptMultiSelectByDone("Select a repo to remove (search enabled)", names, 15)
}

func promptConfirmRemove() (bool, error) {
	return promptConfirm("Proceed", true)
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "skip confirmation prompt")
	removeCmd.Flags().BoolVar(&removeConfigOnly, "config-only", false, "remove only from config, keep filesystem intact")
}
