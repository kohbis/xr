package cmd

import (
	"fmt"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/diff"
	"github.com/kohbis/xr/internal/exitcode"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/parallel"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/spf13/cobra"
)

var (
	diffRepo   []string
	diffJSON   bool
	diffReport string
	diffJobs   int
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiffPattern(cmd, args[0])
	},
}

var diffHistoryCmd = &cobra.Command{
	Use:   "history <query>",
	Short: "Search git commit messages across repositories",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiffHistory(cmd, args[0])
	},
}

func registerDiffRepoFlag(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&diffRepo, "repo", "r", nil, "limit to repo names")
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
	cmd.Flags().IntVarP(&diffJobs, "jobs", "j", 1, "number of repositories to scan concurrently")
}

func registerDiffOutputFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&diffJSON, "json", false, "output in JSON format")
	cmd.Flags().StringVar(&diffReport, "report", "", "write JSON report to file")
}

// loadDiffWorkspace also validates --jobs, since every diff mode goes through
// it before scanning repositories.
func loadDiffWorkspace() (*config.Config, string, error) {
	if err := parallel.ValidateJobs(diffJobs); err != nil {
		return nil, "", err
	}
	cfg, err := config.LoadCommand(rootCmd)
	if err != nil {
		return nil, "", err
	}
	wsDir, err := cfg.WorkspaceDir()
	if err != nil {
		return nil, "", err
	}
	return cfg, wsDir, nil
}

// scanRepoResult classifies one repository of a diff scan. okStatus is the
// command's own word for a repository with matches — "matched" for pattern,
// "ok" for history — which differ only because both are already --json output.
func scanRepoResult(repo string, matches int, errMsg, okStatus string) output.RepoResult {
	status := okStatus
	switch {
	case errMsg != "":
		status = output.StatusFailed
	case matches == 0:
		status = "no_matches"
	}
	return output.RepoResult{
		Name:    repo,
		Status:  status,
		Error:   errMsg,
		Metrics: map[string]int{"matches": matches},
	}
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

func runDiffGit(cmd *cobra.Command, args []string) error {
	if diffJSON || diffReport != "" {
		return fmt.Errorf("--json/--report is not supported for git diff mode")
	}
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}
	failed := 0
	for _, r := range diff.GitDiff(cfg, wsDir, diffRepo, args, diffJobs) {
		output.PrintRepoHeader(r.Repo)
		fmt.Print(r.Output)
		if r.Error != "" {
			failed++
			output.PrintWarning(fmt.Sprintf("%s: %s", r.Repo, r.Error))
		}
	}
	return exitcode.FailedIf(cmd, failed)
}

func runDiffFile(path string) error {
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}

	comparisons, err := diff.CompareFile(cfg, wsDir, path, diffRepo, diffJobs)
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

func runDiffPattern(cmd *cobra.Command, pattern string) error {
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}

	results, err := diff.SearchPattern(cfg, wsDir, pattern, diffRepo, diffJobs)
	if err != nil {
		return fmt.Errorf("searching pattern: %w", err)
	}

	total := 0
	failed := 0
	repos := make([]output.RepoResult, 0, len(results))
	// Keyed by repository for compatibility with earlier report consumers; the
	// human-readable listing below follows configuration order.
	occurrences := make(map[string][]diff.PatternOccurrence, len(results))
	for _, r := range results {
		total += len(r.Matches)
		if r.Error != "" {
			failed++
		}
		repos = append(repos, scanRepoResult(r.Repo, len(r.Matches), r.Error, "matched"))
		occurrences[r.Repo] = r.Matches
	}

	result := output.CommandResult{
		Command: "diff pattern",
		Summary: map[string]int{"repos": len(results), "matches": total},
		Repos:   repos,
		Data:    map[string]any{"occurrences": occurrences},
	}

	if !diffJSON && diffReport == "" {
		for _, r := range results {
			output.PrintRepoHeader(r.Repo)
			if r.Error != "" {
				output.PrintWarning(fmt.Sprintf("searching %s: %s", r.Repo, r.Error))
				continue
			}
			if len(r.Matches) == 0 {
				fmt.Println("  (no matches)")
				continue
			}
			for _, m := range r.Matches {
				fmt.Printf("  %s:%d: %s\n", m.File, m.Line, strings.TrimSpace(m.Content))
			}
		}
	} else if err := writeDiffResult(result); err != nil {
		return err
	}

	return exitcode.FailedIf(cmd, failed)
}

func runDiffHistory(cmd *cobra.Command, query string) error {
	cfg, wsDir, err := loadDiffWorkspace()
	if err != nil {
		return err
	}

	history, err := diff.SearchHistoryResults(cfg, wsDir, query, diffRepo, diffJobs)
	if err != nil {
		return err
	}

	repos := make([]output.RepoResult, 0, len(history))
	matches := 0
	failed := 0
	for _, h := range history {
		m := len(h.Lines)
		matches += m
		if h.Error != "" {
			failed++
		}
		repos = append(repos, scanRepoResult(h.Repo, m, h.Error, "ok"))
	}

	if !diffJSON && diffReport == "" {
		for _, h := range history {
			output.PrintRepoHeader(h.Repo)
			switch {
			case h.Error != "":
				output.PrintWarning(fmt.Sprintf("%s: %s", h.Repo, h.Error))
			case len(h.Lines) == 0:
				fmt.Println("  (no matches)")
			default:
				fmt.Println(strings.Join(h.Lines, "\n"))
			}
		}
		return exitcode.FailedIf(cmd, failed)
	}

	result := output.CommandResult{
		Command: "diff history",
		Summary: map[string]int{"repos": len(history), "matches": matches},
		Repos:   repos,
		Data:    map[string]any{"history": history},
	}
	if err := writeDiffResult(result); err != nil {
		return err
	}
	return exitcode.FailedIf(cmd, failed)
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
