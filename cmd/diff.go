package cmd

import (
	"fmt"
	"strings"

	"github.com/kohbis/xr/internal/diff"
	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/spf13/cobra"
)

var (
	diffPattern string
	diffFile    string
	diffHistory string
	diffGit     bool
	diffRepo    []string
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare patterns or files across repositories",
	Long: `Compare patterns or files across repositories.
Use --pattern to see where a pattern appears across repos,
--file to compare a specific file across repos (unified diff via the diff command),
--history to search git commit history,
or --git to run git diff in each repository (optional arguments are passed to git diff; use -- before flags, e.g. "xr diff --git -- --stat").
Limit repos with --repo / -r when using --git or --history.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		wsDir, err := resolveWorkspaceDir(cfg)
		if err != nil {
			return err
		}

		modeCount := 0
		if diffHistory != "" {
			modeCount++
		}
		if diffGit {
			modeCount++
		}
		if diffFile != "" {
			modeCount++
		}
		if diffPattern != "" {
			modeCount++
		}
		if modeCount > 1 {
			return fmt.Errorf("use only one of --pattern, --file, --history, or --git")
		}
		if modeCount == 0 {
			return fmt.Errorf("specify --pattern, --file, --history, or --git")
		}
		if len(diffRepo) > 0 && (diffPattern != "" || diffFile != "") {
			return fmt.Errorf("--repo applies only with --git or --history")
		}

		switch {
		case diffHistory != "":
			return diff.SearchHistory(cfg, wsDir, diffHistory, diffRepo)
		case diffGit:
			return diff.GitDiff(cfg, wsDir, diffRepo, args)
		case diffFile != "":
			comparisons, err := diff.CompareFile(cfg, wsDir, diffFile)
			if err != nil {
				return fmt.Errorf("comparing files: %w", err)
			}

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
				fmt.Printf("File '%s' not found in multiple repositories.\n", diffFile)
			}
			return nil
		default:
			occurrences, err := diff.SearchPattern(cfg, wsDir, diffPattern)
			if err != nil {
				return fmt.Errorf("searching pattern: %w", err)
			}

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
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVar(&diffPattern, "pattern", "", "show where a pattern appears across repos")
	diffCmd.Flags().StringVar(&diffFile, "file", "", "compare a specific file across repos")
	diffCmd.Flags().StringVar(&diffHistory, "history", "", "search git commit history")
	diffCmd.Flags().BoolVar(&diffGit, "git", false, "run git diff in each repo (optional args passed through)")
	diffCmd.Flags().StringArrayVarP(&diffRepo, "repo", "r", nil, "limit to repo names (with --git or --history)")
	cobra.CheckErr(diffCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}
