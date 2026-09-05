package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/git"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/spf13/cobra"
)

const (
	statusError = "!"
)

var (
	listJSON bool
	listRepo []string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List repositories in the workspace",
	Long: `List all repositories defined in repos.yaml.

Use --repo to limit the listing to specific repositories; they are shown in
repos.yaml order regardless of the order the flags are given in.

Examples:
  xr repo list
  xr repo list -r api -r web
  xr repo list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}
		wsDir, err := cfg.WorkspaceDir()
		if err != nil {
			return fmt.Errorf("resolving workspace path: %w", err)
		}

		repos := cfg.Repositories
		if len(listRepo) > 0 {
			repos = filterRepos(repos, listRepo)
		}

		rows := make([]map[string]string, 0, len(repos))
		result := output.CommandResult{
			Command: "repo list",
			Summary: map[string]int{"repositories": len(repos)},
			Repos:   make([]output.RepoResult, 0, len(repos)),
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "NAME\tTYPE\tBRANCH\tCURRENT\tSTATUS\tPATH\tSOURCE"); err != nil {
			return err
		}
		for _, r := range repos {
			repoPath := filepath.Join(wsDir, r.Path)
			current, status := repoRuntimeStatus(repoPath)
			source := formatSource(cfg.Path, r.Source)
			rows = append(rows, map[string]string{
				"name":    r.Name,
				"type":    string(r.Type),
				"branch":  r.Branch,
				"current": current,
				"status":  status,
				"path":    r.Path,
				"source":  source,
			})
			result.Repos = append(result.Repos, repoListResult(r.Name, repoPath, status))

			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Name, r.Type, r.Branch, current, status, r.Path, source); err != nil {
				return err
			}
		}
		result.Data = map[string]any{"rows": rows}
		if listJSON {
			return output.PrintJSON(result)
		}
		return w.Flush()
	},
}

// repoListResult classifies a repository for --json. A repository absent from
// the workspace is "missing" rather than "failed": like sync and exec skip it,
// it is an expected state before 'xr repo sync --clone-missing', not an error.
func repoListResult(name, repoPath, status string) output.RepoResult {
	if status != statusError {
		return output.RepoResult{Name: name, Status: "ok"}
	}
	if _, err := os.Lstat(repoPath); err != nil {
		return output.RepoResult{Name: name, Status: "missing", Error: "missing in workspace"}
	}
	return output.RepoResult{Name: name, Status: "failed", Error: "repository status unavailable"}
}

// filterRepos returns the repos whose name is in names, in repos.yaml order.
// An unknown name matches nothing, the way --repo behaves in search and exec.
func filterRepos(repos []config.Repository, names []string) []config.Repository {
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}
	filtered := make([]config.Repository, 0, len(repos))
	for _, r := range repos {
		if _, ok := want[r.Name]; ok {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func repoRuntimeStatus(repoPath string) (currentBranch string, status string) {
	snapshot, err := git.Inspect(repoPath)
	if err != nil {
		return "-", statusError
	}
	return snapshot.CurrentBranch, snapshot.Status
}

func formatSource(cfgPath, source string) string {
	if source == "" {
		return source
	}
	// Only try to shorten absolute filesystem paths.
	if !filepath.IsAbs(source) {
		return source
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		home = filepath.Clean(home)
		if source == home {
			return "~"
		}
		prefix := home + string(os.PathSeparator)
		if strings.HasPrefix(source, prefix) {
			return "~" + string(os.PathSeparator) + strings.TrimPrefix(source, prefix)
		}
	}

	cfgDir := filepath.Dir(cfgPath)
	if cfgDir != "" {
		if rel, err := filepath.Rel(cfgDir, source); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}

	return source
}

func init() {
	Cmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")
	listCmd.Flags().StringArrayVarP(&listRepo, "repo", "r", nil, "limit to specific repos (repeatable)")
	cobra.CheckErr(listCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}
