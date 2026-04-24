package cmd

import (
	"fmt"

	"github.com/kohbis/xr/internal/output"
	"github.com/kohbis/xr/internal/search"
	"github.com/kohbis/xr/internal/shellcomp"
	"github.com/spf13/cobra"
)

var (
	searchGlob       string
	searchIgnoreCase bool
	searchContext    int
	searchRegex      bool
	searchRepo       []string
)

var searchCmd = &cobra.Command{
	Use:     "search <pattern>",
	Short:   "Search across all repositories",
	GroupID: "cross",
	Long: `Search for a pattern across all repositories in the workspace.
Uses ripgrep if available, falls back to built-in grep.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		wsDir, err := resolveWorkspaceDir(cfg)
		if err != nil {
			return err
		}

		opts := search.Options{
			Pattern:    args[0],
			Glob:       searchGlob,
			IgnoreCase: searchIgnoreCase,
			Context:    searchContext,
			UseRegex:   searchRegex,
			RepoFilter: searchRepo,
		}

		matches, err := search.Search(cfg, wsDir, opts)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(matches) == 0 {
			fmt.Println("No matches found.")
			return nil
		}

		currentRepo := ""
		for _, m := range matches {
			if m.Repo != currentRepo {
				output.PrintRepoHeader(m.Repo)
				currentRepo = m.Repo
			}
			output.PrintMatchSimple(m.Repo, m.File, m.Line, m.Content, m.IsContext)
		}

		fmt.Printf("\n%d match(es) found.\n", countMatches(matches))
		return nil
	},
}

func countMatches(matches []search.Match) int {
	count := 0
	for _, m := range matches {
		if !m.IsContext {
			count++
		}
	}
	return count
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVarP(&searchGlob, "glob", "g", "", "glob pattern to filter files (e.g. '*.go')")
	searchCmd.Flags().BoolVarP(&searchIgnoreCase, "ignore-case", "i", false, "case-insensitive search")
	searchCmd.Flags().IntVarP(&searchContext, "context", "C", 0, "lines of context around matches")
	searchCmd.Flags().BoolVarP(&searchRegex, "regex", "e", false, "treat pattern as regular expression")
	searchCmd.Flags().StringArrayVarP(&searchRepo, "repo", "r", nil, "limit search to specific repos")
	cobra.CheckErr(searchCmd.RegisterFlagCompletionFunc("repo", shellcomp.CompleteRepoNames))
}
