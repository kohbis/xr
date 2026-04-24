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
	diffRepo    []string
)

var diffCmd = &cobra.Command{
	Use:     "diff",
	Short:   "Run git diff across repositories (or optional pattern/file/history modes)",
	GroupID: "cross",
	Long: `By default runs git diff in each repository (pager disabled). Optional arguments
after -- are passed to git (e.g. "xr diff -- --stat").

Other modes (mutually exclusive): --pattern to see where a regex appears across repos,
--file to compare a specific file across repos (unified diff via the diff command),
--history to search git commit history.

Limit repos with --repo / -r for the default git diff and for --history.`,
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
		if diffFile != "" {
			modeCount++
		}
		if diffPattern != "" {
			modeCount++
		}
		if modeCount > 1 {
			return fmt.Errorf("use only one of --pattern, --file, or --history")
		}
		if len(diffRepo) > 0 && (diffPattern != "" || diffFile != "") {
			return fmt.Errorf("--repo applies only with the default git diff or --history")
		}

		switch {
		case diffHistory != "":
			return diff.SearchHistory(cfg, wsDir, diffHistory, diffRepo)
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
		case diffPattern != "":
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
		default:
			return diff.GitDiff(cfg, wsDir, diffRepo, args)
		}
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVar(&diffPattern, "pattern", "", "show where a pattern appears across repos")
	diffCmd.Flags().StringVar(&diffFile, "file", "", "compare a specific file across repos")
	diffCmd.Flags().StringVar(&diffHistory, "history", "", "search git commit history")
	diffCmd.Flags().StringArrayVarP(&diffRepo, "repo", "r", nil, "limit to repo names (default git diff or --history)")
	cobra.CheckErr(diffCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}
