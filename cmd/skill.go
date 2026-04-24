package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:     "skill",
	Short:   "Print SKILL.md",
	Long:    "Print the repository's SKILL.md to stdout.",
	GroupID: "meta",
	RunE: func(cmd *cobra.Command, args []string) error {
		candidates := []string{
			"SKILL.md",
		}

		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			candidates = append(candidates, filepath.Join(exeDir, "SKILL.md"))
		}

		var lastErr error
		for _, p := range candidates {
			b, err := os.ReadFile(p)
			if err == nil {
				fmt.Print(string(b))
				return nil
			}
			lastErr = err
		}

		// Installed binaries (e.g. Homebrew) won't have SKILL.md beside the executable.
		// Fall back to the embedded copy set by main.
		if skillMDEmbedded != "" {
			fmt.Print(skillMDEmbedded)
			return nil
		}

		return fmt.Errorf("SKILL.md not found (tried: %s): %w", candidates, lastErr)
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
}
