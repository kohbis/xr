package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/kohbis/xr/internal/git"
	"github.com/kohbis/xr/internal/output"
	"github.com/spf13/cobra"
)

const (
	statusError = "!"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List repositories in the workspace",
	Long:  `List all repositories defined in repos.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, cfgPath, err := loadConfigWithPath(cmd)
		if err != nil {
			return err
		}
		wsDir, err := resolveWorkspaceDirFromConfigPath(cfgPath, cfg)
		if err != nil {
			return fmt.Errorf("resolving workspace path: %w", err)
		}

		repos := cfg.Repositories

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
			source := formatSource(cfgPath, r.Source)
			rows = append(rows, map[string]string{
				"name":    r.Name,
				"type":    string(r.Type),
				"branch":  r.Branch,
				"current": current,
				"status":  status,
				"path":    r.Path,
				"source":  source,
			})
			repoStatus := "ok"
			repoErr := ""
			if status == statusError {
				repoStatus = "failed"
				repoErr = "repository status unavailable"
			}
			result.Repos = append(result.Repos, output.RepoResult{Name: r.Name, Status: repoStatus, Error: repoErr})

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
}
