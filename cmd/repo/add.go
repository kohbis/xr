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
	addSource string
	addBranch string
	addPath   string
	addType   string
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a repository to the workspace",
	Long: `Add a new repository to repos.yaml and set it up in the workspace.
The repository type is inferred from the source unless --type is specified:
  - Local path (starts with / or ~) → symlink
  - Remote URL                      → git (submodule)
  - Explicit --type clone           → clone

If a repository with the same name or path already exists, you will be
prompted to update it or abort.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if addSource == "" {
			return fmt.Errorf("--source is required")
		}

		cfgPath := cmd.Root().PersistentFlags().Lookup("config").Value.String()
		if cfgPath == "" {
			cfgPath = "repos.yaml"
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		repoPath := addPath
		if repoPath == "" {
			repoPath = name
		}

		repo := config.Repository{
			Name:   name,
			Source: addSource,
			Branch: addBranch,
			Path:   repoPath,
		}

		if addType != "" {
			repo.Type = config.RepoType(addType)
		}

		// Check for duplicates by name or path
		for i, existing := range cfg.Repositories {
			nameMatch := existing.Name == repo.Name
			pathMatch := existing.Path == repo.Path
			if !nameMatch && !pathMatch {
				continue
			}

			var reason string
			if nameMatch && pathMatch {
				reason = fmt.Sprintf("name %q and path %q", repo.Name, repo.Path)
			} else if nameMatch {
				reason = fmt.Sprintf("name %q", repo.Name)
			} else {
				reason = fmt.Sprintf("path %q", repo.Path)
			}

			fmt.Printf("A repository with the same %s already exists:\n", reason)
			fmt.Printf("  existing: %-20s %-8s %s\n", existing.Name, string(existing.Type), existing.Source)
			fmt.Printf("  new:      %-20s          %s\n", repo.Name, repo.Source)
			fmt.Print("\nUpdate the existing entry? [y/N]: ")

			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" {
				fmt.Println("Aborted.")
				return nil
			}

			cfg.Repositories[i] = repo
			// Re-run type inference via Save → Load round-trip is not needed;
			// config.Load infers type, so we just save and let the next load handle it.
			if err := config.Save(cfgPath, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Printf("Updated %q in %s.\n", name, cfgPath)
			return setupRepo(cfg, repo)
		}

		cfg.Repositories = append(cfg.Repositories, repo)
		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Added %q to %s.\n", name, cfgPath)

		return setupRepo(cfg, repo)
	},
}

func setupRepo(cfg *config.Config, repo config.Repository) error {
	// Reload config to apply type inference and validation
	reloadedCfg, err := reloadConfigFromMemory(cfg)
	if err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	// Find the repo with inferred type
	var resolved config.Repository
	for _, r := range reloadedCfg.Repositories {
		if r.Name == repo.Name {
			resolved = r
			break
		}
	}

	wsDir, err := filepath.Abs(reloadedCfg.Workspace)
	if err != nil {
		return err
	}

	ws := workspace.New(filepath.Dir(wsDir), reloadedCfg)
	if err := ws.Add(resolved); err != nil {
		return fmt.Errorf("setting up repo: %w", err)
	}

	return nil
}

// reloadConfigFromMemory marshals and re-parses the config to apply
// type inference and validation that Load() performs.
func reloadConfigFromMemory(cfg *config.Config) (*config.Config, error) {
	return config.Reload(cfg)
}

func init() {
	addCmd.Flags().StringVarP(&addSource, "source", "s", "", "source URL or local path (required)")
	addCmd.Flags().StringVarP(&addBranch, "branch", "b", "", "branch name")
	addCmd.Flags().StringVarP(&addPath, "path", "p", "", "relative path within workspace (default: name)")
	addCmd.Flags().StringVarP(&addType, "type", "t", "", "repository type: git, symlink, or clone (auto-detected if omitted)")
}
