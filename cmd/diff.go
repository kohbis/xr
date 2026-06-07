package cmd

import (
	"fmt"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/diff"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/spf13/cobra"
)

var (
	diffRepo   []string
	diffJSON   bool
	diffReport string
)

var diffCmd = &cobra.Command{
	Use:     "diff",
	Short:   "Run git diff across repositories",
	GroupID: "cross",
	Long: `Run git diff in each repository (pager disabled). Pass extra arguments
after -- to git (e.g. "xr diff -- --stat").

Other comparison modes are subcommands:
  xr diff file <path>      unified diff of one path across repos
  xr diff pattern <regex>  show where a pattern appears per repo
  xr diff history <query>  search git commit messages across repos

Limit repos with --repo / -r on any diff command.

Examples:
  xr diff
  xr diff -- --stat
  xr diff -- --name-only
  xr diff -r project-a
  xr diff file go.mod
  xr diff pattern "version" -r project-a
  xr diff history "fix:" --json`,
	RunE: runDiffGit,
}

var diffFileCmd = &cobra.Command{
	Use:   "file <path>",
	Short: "Compare a file path across repositories",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return runDiffFile(args[0])
	},
}

var diffPatternCmd = &cobra.Command{
	Use:   "pattern <regex>",
	Short: "Show where a pattern appears in each repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return runDiffPattern(args[0])
	},
}

var diffHistoryCmd = &cobra.Command{
	Use:   "history <query>",
	Short: "Search git commit messages across repositories",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return runDiffHistory(args[0])
	},
}

func registerDiffRepoFlag(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&diffRepo, "repo", "r", nil, "limit to repo names")
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}

func registerDiffOutputFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&diffJSON, "json", false, "output in JSON format")
	cmd.Flags().StringVar(&diffReport, "report", "", "write JSON report to file")
}

func loadDiffWorkspace() (*config.Config, string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, "", err
	}
	wsDir, err := resolveWorkspaceDir(cfg)
	if err != nil {
		return nil, "", err
	}
	return cfg, wsDir, nil
}

func writeDiffResult(result output.CommandResult) error {
	if diffReport != "" {
		if err := output.WriteJSONFile(diffReport, result); err != nil {
			return fmt.Errorf("writing report: %w", err)
		}
	}
	if diffJSON {
		return output.PrintJSON(result)
	}
	return nil
}

func runDiffGit(_ *cobra.Command, args []string) error {
	if diffJSON || diffReport != "" {
		return fmt.Errorf("--json/--report is not supported for git diff mode")
	}
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}
	return diff.GitDiff(cfg, wsDir, diffRepo, args)
}

func runDiffFile(path string) error {
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}

	comparisons, err := diff.CompareFile(cfg, wsDir, path, diffRepo)
	if err != nil {
		return fmt.Errorf("comparing files: %w", err)
	}

	result := output.CommandResult{
		Command: "diff file",
		Summary: map[string]int{"comparisons": len(comparisons)},
		Data:    map[string]any{"comparisons": comparisons},
	}

	if !diffJSON && diffReport == "" {
		for _, comp := range comparisons {
			fmt.Printf("\nComparing '%s' across repos:\n", comp.FileName)
			for i, rf := range comp.Repos {
				fmt.Printf("\n  [%s] %s\n", rf.Repo, rf.Path)
				if i > 0 {
					diffOut, err := diff.DiffFiles(comp.Repos[i-1], rf)
					if err != nil {
						output.PrintWarning(fmt.Sprintf("diff error: %v", err))
						continue
					}
					for _, line := range strings.Split(diffOut, "\n") {
						output.PrintDiffLine(line)
					}
				}
			}
		}
		if len(comparisons) == 0 {
			fmt.Printf("File '%s' not found in multiple repositories.\n", path)
		}
		return nil
	}

	return writeDiffResult(result)
}

func runDiffPattern(pattern string) error {
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}

	occurrences, err := diff.SearchPattern(cfg, wsDir, pattern, diffRepo)
	if err != nil {
		return fmt.Errorf("searching pattern: %w", err)
	}

	total := 0
	repos := make([]output.RepoResult, 0, len(occurrences))
	for repoName, matches := range occurrences {
		total += len(matches)
		status := "matched"
		if len(matches) == 0 {
			status = "no_matches"
		}
		repos = append(repos, output.RepoResult{Name: repoName, Status: status, Metrics: map[string]int{"matches": len(matches)}})
	}

	result := output.CommandResult{
		Command: "diff pattern",
		Summary: map[string]int{"repos": len(occurrences), "matches": total},
		Repos:   repos,
		Data:    map[string]any{"occurrences": occurrences},
	}

	if !diffJSON && diffReport == "" {
		for repoName, matches := range occurrences {
			output.PrintRepoHeader(repoName)
			if len(matches) == 0 {
				fmt.Println("  (no matches)")
				continue
			}
			for _, m := range matches {
				fmt.Printf("  %s:%d: %s\n", m.File, m.Line, strings.TrimSpace(m.Content))
			}
		}
		return nil
	}

	return writeDiffResult(result)
}

func runDiffHistory(query string) error {
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}

	if !diffJSON && diffReport == "" {
		return diff.SearchHistory(cfg, wsDir, query, diffRepo)
	}

	history, err := diff.SearchHistoryResults(cfg, wsDir, query, diffRepo)
	if err != nil {
		return err
	}

	repos := make([]output.RepoResult, 0, len(history))
	matches := 0
	for _, h := range history {
		m := len(h.Lines)
		matches += m
		status := "ok"
		if m == 0 {
			status = "no_matches"
		}
		repos = append(repos, output.RepoResult{Name: h.Repo, Status: status, Metrics: map[string]int{"matches": m}})
	}

	result := output.CommandResult{
		Command: "diff history",
		Summary: map[string]int{"repos": len(history), "matches": matches},
		Repos:   repos,
		Data:    map[string]any{"history": history},
	}
	return writeDiffResult(result)
}

func init() {
	rootCmd.AddCommand(diffCmd)

	registerDiffRepoFlag(diffCmd)

	registerDiffRepoFlag(diffFileCmd)
	registerDiffOutputFlags(diffFileCmd)
	diffCmd.AddCommand(diffFileCmd)

	registerDiffRepoFlag(diffPatternCmd)
	registerDiffOutputFlags(diffPatternCmd)
	diffCmd.AddCommand(diffPatternCmd)

	registerDiffRepoFlag(diffHistoryCmd)
	registerDiffOutputFlags(diffHistoryCmd)
	diffCmd.AddCommand(diffHistoryCmd)
}
