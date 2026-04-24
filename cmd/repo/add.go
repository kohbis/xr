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
	Use:   "add [name]",
	Short: "Add a repository to the workspace",
	Long: `Add a new repository to repos.yaml and set it up in the workspace.
The repository type is inferred from the source unless --type is specified:
  - Local path (starts with / or ~) → symlink
  - Remote URL                      → git (submodule)
  - Explicit --type clone           → clone

If a repository with the same name or path already exists, an error is returned.`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := os.Stdin.Stat()
		if err != nil {
			return err
		}
		isTTY := (in.Mode() & os.ModeCharDevice) != 0

		reader := bufio.NewReader(os.Stdin)
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		// Collect required values first; decide repo name last.
		resolvedSource, err := resolveSource(addSource, isTTY, reader)
		if err != nil {
			return err
		}
		addSource = resolvedSource

		// If name is omitted, infer it from the source (URL or local path).
		// Fallback to prompting only when inference fails and we have a TTY.
		if strings.TrimSpace(name) == "" {
			name = inferNameFromSource(addSource)
			if strings.TrimSpace(name) == "" {
				if !isTTY {
					return fmt.Errorf("missing required value(s): name (argument; could not infer from --source)")
				}
				name = promptRequired(reader, "Repository name", "")
			}
		}

		// In non-interactive environments (no TTY), do not prompt (avoid hanging in CI).
		// Keep defaults without prompting.
		if !isTTY {
			if strings.TrimSpace(addPath) == "" {
				addPath = name
			}
		}

		if strings.TrimSpace(addPath) == "" {
			addPath = promptOptional(reader, "Path within workspace", name)
		}
		if strings.TrimSpace(addType) == "" {
			if isTTY {
				addType = promptRepoTypeInteractive(reader)
			}
		}
		if strings.TrimSpace(addBranch) == "" && isTTY {
			addBranch = promptOptional(reader, "Branch (optional)", "")
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

		repo := config.Repository{
			Name:   name,
			Source: addSource,
			Branch: addBranch,
			Path:   repoPath,
		}

		if addType != "" {
			switch strings.ToLower(strings.TrimSpace(addType)) {
			case "git", "symlink", "clone":
				repo.Type = config.RepoType(strings.ToLower(strings.TrimSpace(addType)))
			default:
				return fmt.Errorf("--type must be one of: git, symlink, clone (or omit for auto)")
			}
		}

		// Check for duplicates by name or path
		for _, existing := range cfg.Repositories {
			nameMatch := existing.Name == repo.Name
			pathMatch := existing.Path == repo.Path
			if !nameMatch && !pathMatch {
				continue
			}

			if nameMatch {
				return fmt.Errorf("repository name %q already exists in %s", repo.Name, cfgPath)
			}
			return fmt.Errorf("repository path %q already exists in %s", repo.Path, cfgPath)
		}

		cfg.Repositories = append(cfg.Repositories, repo)
		if err := config.Save(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Added %q to %s.\n", name, cfgPath)

		return setupRepo(cfg, repo)
	},
}

func promptRepoTypeInteractive(reader *bufio.Reader) string {
	i, err := promptSelect(reader, "Type", []string{"auto", "git (submodule)", "symlink", "clone"}, 10, false)
	if err != nil {
		return ""
	}
	switch i {
	case 0:
		return ""
	case 1:
		return "git"
	case 2:
		return "symlink"
	case 3:
		return "clone"
	default:
		return ""
	}
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
